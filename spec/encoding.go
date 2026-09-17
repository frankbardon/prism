package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Encoding is the map of channel name → channel binding for a leaf spec.
//
// Source / Target / Value (P11) are sankey-specific bindings carrying
// a field name without an axis scale. See D064.
type Encoding struct {
	X         *PositionChannel `json:"x,omitempty"`
	Y         *PositionChannel `json:"y,omitempty"`
	X2        *PositionChannel `json:"x2,omitempty"`
	Y2        *PositionChannel `json:"y2,omitempty"`
	Theta     *PositionChannel `json:"theta,omitempty"`
	Radius    *PositionChannel `json:"radius,omitempty"`
	Color     *MarkChannel     `json:"color,omitempty"`
	Fill      *MarkChannel     `json:"fill,omitempty"`
	Stroke    *MarkChannel     `json:"stroke,omitempty"`
	Opacity   *MarkChannel     `json:"opacity,omitempty"`
	Size      *MarkChannel     `json:"size,omitempty"`
	Shape     *MarkChannel     `json:"shape,omitempty"`
	Text      *TextChannel     `json:"text,omitempty"`
	Tooltip   *TooltipChannel  `json:"tooltip,omitempty"`
	Order     *OrderChannel    `json:"order,omitempty"`
	Detail    *DetailChannel   `json:"detail,omitempty"`
	Row       *FacetChannel    `json:"row,omitempty"`
	Column    *FacetChannel    `json:"column,omitempty"`
	Source    *MarkChannel     `json:"source,omitempty"`
	Target    *MarkChannel     `json:"target,omitempty"`
	Value     *MarkChannel     `json:"value,omitempty"`
	Longitude *MarkChannel     `json:"longitude,omitempty"`
	Latitude  *MarkChannel     `json:"latitude,omitempty"`
	// Feature is the geoshape-specific binding: the table field whose
	// values are geodata feature IDs (e.g. "USA", "US-CA"). Resolves
	// to polygon geometry via the geodata.Store.
	Feature *MarkChannel `json:"feature,omitempty"`
	// Columns declares the column list for a table mark (E1). Each
	// entry is a standard channel binding (field/type/aggregate/…)
	// plus an optional sub-mark, so a column can render as formatted
	// text (the default) or as an embedded mark (e.g. "sparkline").
	// Required and non-empty for mark type "table" — see
	// PRISM_SPEC_040.
	Columns []TableColumn `json:"columns,omitempty"`
}

// TableColumn is one column definition inside a table mark's
// encoding.columns[] array (E1). It carries the same channel-binding
// fields as any other channel encoding (field, type, aggregate,
// scale, title, format, bin, sort, value, condition) via the shared
// ChannelCommon shape, plus an optional sub-mark selecting how the
// column's cells render.
//
// Unlike PositionChannel/MarkChannel, TableColumn does not intercept
// the "field" key for repeat-ref substitution — a plain string field
// name is all that's supported for now — so it needs no custom
// UnmarshalJSON; the embedded ChannelCommon decodes (and inherits
// Decode's DisallowUnknownFields strictness) via plain reflection.
type TableColumn struct {
	ChannelCommon
	// Mark selects the sub-mark used to render this column's cells
	// (e.g. "sparkline" for an inline trend chart). Empty renders the
	// column as formatted text — the default. The encode-time
	// dispatch for a populated Mark lands in E1-S3.
	Mark string `json:"mark,omitempty"`
}

// ChannelCommon holds the fields shared by every channel class.
//
// Field is the bare field-name binding. FieldRef carries the
// {"repeat": "row"|"column"} substitution placeholder when the spec
// uses the polymorphic form; the build-time repeat walker
// (plan/build/composite.go) rewrites FieldRef into Field per cell.
// At most one of the two is populated for a given channel — never
// both. See D055.
type ChannelCommon struct {
	Field     string     `json:"field,omitempty"`
	FieldRef  *RepeatRef `json:"-"`
	Type      string     `json:"type,omitempty"`
	Aggregate string     `json:"aggregate,omitempty"`
	Scale     *Scale     `json:"scale,omitempty"`
	Title     string     `json:"title,omitempty"`
	Format    string     `json:"format,omitempty"`
	Bin       any        `json:"bin,omitempty"`
	Sort      any        `json:"sort,omitempty"`
	Value     any        `json:"value,omitempty"`
	// Condition carries a per-channel conditional encoding clause.
	// nil for unconditional channels (the common case). See
	// spec/condition.go and validate rules PRISM_SPEC_025/026/027.
	Condition *Condition `json:"condition,omitempty"`
	// Key marks this channel as the join key used by the client-side
	// animator to match marks across successive scenes (object
	// constancy). At most one channel per encoding block may set this;
	// the validator enforces uniqueness via PRISM_SPEC_024.
	Key bool `json:"key,omitempty"`
}

