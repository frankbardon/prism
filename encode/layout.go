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
	// needs for its tick marks, labels and title. It is the sum of
	// the per-component reserves below, which is what lets a
	// component suppressed by `axis.ticks: false` / `axis.labels:
	// false` (E3-S2) hand its share back to the plot rect.
	layoutAxisReserve = layoutAxisTickReserve + layoutAxisLabelReserve
	// layoutAxisTickReserve is the share of the axis reserve the tick
	// marks claim; layoutAxisLabelReserve is the share the tick labels
	// claim. The split is the renderer's default tick length (5 px)
	// against the remainder. Like every other metric here these are
	// fixed pixels: a spec- or theme-level `tick_size` /
	// `label_padding` larger than the default draws into the outer
	// margin rather than growing the reservation, which is the same
	// text-metric deferral E1-S1 recorded above.
	layoutAxisTickReserve  = 5.0
	layoutAxisLabelReserve = 15.0
	// layoutTitleReserve is the extra top space a chart title needs.
	layoutTitleReserve = 30.0
	// layoutLegendReserve is the fallback space a side carrying an
	// out-of-plot legend needs when the caller flags the side without
	// measuring the legend. E1-S1 shipped it as a placeholder; E1-S3
	// measured the real box (a symbol legend is 104 px wide plus its
	// offset gap) and made the encoder pass the measured extent
	// through SideChrome.LegendExtent, so this constant now only
	// covers a caller that knows a legend is coming but not how big.
	layoutLegendReserve = 120.0
)

// SideChrome records which chrome occupies one side of the plot rect.
// A side may carry both an axis and a legend, in which case the two
// reservations add.
type SideChrome struct {
	Axis bool
	// AxisReserve is the depth the axis on this side claims, already
	// net of any component suppressed by `axis.labels: false` /
	// `axis.ticks: false` (E3-S2). Set it through
	// LayoutSides.markAxis, which sources it from AxisPlacement —
	// zero with Axis set means the axis draws nothing at all and the
	// side keeps only the base margin.
	AxisReserve float64
	Legend      bool
	// LegendExtent is the measured width (left/right) or height
	// (top/bottom) the legend on this side claims, offset gap
	// included. Zero with Legend set falls back to
	// layoutLegendReserve.
	LegendExtent float64
}

