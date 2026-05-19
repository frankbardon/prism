package encode

import "github.com/frankbardon/prism/encode/scene"

// Padding carries the per-side pixel padding around the plot region.
// P05 defaults: top 20, right 20, bottom 40 (x-axis room), left 40
// (y-axis room). Title (if present) adds 30 to top.
type Padding struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// Layout is the resolved frame + plot region for a single Scene.
// Frame = the outer SVG bounds; Plot = the inner rect marks render
// into.
type Layout struct {
	Frame   scene.Rect
	Plot    scene.Rect
	Padding Padding
}

// DefaultPadding returns the per-side padding constants P05 uses.
// hasTitle reserves extra top space (30 px) for a title text element.
// P06 will compute padding from theme + axis-label metrics rather
// than hard-coding.
func DefaultPadding(hasTitle bool) Padding {
	top := 20.0
	if hasTitle {
		top = 50.0
	}
	return Padding{Top: top, Right: 20, Bottom: 40, Left: 40}
}

// Compute returns the Layout for a width × height frame. Pure
// arithmetic. P05 ships only one variant; P06 introduces a richer
// LayoutOpts shape.
func Compute(width, height float64, hasTitle bool) Layout {
	pad := DefaultPadding(hasTitle)
	frame := scene.Rect{X: 0, Y: 0, W: width, H: height}
	plot := scene.Rect{
		X: pad.Left,
		Y: pad.Top,
		W: width - pad.Left - pad.Right,
		H: height - pad.Top - pad.Bottom,
	}
	return Layout{Frame: frame, Plot: plot, Padding: pad}
}
