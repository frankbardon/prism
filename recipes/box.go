package recipes

import "github.com/frankbardon/prism/spec"

// BoxOptions configures Box. Every statistic is a column of
// precomputed values: the recipe draws summaries, it never computes
// them, so a box over a million rows inlines one row per box.
type BoxOptions struct {
	// Category is one horizontal box per row, the first at the top.
	Category Field
	Order    []string
	// Value titles the value axis (its Name is not read).
	Value Field
	// WhiskerLow / WhiskerHigh draw the whisker rule, Q1 / Q3 the
	// translucent interquartile bar, Center the dot (a median or a
	// mean). Leave a pair (or Center) empty to omit that part.
	WhiskerLow, WhiskerHigh string
	Q1, Q3                  string
	Center                  string
	Tooltip                 []Field
	// Frame; a zero Height is BandHeight(rows).
	Frame Frame
}

// Box draws horizontal summary boxes — whisker rule, interquartile
// bar, centre dot — from precomputed statistics; a mean ± SD chart is
// the same recipe with only whiskers and a centre.
func Box(rows []Row, o BoxOptions) (*spec.Spec, error) {
	if err := require("Box", "Category", o.Category.Name); err != nil {
		return nil, err
	}
	whiskers := o.WhiskerLow != "" && o.WhiskerHigh != ""
	box := o.Q1 != "" && o.Q3 != ""
	if !whiskers && !box && o.Center == "" {
		return nil, errNeedOne("Box", "WhiskerLow/WhiskerHigh, Q1/Q3 or Center")
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	rows = tidy(rows, o.Category.Name)
	order := o.Order
	if len(order) == 0 {
		order = distinct(rows, o.Category.Name)
	}
	y := func() *spec.PositionChannel {
		ch := categoryAxis(o.Category, order, true)
		return ch
	}
	axisTitle := o.Value.label()
	tips := tooltip(o.Tooltip)
	var layers []*spec.Spec
	add := func(md spec.MarkDef, lo, hi string) {
		s := newSpec()
		s.Mark = markDef(md)
		s.Encoding = &spec.Encoding{Y: y(), X: valueAxis(Field{Name: lo, Title: axisTitle}, nil), Tooltip: tips}
		if hi != "" {
			s.Encoding.X2 = bare(hi, "")
		}
		layers = append(layers, s)
	}
	if whiskers {
		add(spec.MarkDef{Type: "rule"}, o.WhiskerLow, o.WhiskerHigh)
	}
	if box {
		add(spec.MarkDef{Type: "bar", FillOpacity: num(0.35)}, o.Q1, o.Q3)
	}
	if o.Center != "" {
		add(spec.MarkDef{Type: "point", Size: num(64)}, o.Center, "")
	}
	s := layer(rows, layers...)
	applyFrame(s, o.Frame, BandHeight(categoryCount(rows, o.Category.Name, order)))
	return s, nil
}
