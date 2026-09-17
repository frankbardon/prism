package svg

import (
	"github.com/frankbardon/prism/encode/scene"
)

// The point-mark symbol vocabulary (scene.PointShape) has exactly one
// emitter, and it lives here. Both the point mark (renderPoint) and a
// legend's shaped swatch (render/svg/legends.go, SwatchSymbol) draw
// through it, so a point's diamond and a legend swatch's diamond are
// the same geometry by construction rather than by two call sites
// agreeing. Every coordinate routes through writeSegTo, which pins it
// to render.FormatFloat's 3-decimal precision.
//
// A circle keeps its own element — <circle cx cy r> — because that is
// what every committed golden holds; only the non-circle shapes
// become a <path>.

// symbolCrossArm is the half-width of a cross's arms as a fraction of
// its radius. A third gives the plus sign roughly equal-weight strokes
// against its 2r extent.
const symbolCrossArm = 1.0 / 3.0

// symbolTag returns the SVG element name a shape is drawn with. An
// empty shape reads as a circle, which is what an IR that predates
// PointGeom.Shape carries.
func symbolTag(shape scene.PointShape) string {
	if shape == "" || shape == scene.ShapeCircle {
		return "circle"
	}
	return "path"
}

// writeSymbolGeom writes the geometry attributes for one symbol,
// centred at (cx, cy) with bounding radius r, onto an element already
// opened with the tag symbolTag returned for the same shape.
//
// An unrecognised shape falls back to a circle — the JSON Schema
// (schema/v1/legend.schema.json, mark.schema.json) constrains the
// enum, so a validated spec never reaches here with one.
func writeSymbolGeom(w *Writer, shape scene.PointShape, cx, cy, r float64) {
	switch shape {
	case scene.ShapeSquare:
		writeSymbolPath(w, [][2]float64{
			{cx - r, cy - r}, {cx + r, cy - r}, {cx + r, cy + r}, {cx - r, cy + r},
		})
	case scene.ShapeTriangle:
		writeSymbolPath(w, [][2]float64{
			{cx, cy - r}, {cx + r, cy + r}, {cx - r, cy + r},
		})
	case scene.ShapeDiamond:
		writeSymbolPath(w, [][2]float64{
			{cx, cy - r}, {cx + r, cy}, {cx, cy + r}, {cx - r, cy},
		})
	case scene.ShapeCross:
		a := r * symbolCrossArm
		writeSymbolPath(w, [][2]float64{
			{cx - a, cy - r}, {cx + a, cy - r}, {cx + a, cy - a},
			{cx + r, cy - a}, {cx + r, cy + a}, {cx + a, cy + a},
			{cx + a, cy + r}, {cx - a, cy + r}, {cx - a, cy + a},
			{cx - r, cy + a}, {cx - r, cy - a}, {cx - a, cy - a},
		})
	default:
		w.AttrFloat("cx", cx)
		w.AttrFloat("cy", cy)
		w.AttrFloat("r", r)
	}
}

// writeSymbolPath emits a closed polygon as a `d` attribute.
func writeSymbolPath(w *Writer, pts [][2]float64) {
	if len(pts) == 0 {
		return
	}
	w.OpenAttr("d")
	writeSegTo(w, "M", pts[0][0], pts[0][1])
	for _, p := range pts[1:] {
		writeSegTo(w, " L", p[0], p[1])
	}
	w.Raw(" Z")
	w.CloseAttr()
}
