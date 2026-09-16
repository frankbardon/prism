// Package marks holds the per-mark encoders that turn rows of a
// materialised table into scene.Mark entries with pixel-resolved
// geometry. P05 supports five marks: bar (Rect), line, area, point,
// rule. Other types (arc/text/path/image, plus composite/specialty)
// emit a PRISM_WARN_MARK_NOT_IMPLEMENTED warning.
package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/projection"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/geodata"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Channel binds an encoding channel to a resolved Scale and the
// field name on the upstream table. Created by encode.Encode and
// handed to each mark encoder.
type Channel struct {
	Field string
	Scale Scale
}

// Scale is the minimal scale surface mark encoders need. Matches
// encode.Scale by structural duck-typing without an import cycle.
type Scale interface {
	Apply(value any) (float64, error)
	Domain() []any
}

// BandScaler is the optional capability bar marks ask for to size
// rect widths. Implemented by encode.BandScale.
type BandScaler interface {
	BandWidth() float64
}

// ColorChannel binds a color encoding (categorical field + palette).
// SequentialPalette is consulted by quantitative-color encoders
// (heatmap, choropleth) — when non-empty, the encoder interpolates
// within the stops rather than calling SequentialColor's hardcoded
// blue gradient.
type ColorChannel struct {
	Field             string
	Categories        []string
	Palette           []*scene.Color
	SequentialPalette []*scene.Color
	// DarkPalette / DarkSequentialPalette (E4-S3) mirror Palette /
	// SequentialPalette but resolved against the active theme's
	// DarkVariant counterpart, using the exact same category / stop
	// ordering (same field, same scheme cascade, different theme).
	// Populated by encode.go only when Inputs.ColorRegistry is
	// non-nil (auto-dark active); nil/empty otherwise. Mark encoders
	// index them positionally alongside Palette/SequentialPalette so
	// each resolved color gets registered as a light+dark pair
	// (ColorRegistry.Resolve) instead of baked as a literal hex.
	DarkPalette           []*scene.Color
	DarkSequentialPalette []*scene.Color
}

// OpacityChannel is a field-driven per-mark opacity binding. The
// heatmap encoder maps the field's numeric values linearly over
// [min, max] to [OpacityFloor, 1.0] so low values stay faintly
// visible. Pair it with a crosstab overlay column (e.g.
// zscore_vs_margin) to shade cells by standardized deviation.
type OpacityChannel struct {
	Field string
}

// OpacityFloor is the minimum opacity a field-driven opacity channel
// maps to, so the least-prominent cell still renders faintly rather
// than vanishing entirely.
const OpacityFloor = 0.15

