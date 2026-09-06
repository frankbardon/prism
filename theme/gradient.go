package theme

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// GradientDef is a named gradient a theme can declare under
// Theme.Gradients. Style blocks reference an entry via url(#name)
// (theme.Theme.ResolveFillRef, E3-S2); render/svg emits the actual
// <linearGradient>/<radialGradient> def (E3-S3, see sceneDef +
// render/svg/style.go's writeGradientPatternDefs).
//
// Type selects the shape:
//   - "linear" uses Angle (degrees, 0 = left-to-right, clockwise) to
//     orient the gradient vector. CX/CY/Radius are ignored.
//   - "radial" uses CX/CY (center, fraction of the shape's bounding
//     box, 0-1) and Radius (fraction of the bounding box). Angle is
//     ignored.
//
// Stops holds at least two color stops; see GradientStop.
type GradientDef struct {
	Type   string         `json:"type"`
	Angle  *float64       `json:"angle,omitempty"`
	CX     *float64       `json:"cx,omitempty"`
	CY     *float64       `json:"cy,omitempty"`
	Radius *float64       `json:"radius,omitempty"`
	Stops  []GradientStop `json:"stops,omitempty"`
}

// GradientStop is one color stop in a gradient, at Offset (0-1)
// along the gradient vector/radius.
type GradientStop struct {
	Offset float64 `json:"offset"`
	Color  string  `json:"color"`
}

// Clone deep-copies a GradientDef.
func (g GradientDef) Clone() GradientDef {
	out := g
	out.Angle = copyFloat(g.Angle)
	out.CX = copyFloat(g.CX)
	out.CY = copyFloat(g.CY)
	out.Radius = copyFloat(g.Radius)
	if g.Stops != nil {
		out.Stops = append([]GradientStop(nil), g.Stops...)
	}
	return out
}

// sceneDef converts g into the scene-level Gradient shape consumed by
// render/svg's <linearGradient>/<radialGradient> emitters (E3-S3),
// doing the angle/center-radius math once here so the renderer only
// has to write attributes verbatim.
//
//   - "linear" converts Angle (default 0, degrees, 0 = left-to-right,
//     clockwise) into a unit vector centered in the [0,1] bounding
//     box — matches SVG's default objectBoundingBox gradientUnits, so
//     no gradientUnits attribute is needed.
//   - "radial" maps CX/CY (default 0.5 each) and Radius (default 0.5)
//     straight through as bounding-box fractions.
//
// A stop color that fails to parse as hex degrades to opaque black,
// mirroring the silent-fallback precedent in encode.applyThemeMarkStyle
// rather than dropping the stop (which would corrupt every remaining
// offset).
func (g GradientDef) sceneDef() scene.Gradient {
	stops := make([]scene.GradientStop, len(g.Stops))
	for i, s := range g.Stops {
		c, err := scene.ColorFromHex(s.Color)
		if err != nil {
			c = &scene.Color{A: 255}
		}
		stops[i] = scene.GradientStop{Offset: s.Offset, Color: *c}
	}
	if g.Type == "radial" {
		cx, cy, r := 0.5, 0.5, 0.5
		if g.CX != nil {
			cx = *g.CX
		}
		if g.CY != nil {
			cy = *g.CY
		}
		if g.Radius != nil {
			r = *g.Radius
		}
		return scene.Gradient{Type: "radial", Stops: stops, X1: cx, Y1: cy, X2: r}
	}
	angle := 0.0
	if g.Angle != nil {
		angle = *g.Angle
	}
	rad := angle * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	return scene.Gradient{
		Type:  "linear",
		Stops: stops,
		X1:    0.5 - dx/2,
		Y1:    0.5 - dy/2,
		X2:    0.5 + dx/2,
		Y2:    0.5 + dy/2,
	}
}