// isJSONNull reports whether raw is the JSON literal null. Used by
// the channel decoders to tell an explicit `"axis": null` /
// `"legend": null` (suppression) apart from an absent key (default).
func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// PositionChannel adds axis + stack to ChannelCommon.
//
// Axis is tri-state on the wire (Vega-Lite's suppression syntax):
//
//	key absent      → Axis nil,  AxisHidden false → default axis
//	"axis": {...}   → Axis set,  AxisHidden false → configured axis
//	"axis": null    → Axis nil,  AxisHidden true  → no axis at all
//
// A nil Axis alone therefore does NOT mean "hidden" — read
// AxisHidden for that. The flag is decode-only state (no wire key of
// its own); MarshalJSON re-emits it as the JSON null it came from.
type PositionChannel struct {
	ChannelCommon
	Axis *Axis `json:"axis,omitempty"`
	// AxisHidden records an explicit `"axis": null` on this channel.
	// Encode suppresses the axis entirely — domain line, ticks,
	// labels, title and grid — and releases the padding the axis's
	// side had reserved (see encode.AxisPlacement).
	AxisHidden bool `json:"-"`
	// Stack (E5-S2) selects the stacking offset for this channel:
	// "zero" (or true) accumulates segments from the baseline,
	// "normalize" rescales each stack to a 0..1 share, "center"
	// is reserved for the streamgraph work and currently resolves
	// to no stacking at all. false — and an explicit null, recorded
	// in StackNull — disable the implicit stacking a bar / area
	// mark would otherwise pick up. See spec/stack.go.
	Stack any `json:"stack,omitempty"`
	// StackNull records an explicit `"stack": null` on this channel.
	// A nil Stack alone does NOT mean "disabled" — an absent key is
	// nil too, and that leaves implicit stacking enabled. Decode-only
	// state (no wire key of its own); MarshalJSON re-emits it as the
	// JSON null it came from.
	StackNull bool `json:"-"`
}