// Inputs carries the per-Encode-call context: table, encoded
// channels, layout, mark style.
//
// Tooltip (P10) carries the encoding.tooltip binding; when non-nil,
// the dispatch attaches one *scene.Tooltip per produced mark via
// AttachTooltips (per-row marks: 1:1; single-mark types: row 0's
// tooltip). See D063.
//
// Source / Target / Value (P11) carry the sankey-specific channel
// bindings — Field name only, no scale. See D064. Used exclusively
// by encodeSankey; other encoders ignore them.
type Inputs struct {
	Table *table.Table
	X     Channel
	Y     Channel
	// X2 / Y2 (E9-S3) are the secondary position channels. Each one
	// carries its own field name but the *same* resolved Scale as its
	// base channel, so a span is measured in the base channel's units
	// and lands on the base channel's axis. A zero Channel means "no
	// span bound", which keeps every mark on its historic
	// baseline-anchored geometry. See span.go.
	X2    Channel
	Y2    Channel
	Color *ColorChannel
	// Detail (E5-S1) carries the encoding.detail binding as an
	// ordered list of table field names. Detail is a pure grouping
	// channel: it partitions a mark's rows into separate series
	// exactly as Color does (see groupRows) but consumes no palette
	// slot, builds no legend, and leaves mark styling untouched. Nil
	// or empty means "no detail bound", which keeps grouping
	// byte-identical to the color-only behavior.
	Detail []string
	// Ordered (E5-S4) reports that the leaf encoding binds `order`,
	// i.e. the author took explicit control of row sequence and the
	// plan already sorted the table (plan/build's injectEncodingOrder;
	// see spec/order.go). The path marks act on it: line and area
	// trace their points in table order instead of re-sorting each
	// group by resolved x pixel. Every other sense of the channel —
	// stack order, draw order — falls out of table order and needs no
	// flag. False (the default) preserves the pre-E5-S4 geometry
	// exactly.
	Ordered bool
	Opacity *OpacityChannel
	Layout  scene.Rect // the Plot region
	Style   scene.Style
	Mark    *spec.MarkDef        // mark-level overrides; nil ok
	Tooltip *spec.TooltipChannel // encoding.tooltip binding; nil ok
	Source  Channel              // sankey source-node field (no scale)
	Target  Channel              // sankey target-node field (no scale)
	Value   Channel              // sankey flow-magnitude field (no scale)
	// Text is the encoding.text binding (E4-S4). Consumed by the text
	// mark to source its label content — `field` reads a column,
	// `value` supplies a literal, and `format` runs the result through
	// encode/format. nil (the default) leaves the text encoder on its
	// historical y-value fallback.
	Text *spec.TextChannel
	// Feature (P18) is the geoshape feature-id binding — the table
	// column whose values are geodata IDs (USA, US-CA, …).
	Feature Channel
	// Longitude / Latitude (P18) are geopoint bindings; field-only.
	Longitude Channel
	Latitude  Channel
	// Projection (P18) maps lon/lat → pixel space for geoshape and
	// geopoint marks. Nil for non-geo marks.
	Projection projection.Projection
	// GeoStore (P18) is the feature-geometry source. Defaults to
	// geodata.DefaultStore() at dispatch time.
	GeoStore geodata.Store
	// GeoTier (P18) is the manifest tier the encoder pulls features
	// from. Defaults to TierWorld110m.
	GeoTier geodata.Tier
	// LayerID is the scene-layer identifier stamped onto every
	// per-row mark's Datum back-reference (D077). Empty defaults to
	// "layer-0" — matches the flat-encoder hardcoded layer ID.
	// Composite callers (layer / facet / repeat) override per-cell.
	LayerID string
	// KeyField (animation) is the encoding-channel field name flagged
	// with key:true in the spec. When non-empty, per-row marks get
	// Mark.Key = "<field>=<value>" so the client-side animator can
	// match marks across scene swaps. Empty (default) leaves Mark.Key
	// blank; the SVG renderer ignores Mark.Key either way.
	KeyField string
	// LabelStyle is the theme-resolved default Style for markType
	// "text" (independent of Style, which resolves the *outer*
	// spec-level markType). Composite encoders that emit their own
	// scene.MarkText sub-marks alongside a differently-styled primary
	// geometry (e.g. funnel's per-stage value label next to its
	// trapezoid Path) use this instead of an empty scene.Style{} so
	// the label picks up the theme's text fill and stays legible
	// across dark/print/high-contrast themes.
	LabelStyle scene.Style
	// ColorRegistry (E4-S3) accumulates light/dark resolved mark-color
	// pairs for the "auto light/dark in one SVG" feature. nil — the
	// default, and the entire state whenever the active theme has no
	// DarkVariant — means "bake the literal palette/theme color",
	// exactly the pre-E4-S3 behavior; every mark encoder that resolves
	// a scale-driven or static theme color must treat nil this way.
	// See ColorVarRegistry and ColorChannel.DarkPalette.
	ColorRegistry *ColorVarRegistry
}

