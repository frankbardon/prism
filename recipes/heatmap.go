package recipes

import "github.com/frankbardon/prism/spec"

// HeatmapOptions configures Heatmap.
type HeatmapOptions struct {
	// X and Y are the two category columns; Value shades the cell.
	X, Y  Field
	Value Field
	// XOrder lists the columns left to right, YOrder the rows top to
	// bottom; empty keeps row order.
	XOrder, YOrder []string
	// Domain pins the colour ramp to [lo, hi] — [-1, 1] centres a
	// correlation on a diverging ramp's midpoint; values outside take
	// the nearer end. Empty spans the values' own extent.
	Domain []float64
	// Scheme names the ramp (a built-in or theme scheme); Reverse
	// flips it.
	Scheme  string
	Reverse bool
	// Labels prints each cell's value through Value.Format, or the
	// LabelField column when set (a value with a significance star,
	// say). Labelled cells draw at CellOpacity (0.6 when zero) so the
	// text stays legible on both ends of the ramp.
	Labels      bool
	LabelField  string
	CellOpacity float64
	// Legend shows the gradient legend right of the plot.
	Legend bool
	// XAxisTop puts the column labels above the grid.
	XAxisTop bool
	Tooltip  []Field
	// Frame; a zero Height gives each row HeatmapRowHeight.
	Frame Frame
}

// HeatmapRowHeight is the default height of one heatmap row.
const HeatmapRowHeight = 28.0

// heatmapLegendHeight is the least default height that fits the
// gradient legend (title, 104 px ramp, end labels).
const heatmapLegendHeight = 230.0

// Heatmap draws a cell per row at (X, Y), shaded by Value, optionally
// lettered with its value.
func Heatmap(rows []Row, o HeatmapOptions) (*spec.Spec, error) {
	if err := require("Heatmap", "X", o.X.Name, "Y", o.Y.Name, "Value", o.Value.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	rows = withAll(tidy(rows, o.X.Name, o.Y.Name), o.Value.Name)
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	x := func() *spec.PositionChannel {
		ch := categoryAxis(o.X, o.XOrder, false)
		ch.Axis = &spec.Axis{Grid: flag(false)}
		if o.XAxisTop {
			ch.Axis.Orient = "top"
		}
		return ch
	}
	y := func() *spec.PositionChannel {
		ch := categoryAxis(o.Y, o.YOrder, true)
		ch.Axis = &spec.Axis{Grid: flag(false)}
		return ch
	}
	color := &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: o.Value.Name, Type: "quantitative", Title: o.Value.Title}}
	if len(o.Domain) == 2 || o.Scheme != "" || o.Reverse {
		color.Scale = &spec.Scale{Scheme: o.Scheme}
		if len(o.Domain) == 2 {
			color.Scale.Domain = []any{o.Domain[0], o.Domain[1]}
		}
		if o.Reverse {
			color.Scale.Reverse = flag(true)
		}
	}
	if o.Legend {
		color.Legend = &spec.Legend{Orient: "right"}
	} else {
		color.LegendHidden = true
	}
	tips := tooltip(o.Tooltip)

	cells := newSpec()
	md := spec.MarkDef{Type: "heatmap"}
	if o.Labels {
		op := o.CellOpacity
		if op <= 0 {
			op = 0.6
		}
		md.FillOpacity = num(op)
	}
	cells.Mark = markDef(md)
	cells.Encoding = &spec.Encoding{X: x(), Y: y(), Color: color, Tooltip: tips}

	height := float64(categoryCount(rows, o.Y.Name, o.YOrder))*HeatmapRowHeight + PanelChrome
	if o.Legend && height < heatmapLegendHeight {
		// The gradient legend is a fixed-length ramp; a short grid
		// would clip its lower labels.
		height = heatmapLegendHeight
	}
	if !o.Labels {
		cells.Data = data(rows)
		applyFrame(cells, o.Frame, height)
		return cells, nil
	}
	text := &spec.TextChannel{Field: o.Value.Name, Format: o.Value.Format}
	if o.LabelField != "" {
		text = &spec.TextChannel{Field: o.LabelField}
	}
	labels := newSpec()
	labels.Mark = markDef(spec.MarkDef{Type: "text", FontSize: num(11)})
	labels.Encoding = &spec.Encoding{X: x(), Y: y(), Text: text, Tooltip: tips}
	s := layer(rows, cells, labels)
	applyFrame(s, o.Frame, height)
	return s, nil
}
