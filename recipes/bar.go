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
	// Start, when set, makes each bar a span from the Start column to
	// Value instead of from the zero baseline (a range, or a bullet's
	// observed bar beside a narrow expected one). Value's Title still
	// titles the value axis. Spans in one category overlap in data
	// order, the later on top; colour them by a series with Color.
	Start string
	// Low / High draw an interval rule over each bar whose row has
	// both bounds (a confidence interval, say).
	Low, High string
	// Color colours bars by a category.
	Color *Color
	// Bands shade a span of the value axis behind the bars, across
	// every category (a ±1.96 band a z-score falls in by chance, say).
	// The bands' bounds join the value domain.
	Bands []Band
	// References are dashed rules across the value axis, drawn over
	// the bars (a baseline of 0, or 100 for an index). Each joins the
	// value domain.
	References []float64
	// Labels prints each bar's value through Value.Format at the bar's
	// end, or the LabelField column when set (a value with a
	// significance star, say). Pin a Domain with headroom past the
	// outer bars to keep their labels inside the plot.
	Labels     bool
	LabelField string
	// Domain pins the value axis to [lo, hi].
	Domain []float64
	// Tooltip lines, on the bars, the interval rules and the labels.
	Tooltip []Field
	// Frame; a zero Height is BandHeight(categories) when Horizontal.
	Frame Frame
}

// Band is a shaded span of a value axis, from Low to High. Opacity is
// its fill opacity (0.15 when zero).
type Band struct {
	Low, High float64
	Opacity   float64
}

// bandOpacity is a Band's fill opacity when it leaves it zero.
const bandOpacity = 0.15

// Bar draws one bar per row from the value baseline (or from Start),
// optionally over shaded bands and with interval rules, dashed
// references and value labels over the bars.
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
	// value binds a column on the value axis, titled as Value and
	// pinned to Domain, so whichever layer comes first the shared
	// axis reads the same.
	value := func(col string) *spec.PositionChannel {
		return valueAxis(Field{Name: col, Title: o.Value.Title}, o.Domain)
	}
	tips := tooltip(o.Tooltip)

	bar := newSpec()
	bar.Mark = mark("bar")
	bar.Encoding = &spec.Encoding{Color: colorChannel(o.Color), Tooltip: tips}
	if o.Start == "" {
		place(bar.Encoding, cat(), valueAxis(o.Value, o.Domain), o.Horizontal)
	} else {
		place(bar.Encoding, cat(), value(o.Start), o.Horizontal)
		end(bar.Encoding, o.Value.Name, o.Horizontal)
	}

	height := 0.0
	if o.Horizontal {
		height = BandHeight(categoryCount(rows, o.Category.Name, o.Order))
	}
	var layers []*spec.Spec
	for _, b := range o.Bands {
		layers = append(layers, bandLayer(rows, o, b, cat(), value))
	}
	layers = append(layers, bar)
	if o.Low != "" && o.High != "" {
		if ci := withAll(rows, o.Low, o.High); len(ci) > 0 {
			rule := newSpec()
			rule.Data = data(ci)
			rule.Mark = markDef(spec.MarkDef{Type: "rule", StrokeWidth: num(2)})
			rule.Encoding = &spec.Encoding{Tooltip: tips}
			place(rule.Encoding, cat(), bare(o.Low, "quantitative"), o.Horizontal)
			end(rule.Encoding, o.High, o.Horizontal)
			layers = append(layers, rule)
		}
	}
	refChannel := "y"
	if o.Horizontal {
		refChannel = "x"
	}
	for _, ref := range o.References {
		layers = append(layers, referenceRule(refChannel, ref))
	}
	if o.Labels {
		if lettered := withAll(rows, o.Value.Name); len(lettered) > 0 {
			text := &spec.TextChannel{Field: o.Value.Name, Format: o.Value.Format}
			if o.LabelField != "" {
				text = &spec.TextChannel{Field: o.LabelField}
			}
			labels := newSpec()
			labels.Data = data(lettered)
			labels.Mark = markDef(spec.MarkDef{Type: "text", FontSize: num(11)})
			labels.Encoding = &spec.Encoding{Text: text, Tooltip: tips}
			place(labels.Encoding, cat(), value(o.Value.Name), o.Horizontal)
			layers = append(layers, labels)
		}
	}
	if len(layers) == 1 {
		bar.Data = data(rows)
		applyFrame(bar, o.Frame, height)
		return bar, nil
	}
	s := layer(rows, layers...)
	applyFrame(s, o.Frame, height)
	return s, nil
}

// end binds a span's far end to x2 (horizontal) or y2.
func end(enc *spec.Encoding, col string, horizontal bool) {
	if horizontal {
		enc.X2 = bare(col, "")
	} else {
		enc.Y2 = bare(col, "")
	}
}

// bandLayer shades b across every category: one Low→High span per
// band, on its own rows so the bar data is untouched.
func bandLayer(rows []Row, o BarOptions, b Band, cat *spec.PositionChannel, value func(string) *spec.PositionChannel) *spec.Spec {
	cats := append([]string(nil), o.Order...)
	listed := map[string]bool{}
	for _, c := range cats {
		listed[c] = true
	}
	for _, c := range distinct(rows, o.Category.Name) {
		if !listed[c] {
			cats = append(cats, c)
		}
	}
	lowCol, highCol := freeName("band_low", o.Category.Name), freeName("band_high", o.Category.Name)
	spans := make([]Row, len(cats))
	for i, c := range cats {
		spans[i] = Row{o.Category.Name: c, lowCol: b.Low, highCol: b.High}
	}
	op := b.Opacity
	if op <= 0 {
		op = bandOpacity
	}
	s := newSpec()
	s.Data = data(spans)
	s.Mark = markDef(spec.MarkDef{Type: "bar", FillOpacity: num(op)})
	s.Encoding = &spec.Encoding{}
	place(s.Encoding, cat, value(lowCol), o.Horizontal)
	end(s.Encoding, highCol, o.Horizontal)
	return s
}

// freeName is want, suffixed with underscores until it differs from
// every taken column.
func freeName(want string, taken ...string) string {
	for {
		clash := false
		for _, t := range taken {
			if t == want {
				clash = true
				break
			}
		}
		if !clash {
			return want
		}
		want += "_"
	}
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
