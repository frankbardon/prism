package theme

// GradientDef is a named gradient a theme can declare under
// Theme.Gradients. Style blocks will reference an entry via
// url(#name) once Fill/Stroke/Background resolution wires it up
// (E3-S2); actual <linearGradient>/<radialGradient> SVG emission
// lands in E3-S3. This story is model + validation only.
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
