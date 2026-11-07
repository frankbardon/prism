package recipes

import "github.com/frankbardon/prism/spec"

// ForestOptions configures Forest.
type ForestOptions struct {
	// Label is the row name column: one band per row, the first at the
	// top. Names must be distinct (a band scale merges equal ones).
	Label Field
	// Order lists the labels top to bottom; empty keeps row order.
	Order []string
	// Estimate is the point column; its Title titles the value axis.
	Estimate Field
	// Low / High are the interval bounds. A row missing either (or
	// both options empty) draws its point with no interval rule.
	Low, High string
	// Color colours the rules and points by a category (significance,
	// say). The rule layer never repeats the legend.
	Color *Color
	// References are dashed vertical rules (0 for "no effect", α for a
	// p-value panel). Each also widens the value domain to include it.
	References []float64
	// Domain pins the value axis to [lo, hi].
	Domain []float64
	// HideLabels drops the label axis (a one-row panel titled by its
	// own name, say).
	HideLabels bool
	// PointSize is the dot area in px² (64 when zero).
	PointSize float64
	// Tooltip lines, on both the rule and the point.
	Tooltip []Field
	// Frame; a zero Height is BandHeight(rows).
	Frame Frame
}

// Forest draws an interval (forest / coefficient) plot: per row, a
// rule from Low to High and a dot at Estimate, on one value axis, with
// dashed reference rules beneath. Rows with no finite Estimate keep
// their band (the label still lists them) but draw nothing.
//
// To split rows with different units onto separate value scales, build
// one Forest per group and Stack them; each panel then asks for its own
// BandHeight.
func Forest(rows []Row, o ForestOptions) (*spec.Spec, error) {
	if err := require("Forest", "Label", o.Label.Name, "Estimate", o.Estimate.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	colorField := ""
	if o.Color != nil {
		colorField = o.Color.Field.Name
	}
	rows = tidy(rows, o.Label.Name, colorField)
	order := o.Order
	if len(order) == 0 {
		order = distinct(rows, o.Label.Name)
	}

	label := func() *spec.PositionChannel {
		y := categoryAxis(o.Label, order, true)
		if o.HideLabels {
			y.AxisHidden = true
		} else {
			// The band labels name the rows; a title would repeat them.
			y.Axis = &spec.Axis{Title: ""}
		}
		return y
	}
	color := colorChannel(o.Color)
	tips := tooltip(o.Tooltip)

	var layers []*spec.Spec
	for _, ref := range o.References {
		layers = append(layers, referenceRule("x", ref))
	}
	if o.Low != "" && o.High != "" {
		if ci := withAll(rows, o.Low, o.High); len(ci) > 0 {
			rule := newSpec()
			rule.Data = data(ci)
			rule.Mark = mark("rule")
			rule.Encoding = &spec.Encoding{
				Y:       label(),
				X:       valueAxis(Field{Name: o.Low, Title: o.Estimate.label()}, o.Domain),
				X2:      bare(o.High, ""),
				Color:   withoutLegend(color),
				Tooltip: tips,
			}
			layers = append(layers, rule)
		}
	}
	size := o.PointSize
	if size <= 0 {
		size = 64
	}
	point := newSpec()
	point.Data = data(withAll(rows, o.Estimate.Name))
	point.Mark = markDef(spec.MarkDef{Type: "point", Size: num(size)})
	point.Encoding = &spec.Encoding{
		Y:       label(),
		X:       valueAxis(Field{Name: o.Estimate.Name, Title: o.Estimate.label()}, o.Domain),
		Color:   color,
		Tooltip: tips,
	}
	layers = append(layers, point)

	s := layer(rows, layers...)
	applyFrame(s, o.Frame, BandHeight(categoryCount(rows, o.Label.Name, order)))
	return s, nil
}

// referenceRule is a dashed rule at value on channel ("x": vertical,
// "y": horizontal). Its own one-row data also puts value in the shared
// domain, so a reference is never off the axis.
func referenceRule(channel string, value float64) *spec.Spec {
	const col = "value"
	s := newSpec()
	s.Data = data([]Row{{col: value}})
	s.Mark = markDef(spec.MarkDef{Type: "rule", StrokeDash: []float64{4, 3}})
	enc := &spec.Encoding{}
	if channel == "y" {
		enc.Y = bare(col, "quantitative")
	} else {
		enc.X = bare(col, "quantitative")
	}
	s.Encoding = enc
	return s
}