// UnmarshalJSON intercepts the `field` key so the channel accepts
// either a bare string or a {"repeat": <axis>} substitution object,
// and the `axis` / `stack` keys so an explicit null is
// distinguishable from an absent key. All other keys decode through
// the default struct path.
//
// Strictness caveat: a custom UnmarshalJSON receives raw bytes, and
// the outer decoder's DisallowUnknownFields setting does not reach
// inside it, so an unknown key written *within* a channel object is
// dropped here rather than raising a decode error. Every other
// channel class with a custom decoder (MarkChannel, TooltipChannel,
// OrderChannel, DetailChannel) and every block they decode by hand
// (Axis, Legend) share the caveat. The JSON Schema shape stage is
// what rejects those keys — `additionalProperties: false` on each
// channel $def in schema/v1/encoding.schema.json — so a caller that
// runs spec.Decode without validate.ShapeValidator sees the key
// silently dropped. Route decoding through the validator, not
// Decode alone, when strictness matters.
func (p *PositionChannel) UnmarshalJSON(data []byte) error {
	type alias PositionChannel
	var aux struct {
		Field json.RawMessage `json:"field"`
		Axis  json.RawMessage `json:"axis"`
		Stack json.RawMessage `json:"stack"`
		alias
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*p = PositionChannel(aux.alias)
	// The outer `Stack` raw field shadows the embedded alias's `stack`
	// key at the type level, so the value never lands via aux.alias —
	// decode it here, exactly as `field` and `axis` are.
	if len(aux.Stack) > 0 {
		if isJSONNull(aux.Stack) {
			p.Stack = nil
			p.StackNull = true
		} else {
			var v any
			if err := json.Unmarshal(aux.Stack, &v); err != nil {
				return fmt.Errorf("stack: %w", err)
			}
			p.Stack = v
		}
	}
	if len(aux.Field) > 0 {
		f, ref, err := fieldOrRepeat(aux.Field)
		if err != nil {
			return err
		}
		p.Field = f
		p.FieldRef = ref
	}
	if len(aux.Axis) > 0 {
		if isJSONNull(aux.Axis) {
			p.Axis = nil
			p.AxisHidden = true
		} else {
			var ax Axis
			if err := json.Unmarshal(aux.Axis, &ax); err != nil {
				return fmt.Errorf("axis: %w", err)
			}
			p.Axis = &ax
		}
	}
	return nil
}

// MarshalJSON re-emits an explicit `"axis": null` for a hidden axis
// and `"stack": null` for explicitly disabled stacking. Without it the
// nil pointer plus omitempty would drop the key and silently turn
// "hidden" / "disabled" back into "default" on a round-trip. The
// plain path (neither flag set) marshals through the struct encoding,
// so its bytes are unchanged.
func (p PositionChannel) MarshalJSON() ([]byte, error) {
	type alias PositionChannel
	null := json.RawMessage("null")
	// One aux shape per flag combination. A shadowing outer field must
	// always be populated: encoding/json resolves the name conflict at
	// type level, so an unset outer `axis` would suppress the embedded
	// (configured) one rather than fall through to it.
	switch {
	case !p.AxisHidden && !p.StackNull:
		return json.Marshal(alias(p))
	case p.AxisHidden && !p.StackNull:
		var aux struct {
			alias
			Axis *json.RawMessage `json:"axis"`
		}
		aux.alias, aux.Axis = alias(p), &null
		return json.Marshal(aux)
	case !p.AxisHidden && p.StackNull:
		var aux struct {
			alias
			Stack *json.RawMessage `json:"stack"`
		}
		aux.alias, aux.Stack = alias(p), &null
		return json.Marshal(aux)
	default:
		var aux struct {
			alias
			Axis  *json.RawMessage `json:"axis"`
			Stack *json.RawMessage `json:"stack"`
		}
		aux.alias, aux.Axis, aux.Stack = alias(p), &null, &null
		return json.Marshal(aux)
	}
}

// MarkChannel adds legend to ChannelCommon.
//
// Legend is tri-state on the wire exactly as PositionChannel.Axis is:
// an absent key leaves Legend nil with LegendHidden false (default
// legend), `"legend": null` leaves Legend nil with LegendHidden true
// (no legend at all).
type MarkChannel struct {
	ChannelCommon
	Legend *Legend `json:"legend,omitempty"`
	// LegendHidden records an explicit `"legend": null` on this
	// channel. Encode emits no legend for it.
	LegendHidden bool `json:"-"`
}

// UnmarshalJSON intercepts the `field` key for the same reason as
// PositionChannel, and the `legend` key so an explicit null is
// distinguishable from an absent key.
func (m *MarkChannel) UnmarshalJSON(data []byte) error {
	type alias MarkChannel
	var aux struct {
		Field  json.RawMessage `json:"field"`
		Legend json.RawMessage `json:"legend"`
		alias
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*m = MarkChannel(aux.alias)
	if len(aux.Field) > 0 {
		f, ref, err := fieldOrRepeat(aux.Field)
		if err != nil {
			return err
		}
		m.Field = f
		m.FieldRef = ref
	}
	if len(aux.Legend) > 0 {
		if isJSONNull(aux.Legend) {
			m.Legend = nil
			m.LegendHidden = true
		} else {
			var lg Legend
			if err := json.Unmarshal(aux.Legend, &lg); err != nil {
				return fmt.Errorf("legend: %w", err)
			}
			m.Legend = &lg
		}
	}
	return nil
}

// MarshalJSON re-emits an explicit `"legend": null` for a hidden
// legend; see PositionChannel.MarshalJSON.
func (m MarkChannel) MarshalJSON() ([]byte, error) {
	type alias MarkChannel
	if !m.LegendHidden {
		return json.Marshal(alias(m))
	}
	var aux struct {
		alias
		Legend *json.RawMessage `json:"legend"`
	}
	aux.alias = alias(m)
	null := json.RawMessage("null")
	aux.Legend = &null
	return json.Marshal(aux)
}

// TextChannel is a slimmer channel for text marks and tooltips.
type TextChannel struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
	Format    string `json:"format,omitempty"`
	Title     string `json:"title,omitempty"`
	Value     any    `json:"value,omitempty"`
}

// TooltipChannel is either a single text channel or an array.
type TooltipChannel struct {
	Single *TextChannel
	Multi  []TextChannel
}

// MarshalJSON emits the single channel or the array.
func (c TooltipChannel) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either form.
func (c *TooltipChannel) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []TextChannel
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("tooltip: %w", err)
		}
		c.Multi = arr
		return nil
	}
	var single TextChannel
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("tooltip: %w", err)
	}
	c.Single = &single
	return nil
}

// OrderChannelEntry is one element in an order channel.
type OrderChannelEntry struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
	Sort      string `json:"sort,omitempty"`
}

// OrderChannel is either a single entry or an array.
type OrderChannel struct {
	Single *OrderChannelEntry
	Multi  []OrderChannelEntry
}

// MarshalJSON emits a single entry or array.
func (c OrderChannel) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either form.
func (c *OrderChannel) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []OrderChannelEntry
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("order: %w", err)
		}
		c.Multi = arr
		return nil
	}
	var single OrderChannelEntry
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("order: %w", err)
	}
	c.Single = &single
	return nil
}

// DetailChannelEntry is one detail-channel element.
type DetailChannelEntry struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
}

// DetailChannel is either a single entry or an array.
type DetailChannel struct {
	Single *DetailChannelEntry
	Multi  []DetailChannelEntry
}

// MarshalJSON emits the underlying form.
func (c DetailChannel) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either form.
func (c *DetailChannel) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []DetailChannelEntry
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("detail: %w", err)
		}
		c.Multi = arr
		return nil
	}
	var single DetailChannelEntry
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("detail: %w", err)
	}
	c.Single = &single
	return nil
}

// FacetChannel binds a field for row/column facetting.
type FacetChannel struct {
	Field  string            `json:"field,omitempty"`
	Type   string            `json:"type,omitempty"`
	Sort   any               `json:"sort,omitempty"`
	Header *FacetChannelHead `json:"header,omitempty"`
}

// FacetChannelHead carries optional header rendering options.
type FacetChannelHead struct {
	Title  string `json:"title,omitempty"`
	Labels *bool  `json:"labels,omitempty"`
}
