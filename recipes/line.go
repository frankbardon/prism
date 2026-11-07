package recipes

import (
	"sort"

	"github.com/frankbardon/prism/spec"
)

// LineOptions configures Line.
type LineOptions struct {
	// X is the run column. XType is its channel type: "ordinal" (the
	// default — labels in XOrder, else sorted), "temporal" or
	// "quantitative".
	X     Field
	XType string
	// XOrder lists ordinal x labels left to right; empty sorts them.
	XOrder []string
	Y      Field
	// Series draws one coloured line per category.
	Series *Color
	// Points overlays a dot per row. A line path carries one tooltip,
	// so the per-row Tooltip lines ride on the dots; without Points the
	// tooltip is the first row's of each line.
	Points bool
	// PointSize is the dot area in px² (36 when zero).
	PointSize float64
	// Domain pins the y axis to [lo, hi].
	Domain  []float64
	Tooltip []Field
	Frame   Frame
}

// Line draws a line per series through the rows in x order.
func Line(rows []Row, o LineOptions) (*spec.Spec, error) {
	if err := require("Line", "X", o.X.Name, "Y", o.Y.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	xType := o.XType
	if xType == "" {
		xType = "ordinal"
	}
	seriesField := ""
	if o.Series != nil {
		seriesField = o.Series.Field.Name
	}
	cats := []string{seriesField}
	if xType == "ordinal" {
		cats = append(cats, o.X.Name)
	}
	rows = tidy(rows, cats...)
	x := func() *spec.PositionChannel {
		ch := position(o.X, xType)
		if xType == "ordinal" {
			order := o.XOrder
			if len(order) == 0 {
				order = distinct(rows, o.X.Name)
				sortStrings(order)
			}
			ch.Sort = anyList(order)
		}
		return ch
	}
	y := func() *spec.PositionChannel { return valueAxis(o.Y, o.Domain) }
	color := colorChannel(o.Series)

	line := newSpec()
	line.Mark = mark("line")
	line.Encoding = &spec.Encoding{X: x(), Y: y(), Color: color}
	if !o.Points {
		line.Encoding.Tooltip = tooltip(o.Tooltip)
		line.Data = data(rows)
		applyFrame(line, o.Frame, 0)
		return line, nil
	}
	size := o.PointSize
	if size <= 0 {
		size = 36
	}
	pts := newSpec()
	pts.Mark = markDef(spec.MarkDef{Type: "point", Size: num(size)})
	pts.Encoding = &spec.Encoding{X: x(), Y: y(), Color: withoutLegend(color), Tooltip: tooltip(o.Tooltip)}
	s := layer(rows, line, pts)
	applyFrame(s, o.Frame, 0)
	return s, nil
}

// Reference is a reference line drawn under a scatter's points:
// a rule across the plot at X or Y, or a polyline through Points
// (a y = x diagonal, a fitted or theoretical curve). Build one with
// RuleX, RuleY or Polyline.
type Reference struct {
	X, Y   *float64
	Points [][2]float64
	// Solid draws the line solid; references are dashed by default.
	Solid bool
	// Opacity (0 for 1) mutes a reference under the marks it frames.
	Opacity float64
}

// RuleX is a dashed vertical rule at x.
func RuleX(x float64) Reference { return Reference{X: &x} }

// RuleY is a dashed horizontal rule at y.
func RuleY(y float64) Reference { return Reference{Y: &y} }

// Polyline is a dashed line through points, in order.
func Polyline(points [][2]float64) Reference { return Reference{Points: points} }

// spec is the reference's layer, or nil when it draws nothing. Its own
// data puts the reference in the shared domains, so a reference is
// never clipped off an axis.
func (r Reference) spec() *spec.Spec {
	md := spec.MarkDef{}
	if !r.Solid {
		md.StrokeDash = []float64{4, 3}
	}
	if r.Opacity > 0 {
		md.Opacity = num(r.Opacity)
	}
	switch {
	case r.X != nil:
		s := referenceRule("x", *r.X)
		s.Mark.Def.StrokeDash, s.Mark.Def.Opacity = md.StrokeDash, md.Opacity
		return s
	case r.Y != nil:
		s := referenceRule("y", *r.Y)
		s.Mark.Def.StrokeDash, s.Mark.Def.Opacity = md.StrokeDash, md.Opacity
		return s
	case len(r.Points) >= 2:
		rows := make([]Row, len(r.Points))
		for i, p := range r.Points {
			rows[i] = Row{"x": p[0], "y": p[1], "order": i}
		}
		md.Type = "line"
		s := newSpec()
		s.Data = data(rows)
		s.Mark = markDef(md)
		// The order channel keeps the points in the sequence given, so
		// a curve that doubles back (a contour) is not re-sorted by x.
		s.Encoding = &spec.Encoding{
			X:     bare("x", "quantitative"),
			Y:     bare("y", "quantitative"),
			Order: &spec.OrderChannel{Single: &spec.OrderChannelEntry{Field: "order", Type: "quantitative"}},
		}
		return s
	}
	return nil
}

// ScatterOptions configures Scatter.
type ScatterOptions struct {
	X, Y  Field
	Color *Color
	// PointSize is the dot area in px² (24 when zero); Opacity its fill
	// opacity (0.6 when zero), so overplotted dots stay readable.
	PointSize float64
	Opacity   float64
	// XDomain / YDomain pin the axes to [lo, hi].
	XDomain, YDomain []float64
	// References draw under the points.
	References []Reference
	Tooltip    []Field
	Frame      Frame
}

// Scatter draws a dot per row on two quantitative axes, over any
// reference lines (a zero rule, a y = x diagonal, a QQ line — see
// NormalQQ — or a contour).
func Scatter(rows []Row, o ScatterOptions) (*spec.Spec, error) {
	if err := require("Scatter", "X", o.X.Name, "Y", o.Y.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	colorField := ""
	if o.Color != nil {
		colorField = o.Color.Field.Name
	}
	rows = tidy(rows, colorField)
	size, opacity := o.PointSize, o.Opacity
	if size <= 0 {
		size = 24
	}
	if opacity <= 0 {
		opacity = 0.6
	}
	pts := newSpec()
	pts.Mark = markDef(spec.MarkDef{Type: "point", Size: num(size), FillOpacity: num(opacity)})
	pts.Encoding = &spec.Encoding{
		X:       valueAxis(Field{Name: o.X.Name, Title: o.X.label()}, o.XDomain),
		Y:       valueAxis(Field{Name: o.Y.Name, Title: o.Y.label()}, o.YDomain),
		Color:   colorChannel(o.Color),
		Tooltip: tooltip(o.Tooltip),
	}
	var refs []*spec.Spec
	for _, r := range o.References {
		if s := r.spec(); s != nil {
			refs = append(refs, s)
		}
	}
	if len(refs) == 0 {
		pts.Data = data(rows)
		applyFrame(pts, o.Frame, 0)
		return pts, nil
	}
	s := layer(rows, append(refs, pts)...)
	applyFrame(s, o.Frame, 0)
	return s, nil
}

func sortStrings(xs []string) { sort.Strings(xs) }
