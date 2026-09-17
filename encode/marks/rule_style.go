package marks

import "github.com/frankbardon/prism/encode/scene"

// StrokeStyleFor adapts a mark style for a sub-mark drawn as a LINE.
//
// A composite mark (boxplot, bullet) hands its children the parent's
// resolved style. That style describes the parent's dominant shape,
// which for every composite Prism draws is an area: a box, a measure
// bar. So it carries a fill and, in every built-in theme, no stroke.
//
// An SVG <line> has no fill area. A line painted with only a fill is
// emitted at correct coordinates and draws NOTHING — which is how box
// plots shipped with a visible box and an invisible median, whiskers
// and caps, including in the committed gallery goldens.
//
// This returns a style that paints: the stroke is taken from the
// parent's fill when the parent set no stroke of its own, so the
// whiskers match the box rather than picking up an unrelated theme
// colour, and a zero stroke width becomes 1. The fill is left in place
// so the result matches what a standalone `rule` mark already emits
// (fill and stroke both present, the fill inert on a line).
//
// Paint indirection is preserved in the same precedence order the
// renderer reads: a themed gradient/pattern ref or a dark-variant CSS
// variable is carried across to the stroke rather than being flattened
// to a literal colour, because Fill is nil in exactly those cases and
// copying it alone would paint nothing.
func StrokeStyleFor(base scene.Style) scene.Style {
	out := base
	if out.Stroke == nil && out.StrokeRef == "" && out.StrokeVar == "" {
		out.Stroke = base.Fill
		out.StrokeRef = base.FillRef
		out.StrokeVar = base.FillVar
	}
	if out.StrokeWidth == 0 {
		out.StrokeWidth = 1
	}
	return out
}