// Encode dispatches markType to its per-mark helper. Returns the
// generated marks + an optional warning (for unsupported types).
// Errors bubble PRISM_ENCODE_001 or PRISM_RENDER_001 from the
// helpers.
//
// P10: tooltip materialisation runs post-dispatch. When
// in.Tooltip != nil, BuildTooltips walks the upstream table once
// and AttachTooltips attaches the *scene.Tooltip to each mark in
// per-row order (single-mark types receive row 0's tooltip).
func Encode(markType string, in Inputs) ([]scene.Mark, *scene.Warning, error) {
	var (
		marksOut []scene.Mark
		warn     *scene.Warning
		err      error
	)
	switch markType {
	case "bar":
		marksOut, err = encodeBar(in)
	case "line":
		marksOut, err = encodeLine(in)
	case "area":
		marksOut, err = encodeArea(in)
	case "point":
		marksOut, err = encodePoint(in)
	case "rule":
		marksOut, err = encodeRule(in)
	case "text":
		marksOut, err = encodeText(in)
	case "tick":
		marksOut, err = encodeTick(in)
	case "rect":
		marksOut, err = encodeRect(in)
	case "arc", "pie", "donut":
		marksOut, err = encodeArc(in, markType)
	case "histogram":
		result, herr := EncodeHistogram(in)
		if herr != nil {
			return nil, nil, herr
		}
		marksOut = result.Marks
	case "heatmap":
		marksOut, err = encodeHeatmap(in)
	case "boxplot":
		marksOut, err = encodeBoxplot(in)
	case "violin":
		marksOut, err = encodeViolin(in)
	case "path":
		marksOut, err = encodePath(in)
	case "image":
		marksOut, err = encodeImage(in)
	case "sankey":
		marksOut, err = encodeSankey(in)
	case "funnel":
		marksOut, err = encodeFunnel(in)
	case "bullet":
		marksOut, err = encodeBullet(in)
	case "sparkline":
		marksOut, err = encodeSparkline(in)
	case "sparkbar":
		marksOut, err = encodeSparkbar(in)
	case "winloss":
		marksOut, err = encodeWinloss(in)
	case "sparkarea":
		marksOut, err = encodeSparkarea(in)
	case "geoshape":
		marksOut, err = encodeGeoshape(in)
	case "geopoint":
		marksOut, err = encodeGeopoint(in)
	case "tree":
		marksOut, err = encodeTree(in)
	case "dendrogram":
		marksOut, err = encodeDendrogram(in)
	case "network":
		marksOut, err = encodeNetwork(in)
	default:
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Unknown mark type %q.", markType),
			map[string]any{"Field": "<mark>", "Source": "<spec>", "Available": "bar|line|area|point|rule|arc|pie|donut|histogram|heatmap|boxplot|violin|path|image|sankey|funnel|sparkline|sparkbar|winloss"},
		)
	}
	if err != nil {
		return nil, nil, err
	}
	// Stamp Datum back-references on per-row marks (D077).
	// Per-row encoders produce one mark per upstream row; composite
	// encoders that emit fewer/more marks just get the leading prefix
	// stamped — the JS hit-test silently ignores marks without the
	// data-prism-datum-row attribute (see D077).
	if in.Table != nil && len(marksOut) > 0 {
		AttachDatum(marksOut, in.LayerID, in.Table.NumRows())
	}
	if in.KeyField != "" && in.Table != nil && len(marksOut) > 0 {
		AttachKeys(marksOut, in.Table, in.KeyField)
	}
	if in.Tooltip != nil && len(marksOut) > 0 {
		tooltips := BuildTooltips(in.Table, in.Tooltip, in.Table.NumRows())
		AttachTooltips(marksOut, tooltips)
	}
	return marksOut, warn, nil
}

// readField returns a slice of values from the named column on tbl.
// Returns PRISM_ENCODE_001 when the field is missing.
func readField(tbl *table.Table, name string) ([]any, error) {
	col, ok := tbl.Column(name)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Field %q not present in upstream table.", name),
			map[string]any{"Field": name, "Source": "<table>", "Available": joinFieldNames(tbl)},
		)
	}
	out := make([]any, col.Len())
	for i := 0; i < col.Len(); i++ {
		out[i] = col.ValueAt(i)
	}
	return out, nil
}

// joinFieldNames returns the table's columns as a comma-separated
// string for error context.
func joinFieldNames(tbl *table.Table) string {
	names := tbl.FieldNames()
	if len(names) == 0 {
		return ""
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}
