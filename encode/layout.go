package encode

import "github.com/frankbardon/prism/encode/scene"

// Padding carries the per-side pixel padding around the plot region.
// It is derived, never hand-written: LayoutOpts.Padding computes it
// from which sides actually carry chrome (an axis, an out-of-plot
// legend) plus the title reservation.
type Padding struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// Per-side reservation constants. The defaults are chosen so that the
// standard cartesian chart (x axis on the bottom, y axis on the left,
// no out-of-plot legend) lands on the historical fixed padding of
// {Top: 20, Right: 20, Bottom: 40, Left: 40} — the layout refactor is
// behaviour-preserving for default input.
//
// The metrics are fixed pixels rather than theme-derived label
// measurements: Prism has no text-measurement pass, and introducing
// one would move every plot rect in the repo. Consulting the
// --prism-axis-* label/tick tokens is deferred by design.
const (
	// layoutMargin is the breathing room every side of the plot rect
	// gets regardless of what chrome it carries.
	layoutMargin = 20.0
	// layoutAxisReserve is the extra space a side carrying an axis
	// needs for its tick marks, labels and title.
	layoutAxisReserve = 20.0
	// layoutTitleReserve is the extra top space a chart title needs.
	layoutTitleReserve = 30.0
	// layoutLegendReserve is the extra space a side carrying an
	// out-of-plot legend needs. Legends currently overlay the plot at
	// its top-right corner, so no caller reserves legend space yet;
	// the reservation takes effect the moment one sets Legend on a
	// side.
	layoutLegendReserve = 80.0
)

// SideChrome records which chrome occupies one side of the plot rect.
// A side may carry both an axis and a legend, in which case the two
// reservations add.
type SideChrome struct {
	Axis   bool
	Legend bool
}

// reserve returns the extra pixels this side needs beyond the base
// margin every side gets.
func (c SideChrome) reserve() float64 {
	r := 0.0
	if c.Axis {
		r += layoutAxisReserve
	}
	if c.Legend {
		r += layoutLegendReserve
	}
	return r
}

// LayoutSides is the per-side chrome map the padding derives from.
type LayoutSides struct {
	Top    SideChrome
	Right  SideChrome
	Bottom SideChrome
	Left   SideChrome
}

// markAxis flags the side pos names as carrying an axis. An empty
// position (no axis on that channel) flags nothing.
func (s *LayoutSides) markAxis(pos scene.AxisPosition) {
	switch pos {
	case scene.AxisPositionTop:
		s.Top.Axis = true
	case scene.AxisPositionRight:
		s.Right.Axis = true
	case scene.AxisPositionBottom:
		s.Bottom.Axis = true
	case scene.AxisPositionLeft:
		s.Left.Axis = true
	}
}

// AxisPlacement is the resolved side each position channel's axis
// occupies. It is the single source of truth for both the padding
// reservation and the scene.Axis.Position the axis builder stamps, so
// the two can never disagree.
type AxisPlacement struct {
	X scene.AxisPosition
	Y scene.AxisPosition
}

// DefaultAxisPlacement is Vega-Lite's default orientation: the x axis
// on the bottom, the y axis on the left.
func DefaultAxisPlacement() AxisPlacement {
	return AxisPlacement{X: scene.AxisPositionBottom, Y: scene.AxisPositionLeft}
}

// Sides reports which sides this placement's axes occupy.
func (p AxisPlacement) Sides() LayoutSides {
	var s LayoutSides
	s.markAxis(p.X)
	s.markAxis(p.Y)
	return s
}

// LayoutOpts is the input to Compute: the frame size plus what
// occupies each side of the plot rect.
type LayoutOpts struct {
	Width  float64
	Height float64
	// Title reserves extra top space for the chart title element.
	Title bool
	// Sides records which chrome occupies each side. Build it from an
	// AxisPlacement (see AxisPlacement.Sides) and set Legend on the
	// side an out-of-plot legend claims.
	Sides LayoutSides
}

// Padding derives the per-side padding from the reserved sides.
func (o LayoutOpts) Padding() Padding {
	top := layoutMargin + o.Sides.Top.reserve()
	if o.Title {
		top += layoutTitleReserve
	}
	return Padding{
		Top:    top,
		Right:  layoutMargin + o.Sides.Right.reserve(),
		Bottom: layoutMargin + o.Sides.Bottom.reserve(),
		Left:   layoutMargin + o.Sides.Left.reserve(),
	}
}

// Layout is the resolved frame + plot region for a single Scene.
// Frame = the outer SVG bounds; Plot = the inner rect marks render
// into.
type Layout struct {
	Frame   scene.Rect
	Plot    scene.Rect
	Padding Padding
}

// Compute returns the Layout for the requested frame. Pure
// arithmetic — the padding comes from opts.Padding, which reads the
// per-side reservations rather than any hard-coded asymmetry.
func Compute(opts LayoutOpts) Layout {
	return layoutFrom(opts.Width, opts.Height, opts.Padding())
}

// ComputeSparkline returns a Layout for a sparkline plot: 4-px
// padding all sides, no axis/legend/title reservation. See D067.
func ComputeSparkline(width, height float64) Layout {
	return layoutFrom(width, height, Padding{Top: 4, Right: 4, Bottom: 4, Left: 4})
}

// layoutFrom insets the frame by pad to produce the plot rect.
func layoutFrom(width, height float64, pad Padding) Layout {
	frame := scene.Rect{X: 0, Y: 0, W: width, H: height}
	plot := scene.Rect{
		X: pad.Left,
		Y: pad.Top,
		W: width - pad.Left - pad.Right,
		H: height - pad.Top - pad.Bottom,
	}
	return Layout{Frame: frame, Plot: plot, Padding: pad}
}
