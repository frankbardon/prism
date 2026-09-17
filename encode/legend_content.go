package encode

import (
	"fmt"
	"strconv"

	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// LegendContent is the resolved *content* half of a channel's
// `legend` block: what the legend is titled, which entries it shows,
// how their labels read and how wide a label may grow before it is
// truncated. It is the companion of LegendPlacement, which resolves
// the *placement* half (E1-S3).
//
// Everything that reads a spec.Legend field goes through this type,
// and every frame / margin measurement goes through LegendBox. E3-S4
// added the presentation half — `type`, `direction`, `symbol_type`
// and `symbol_size` — here alongside Format and Values, and feeds
// Direction / SymbolSize through to LegendBox so the frame and the
// side reservation follow from one measurement.
type LegendContent struct {
	// Title is the resolved title. It is only meaningful when
	// TitleSet is true — otherwise the caller's derived title (the
	// channel's field name) stands.
	Title string
	// TitleSet reports that legend.title was specified, including the
	// `false` form that suppresses the title (Title is "" then). It
	// is the same string-or-false convention axisTitleString applies
	// to axis.title.
	TitleSet bool
	// Values is legend.values: the entries to show, in the order the
	// author listed them. Empty means "every category".
	Values []any
	// Format is the parsed legend.format d3 specifier, or nil when
	// none was given. Labels route through encode/format — the same
	// subset validate/rules/format_string_valid.go already checks the
	// string against — never through fmt verbs.
	Format *format.Spec
	// TickCount is legend.tick_count: how many labelled stops a
	// gradient legend draws. Nil leaves the default.
	TickCount *int
	// LabelLimit is legend.label_limit: the maximum label width in
	// pixels. Nil or non-positive means unlimited.
	LabelLimit *float64
	// Kind is legend.type: an explicit override of the legend form,
	// or "" to infer it from the channel (see ResolveLegendKind).
	Kind LegendKind
	// Direction is legend.direction: which way entries flow. "" reads
	// as scene.LegendVertical, the pre-E3-S4 column.
	Direction scene.LegendDirection
	// SymbolType is legend.symbol_type: the point-mark shape a symbol
	// legend draws its swatches with. "" leaves the solid square
	// swatch every legend drew before E3-S4.
	SymbolType scene.PointShape
	// SymbolSize is legend.symbol_size in pixels. Nil or
	// non-positive leaves the renderer's own default.
	SymbolSize *float64
}

// LegendKind discriminates the two forms a channel's legend takes: a
// column of category swatches, or a continuous gradient bar. It is
// the resolved `legend.type`, and ResolveLegendKind is the single
// place the decision is made.
type LegendKind string

const (
	LegendKindSymbol   LegendKind = "symbol"
	LegendKindGradient LegendKind = "gradient"
)

// ResolveLegendKind decides which form a channel's legend takes.
//
// `legend.type` overrides outright. With no override the channel's
// declared type decides: a quantitative channel reads as a continuous
// ramp and gets a gradient bar, everything else gets category
// swatches. Prism requires `type` on every channel, so there is no
// inference fallback to guess at.
//
// A gradient still needs a numeric domain to label, so the caller
// checks that separately (legendGradientFor) and falls back to a
// symbol legend when the bound column carries no numbers.
func ResolveLegendKind(ch *spec.MarkChannel, c LegendContent) LegendKind {
	if c.Kind != "" {
		return c.Kind
	}
	if ch != nil && ch.Type == "quantitative" {
		return LegendKindGradient
	}
	return LegendKindSymbol
}

// ResolveLegendContent reads the content fields off a channel's
// legend block. A nil block (absent key, or `"legend": null`) yields
// the zero value, which reproduces the pre-E3-S3 defaults exactly.
//
// An unparseable legend.format is ignored rather than failed:
// PRISM_SPEC_011 (validate/rules/format_string_valid.go) already
// rejects it, so a validated spec never reaches here with one.
func ResolveLegendContent(lg *spec.Legend) LegendContent {
	var c LegendContent
	if lg == nil {
		return c
	}
	if t, ok := legendTitleString(lg.Title); ok {
		c.Title, c.TitleSet = t, true
	}
	c.Values = lg.Values
	if lg.Format != "" {
		if sp, err := format.Parse(lg.Format); err == nil {
			c.Format = sp
		}
	}
	if lg.TickCount != nil {
		n := *lg.TickCount
		c.TickCount = &n
	}
	if lg.LabelLimit != nil {
		l := *lg.LabelLimit
		c.LabelLimit = &l
	}
	// Presentation half (E3-S4). Each field is read verbatim; the
	// JSON Schema constrains all three enums, so an unrecognised
	// value never reaches a validated spec and is treated as unset
	// here rather than failing the encode.
	switch LegendKind(lg.Type) {
	case LegendKindSymbol, LegendKindGradient:
		c.Kind = LegendKind(lg.Type)
	}
	if lg.Direction == string(scene.LegendHorizontal) {
		c.Direction = scene.LegendHorizontal
	}
	if legendSymbolShapeKnown(scene.PointShape(lg.SymbolType)) {
		c.SymbolType = scene.PointShape(lg.SymbolType)
	}
	if lg.SymbolSize != nil && *lg.SymbolSize > 0 {
		sz := *lg.SymbolSize
		c.SymbolSize = &sz
	}
	return c
}

// legendSymbolShapeKnown reports whether s names a shape the swatch
// emitter can draw. The vocabulary is the point mark's own
// (scene.PointShapes) because the two are drawn by the same emitter,
// render/svg/symbols.go.
func legendSymbolShapeKnown(s scene.PointShape) bool {
	for _, k := range scene.PointShapes {
		if s == k {
			return true
		}
	}
	return false
}

// swatchFor builds the swatch one symbol-legend entry draws: the
// solid square every legend drew before E3-S4, or — once
// legend.symbol_type names a shape — a shaped symbol carrying the
// same colour. legend.symbol_size sizes either form.
func (c LegendContent) swatchFor(color *scene.Color) scene.SwatchSpec {
	sw := scene.SwatchSpec{Type: scene.SwatchSolid, Color: color}
	if c.SymbolType != "" {
		sw.Type, sw.Shape = scene.SwatchSymbol, c.SymbolType
	}
	if c.SymbolSize != nil {
		sw.Size = *c.SymbolSize
	}
	return sw
}

// legendTitleString accepts the polymorphic legend.title field — a
// string, or `false` to suppress — using the same convention
// axisTitleString applies to axis.title. Returns ("", true) when
// explicitly suppressed.
func legendTitleString(v any) (string, bool) { return axisTitleString(v) }

// legendTitle folds the content override onto the title the encoder
// derived from the bound field.
func legendTitle(derived string, c LegendContent) string {
	if c.TitleSet {
		return c.Title
	}
	return derived
}

// MaxChars is the label character budget the legend box is sized
// against. label_limit is a pixel width, so it converts through the
// same per-character estimate the truncation uses; with no limit the
// historical 14-character budget stands, which is what keeps every
// committed legend golden byte-identical.
func (c LegendContent) MaxChars() float64 {
	if c.LabelLimit != nil && *c.LabelLimit > 0 {
		return *c.LabelLimit / legendLabelCharW
	}
	return legendLabelMaxChars
}

// SelectCategories returns the indices of cats the legend shows, in
// the order it shows them. With no legend.values that is every
// category in its original order; otherwise it is the listed values,
// in the author's order, mapped back to their category index. A
// listed value matching no category is dropped.
//
// Returning *indices* rather than labels is the load-bearing part: a
// swatch colour is palette[index % len(palette)], indexed by the
// category's original position, so filtering the legend never
// re-colours it out of step with the marks.
func (c LegendContent) SelectCategories(cats []string) []int {
	if len(c.Values) == 0 {
		idx := make([]int, len(cats))
		for i := range cats {
			idx[i] = i
		}
		return idx
	}
	out := make([]int, 0, len(c.Values))
	taken := make(map[int]bool, len(c.Values))
	for _, v := range c.Values {
		for i, cat := range cats {
			if taken[i] || !legendValueMatches(v, cat) {
				continue
			}
			taken[i] = true
			out = append(out, i)
			break
		}
	}
	return out
}

// Label renders one entry value through legend.format. A category
// always arrives as a string (the palette partitioner works on
// strings), so a numeric-looking category is coerced first — that is
// what lets legend.format reach a symbol legend whose bound field is
// quantitative. With no format the value is rendered verbatim.
func (c LegendContent) Label(v any) string {
	if c.Format == nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	if s, ok := v.(string); ok {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return c.Format.Apply(f)
		}
		return c.Format.Apply(s)
	}
	return c.Format.Apply(v)
}

// Truncate shortens a label to legend.label_limit pixels, appending
// an ellipsis, using the same estimate axis labels use. A nil or
// non-positive limit returns the label untouched.
func (c LegendContent) Truncate(label string) string {
	if c.LabelLimit == nil || *c.LabelLimit <= 0 {
		return label
	}
	return truncateToWidth(label, *c.LabelLimit)
}

// legendValueMatches reports whether a legend.values entry names the
// given category. JSON gives no type discipline here — a category is
// always a string while the listed value may be a number or a bool —
// so the comparison is on the canonical rendering first and on
// numeric equality second (so 1 matches "1.0").
func legendValueMatches(v any, cat string) bool {
	if legendValueKey(v) == cat {
		return true
	}
	vf, ok := legendValueFloat(v)
	if !ok {
		return false
	}
	cf, err := strconv.ParseFloat(cat, 64)
	return err == nil && vf == cf
}

// legendValueKey renders a legend.values entry the way a category
// string would read.
func legendValueKey(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return fmt.Sprintf("%v", v)
}

// legendValueFloat coerces a legend.values entry to a float, covering
// the numeric shapes a decoded spec can carry.
func legendValueFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}