// reserve returns the extra pixels this side needs beyond the base
// margin every side gets.
func (c SideChrome) reserve() float64 {
	r := 0.0
	if c.Axis {
		r += c.AxisReserve
	}
	if c.Legend {
		if c.LegendExtent > 0 {
			r += c.LegendExtent
		} else {
			r += layoutLegendReserve
		}
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

// markAxis flags the side pos names as carrying an axis claiming
// reserve pixels. An empty position (no axis on that channel) flags
// nothing.
func (s *LayoutSides) markAxis(pos scene.AxisPosition, reserve float64) {
	switch pos {
	case scene.AxisPositionTop:
		s.Top.Axis, s.Top.AxisReserve = true, reserve
	case scene.AxisPositionRight:
		s.Right.Axis, s.Right.AxisReserve = true, reserve
	case scene.AxisPositionBottom:
		s.Bottom.Axis, s.Bottom.AxisReserve = true, reserve
	case scene.AxisPositionLeft:
		s.Left.Axis, s.Left.AxisReserve = true, reserve
	}
}

// MarkLegend flags the side named by pos as carrying an out-of-plot
// legend claiming extent pixels. Corner placements overlay the plot
// and reserve nothing, so they are a no-op here.
func (s *LayoutSides) MarkLegend(pos scene.LegendPosition, extent float64) {
	switch pos {
	case scene.LegendTop:
		s.Top.Legend, s.Top.LegendExtent = true, extent
	case scene.LegendRight:
		s.Right.Legend, s.Right.LegendExtent = true, extent
	case scene.LegendBottom:
		s.Bottom.Legend, s.Bottom.LegendExtent = true, extent
	case scene.LegendLeft:
		s.Left.Legend, s.Left.LegendExtent = true, extent
	}
}

// AxisPlacement is the resolved side each position channel's axis
// occupies. It is the single source of truth for both the padding
// reservation and the scene.Axis.Position the axis builder stamps, so
// the two can never disagree.
//
// XHidden / YHidden carry the channel's `"axis": null` suppression
// (spec.PositionChannel.AxisHidden). A hidden axis occupies no side:
// Sides skips it, so the padding it would have reserved is released
// and the plot rect expands into the freed space.
type AxisPlacement struct {
	X       scene.AxisPosition
	Y       scene.AxisPosition
	XHidden bool
	YHidden bool
	// XReserve / YReserve are the depths each channel's axis claims on
	// the side it occupies (E3-S2). Nil — the zero value, and what a
	// bare AxisPlacement literal carries — means the full
	// layoutAxisReserve, so a caller that knows nothing about the
	// channel's axis block still reserves what it always did. A caller
	// that has resolved the channel's AxisOpts narrows them through
	// ReserveFrom, so suppressing the labels or the tick marks hands
	// that share back to the plot rect.
	XReserve *float64
	YReserve *float64
}

// DefaultAxisPlacement is Vega-Lite's default orientation: the x axis
// on the bottom, the y axis on the left, each reserving the full axis
// depth.
func DefaultAxisPlacement() AxisPlacement {
	return AxisPlacement{X: scene.AxisPositionBottom, Y: scene.AxisPositionLeft}
}

// ReserveFrom narrows each channel's reservation to what its resolved
// AxisOpts actually draws. Call it with the same opts the axis is
// later built from, so the padding can never disagree with the
// geometry.
func (p *AxisPlacement) ReserveFrom(x, y AxisOpts) {
	xr, yr := AxisSideReserve(x), AxisSideReserve(y)
	p.XReserve, p.YReserve = &xr, &yr
}

// axisReserveOr resolves an optional per-channel reservation, falling
// back to the full axis depth when the caller stated none.
func axisReserveOr(v *float64) float64 {
	if v == nil {
		return layoutAxisReserve
	}
	return *v
}

// AxisSideReserve returns the padding depth an axis drawn with opts
// needs on the side it occupies: the tick-mark share plus the label
// share, each released when `axis.ticks` / `axis.labels` suppress the
// component. The axis title rides inside the outer margin and is not
// separately reserved, as it never has been.
func AxisSideReserve(opts AxisOpts) float64 {
	r := 0.0
	if opts.Ticks {
		r += layoutAxisTickReserve
	}
	if opts.Labels {
		r += layoutAxisLabelReserve
	}
	return r
}

// Sides reports which sides this placement's axes occupy. A hidden
// axis claims nothing.
func (p AxisPlacement) Sides() LayoutSides {
	var s LayoutSides
	if !p.XHidden {
		s.markAxis(p.X, axisReserveOr(p.XReserve))
	}
	if !p.YHidden {
		s.markAxis(p.Y, axisReserveOr(p.YReserve))
	}
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

// LegendBand returns the pixel depth of the strip reserved between
// the plot edge on side pos and the outer margin — the axis
// reservation plus the legend extent. A side legend anchors its far
// edge there, which is what keeps it clear of the axis chrome. The
// top band excludes the title reservation so a top legend never
// lands under the chart title.
func (p Padding) LegendBand(pos scene.LegendPosition, hasTitle bool) float64 {
	band := 0.0
	switch pos {
	case scene.LegendLeft:
		band = p.Left - layoutMargin
	case scene.LegendRight:
		band = p.Right - layoutMargin
	case scene.LegendBottom:
		band = p.Bottom - layoutMargin
	case scene.LegendTop:
		band = p.Top - layoutMargin
		if hasTitle {
			band -= layoutTitleReserve
		}
	}
	if band < 0 {
		return 0
	}
	return band
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
