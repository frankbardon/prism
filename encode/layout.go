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
	Axis   bool
	Legend bool
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
		r += layoutAxisReserve
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
}

// DefaultAxisPlacement is Vega-Lite's default orientation: the x axis
// on the bottom, the y axis on the left.
func DefaultAxisPlacement() AxisPlacement {
	return AxisPlacement{X: scene.AxisPositionBottom, Y: scene.AxisPositionLeft}
}

// DefaultAxisPosition is the side channel's axis occupies when the
// spec sets no `axis.orient`: bottom for x, left for y. Any other
// channel has no cartesian axis and yields the empty position, which
// markAxis treats as "no side".
func DefaultAxisPosition(channel scene.Channel) scene.AxisPosition {
	switch channel {
	case scene.ChannelX:
		return scene.AxisPositionBottom
	case scene.ChannelY:
		return scene.AxisPositionLeft
	}
	return ""
}

// AxisPositionFor resolves a spec `axis.orient` value into the scene
// position for channel. An x axis may sit top or bottom and a y axis
// left or right; an empty orient, or one meaningless for the channel
// ("left" on x), falls back to the channel's default side. The
// meaningless case is an author error rejected by PRISM_SPEC_044
// during validation — the fallback here only keeps the encoder total
// for callers that skipped semantic validation.
//
// This is the single orient → position mapping: AxisPlacement is
// built from it, and the placement is what both the padding
// reservation and the built axis read.
func AxisPositionFor(channel scene.Channel, orient string) scene.AxisPosition {
	switch channel {
	case scene.ChannelX:
		switch orient {
		case string(scene.AxisPositionTop):
			return scene.AxisPositionTop
		case string(scene.AxisPositionBottom):
			return scene.AxisPositionBottom
		}
	case scene.ChannelY:
		switch orient {
		case string(scene.AxisPositionLeft):
			return scene.AxisPositionLeft
		case string(scene.AxisPositionRight):
			return scene.AxisPositionRight
		}
	}
	return DefaultAxisPosition(channel)
}

// Sides reports which sides this placement's axes occupy. A hidden
// axis claims nothing.
func (p AxisPlacement) Sides() LayoutSides {
	var s LayoutSides
	if !p.XHidden {
		s.markAxis(p.X)
	}
	if !p.YHidden {
		s.markAxis(p.Y)
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
