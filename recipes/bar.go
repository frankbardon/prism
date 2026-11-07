package recipes

import "github.com/frankbardon/prism/spec"

// BarOptions configures Bar.
type BarOptions struct {
	// Category is the band column; Value the bar length.
	Category Field
	Value    Field
	// Order lists the categories (left to right, or top to bottom when
	// Horizontal); empty keeps row order. Sort the rows, or list the
	// order, to rank bars by value.
	Order []string
	// Horizontal lays the bars along x, one band per row, the first at
	// the top.
	Horizontal bool
	// Low / High draw an interval rule over each bar whose row has
	// both bounds (a confidence interval, say).
	Low, High string
	// Color colours bars by a category.
	Color *Color
	// Domain pins the value axis to [lo, hi].
	Domain []float64
	// Tooltip lines, on the bars and the interval rules.
	Tooltip []Field
	// Frame; a zero Height is BandHeight(categories) when Horizontal.
	Frame Frame
}

// Bar draws one bar per row from the value baseline, optionally with
// interval rules over the bars.
func Bar(rows []Row, o BarOptions) (*spec.Spec, error) {
	if err := require("Bar", "Category", o.Category.Name, "Value", o.Value.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	colorField := ""
	if o.Color != nil {
		colorField = o.Color.Field.Name
	}
	rows = tidy(rows, o.Category.Name, colorField)
	cat := func() *spec.PositionChannel { return categoryAxis(o.Category, o.Order, o.Horizontal) }
	val := valueAxis(o.Value, o.Domain)
	tips := tooltip(o.Tooltip)

	bar := newSpec()
	bar.Mark = mark("bar")
	bar.Encoding = &spec.Encoding{Color: colorChannel(o.Color), Tooltip: tips}
	place(bar.Encoding, cat(), val, o.Horizontal)

	height := 0.0
	if o.Horizontal {
		height = BandHeight(categoryCount(rows, o.Category.Name, o.Order))
	}
	var ci []Row
	if o.Low != "" && o.High != "" {
		ci = withAll(rows, o.Low, o.High)
	}
	if len(ci) == 0 {
		bar.Data = data(rows)
		applyFrame(bar, o.Frame, height)
		return bar, nil
	}
	rule := newSpec()
	rule.Data = data(ci)
	rule.Mark = markDef(spec.MarkDef{Type: "rule", StrokeWidth: num(2)})
	rule.Encoding = &spec.Encoding{Tooltip: tips}
	lo := bare(o.Low, "quantitative")
	place(rule.Encoding, cat(), lo, o.Horizontal)
	if o.Horizontal {
		rule.Encoding.X2 = bare(o.High, "")
	} else {
		rule.Encoding.Y2 = bare(o.High, "")
	}
	s := layer(rows, bar, rule)
	applyFrame(s, o.Frame, height)
	return s, nil
}

// place binds the category and value channels to x / y.
func place(enc *spec.Encoding, cat, val *spec.PositionChannel, horizontal bool) {
	if horizontal {
		enc.Y, enc.X = cat, val
		return
	}
	enc.X, enc.Y = cat, val
}

// GroupedBarOptions configures GroupedBar.
type GroupedBarOptions struct {
	// Category is the outer band; Group subdivides each band into one
	// bar per group (dodged, never stacked) and colours it.
	Category Field
	Group    Field
	Value    Field
	// Order / GroupOrder list the categories and the groups; empty
	// keeps row order for the categories and sorts the groups.
	Order      []string
	GroupOrder []string
	// Horizontal dodges along y, the first category at the top.
	Horizontal bool
	// Scheme / Range colour the groups (see Color).
	Scheme string
	Range  []string
	// HideLegend drops the group legend.
	HideLegend bool
	// Domain pins the value axis to [lo, hi].
	Domain  []float64
	Tooltip []Field
	Frame   Frame
}

// GroupedBar draws bars side by side within each category, one per
// group, through Prism's x_offset (y_offset when Horizontal) channel.
func GroupedBar(rows []Row, o GroupedBarOptions) (*spec.Spec, error) {
	if err := require("GroupedBar", "Category", o.Category.Name, "Group", o.Group.Name, "Value", o.Value.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	rows = tidy(rows, o.Category.Name, o.Group.Name)
	offset := &spec.OffsetChannel{Field: o.Group.Name, Type: "nominal"}
	if len(o.GroupOrder) > 0 {
		offset.Scale = &spec.Scale{Domain: anyList(o.GroupOrder)}
	}
	s := newSpec()
	s.Data = data(rows)
	s.Mark = mark("bar")
	s.Encoding = &spec.Encoding{
		Color: colorChannel(&Color{
			Field: o.Group, Domain: groupDomain(rows, o.Group.Name, o.GroupOrder),
			Scheme: o.Scheme, Range: o.Range, HideLegend: o.HideLegend,
		}),
		Tooltip: tooltip(o.Tooltip),
	}
	place(s.Encoding, categoryAxis(o.Category, o.Order, o.Horizontal), valueAxis(o.Value, o.Domain), o.Horizontal)
	if o.Horizontal {
		s.Encoding.YOffset = offset
	} else {
		s.Encoding.XOffset = offset
	}
	height := 0.0
	if o.Horizontal {
		height = BandHeight(categoryCount(rows, o.Category.Name, o.Order))
	}
	applyFrame(s, o.Frame, height)
	return s, nil
}

// groupDomain is the colour order of the groups: the order the offset
// draws them in (GroupOrder, else ascending, the offset's default), so
// the legend reads in the same order the bars stand.
func groupDomain(rows []Row, col string, order []string) []string {
	if len(order) > 0 {
		return order
	}
	groups := distinct(rows, col)
	sortStrings(groups)
	return groups
}
