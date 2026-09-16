package encode

import (
	"fmt"
	"strconv"

	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/spec"
)

// LegendContent is the resolved *content* half of a channel's
// `legend` block: what the legend is titled, which entries it shows,
// how their labels read and how wide a label may grow before it is
// truncated. It is the companion of LegendPlacement, which resolves
// the *placement* half (E1-S3).
//
// Everything that reads a spec.Legend content field goes through this
// type, and every frame / margin measurement goes through LegendBox.
// Those two are the seam a follow-up extends: E3-S4's `direction`
// belongs on LegendBox (which already feeds the frame, both builders
// and the side reservation from one place), and `type` /
// `symbol_type` / `symbol_size` belong here alongside Format and
// Values.
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
	return c
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
