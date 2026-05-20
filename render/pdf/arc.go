//go:build !js

package pdf

import "math"

// arcSegment is one cubic-bezier segment of an arc decomposition.
// X0 / Y0 is the start point; (C1X, C1Y) and (C2X, C2Y) are the two
// control points; (X1, Y1) is the end point.
type arcSegment struct {
	X0, Y0   float64
	C1X, C1Y float64
	C2X, C2Y float64
	X1, Y1   float64
}

// arcToBeziers converts a circular / elliptical arc into a sequence
// of cubic-bezier segments suitable for emission via gopdf.Curve.
// Each segment covers at most pi/2 radians; the caller composes the
// chain by drawing them in order.
//
// Parameters mirror the SVG center-parameterised arc form:
//   - (cx, cy)         center
//   - rx, ry           x / y radii (positive)
//   - startAngle       starting angle in radians (0 = +X axis)
//   - deltaAngle       signed sweep in radians (positive = CCW in
//     math coords, which is CW in PDF's y-down
//     page coords — callers compose the page-
//     direction sign before calling)
//   - xAxisRotation    rotation of the ellipse's x-axis in radians
//
// Maximum 90° per segment per the standard W3C SVG implementation
// note F.6.5; this is the closed-form approximation that bounds
// error below 0.0003r for r ≤ 1 (good to ~3 decimals at any
// scale we plot).
func arcToBeziers(cx, cy, rx, ry, startAngle, deltaAngle, xAxisRotation float64) []arcSegment {
	if rx == 0 || ry == 0 || deltaAngle == 0 {
		return nil
	}

	// Split into ≤ pi/2 chunks. ceil(|delta| / (pi/2)) segments.
	const quarter = math.Pi / 2
	n := int(math.Ceil(math.Abs(deltaAngle) / quarter))
	if n < 1 {
		n = 1
	}
	segDelta := deltaAngle / float64(n)

	out := make([]arcSegment, 0, n)
	for i := 0; i < n; i++ {
		theta1 := startAngle + float64(i)*segDelta
		theta2 := theta1 + segDelta
		out = append(out, oneArcSegment(cx, cy, rx, ry, theta1, theta2, xAxisRotation))
	}
	return out
}

// oneArcSegment computes one ≤90° cubic-bezier approximation of an
// elliptical arc from theta1 to theta2 (radians, signed). Standard
// formula: alpha = sin(delta) * (sqrt(4 + 3*tan²(delta/2)) - 1) / 3
// where delta = theta2 - theta1.
func oneArcSegment(cx, cy, rx, ry, theta1, theta2, phi float64) arcSegment {
	delta := theta2 - theta1
	alpha := math.Sin(delta) * (math.Sqrt(4+3*math.Pow(math.Tan(delta/2), 2)) - 1) / 3

	cos1, sin1 := math.Cos(theta1), math.Sin(theta1)
	cos2, sin2 := math.Cos(theta2), math.Sin(theta2)

	// Start point on the ellipse axis-aligned.
	px0 := rx * cos1
	py0 := ry * sin1
	// End point.
	px3 := rx * cos2
	py3 := ry * sin2
	// First control point.
	px1 := px0 - alpha*rx*sin1
	py1 := py0 + alpha*ry*cos1
	// Second control point.
	px2 := px3 + alpha*rx*sin2
	py2 := py3 - alpha*ry*cos2

	// Apply ellipse rotation phi, then translate by center.
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)
	rot := func(x, y float64) (float64, float64) {
		return cx + x*cosPhi - y*sinPhi, cy + x*sinPhi + y*cosPhi
	}
	x0, y0 := rot(px0, py0)
	c1x, c1y := rot(px1, py1)
	c2x, c2y := rot(px2, py2)
	x1, y1 := rot(px3, py3)
	return arcSegment{
		X0: x0, Y0: y0,
		C1X: c1x, C1Y: c1y,
		C2X: c2x, C2Y: c2y,
		X1: x1, Y1: y1,
	}
}

// endpointToCenter converts the SVG endpoint-arc form to center
// parameter form. Used by the path-mini parser's A / a command.
//
// Inputs:
//   - (x1, y1), (x2, y2): start and end points of the arc
//   - rx, ry: radii (may be adjusted if too small)
//   - phi: x-axis rotation (radians)
//   - largeArcFlag: SVG large-arc flag
//   - sweepFlag: SVG sweep flag
//
// Outputs:
//   - (cx, cy): center of the ellipse
//   - theta1: starting angle (radians)
//   - deltaTheta: signed sweep (radians)
//
// Implements W3C SVG Implementation Notes F.6.5.
func endpointToCenter(x1, y1, rx, ry, phi float64, largeArcFlag, sweepFlag bool, x2, y2 float64) (cx, cy, theta1, deltaTheta float64) {
	if rx == 0 || ry == 0 || (x1 == x2 && y1 == y2) {
		return 0, 0, 0, 0
	}
	rx = math.Abs(rx)
	ry = math.Abs(ry)

	cosPhi := math.Cos(phi)
	sinPhi := math.Sin(phi)

	// Step 1: compute (x1', y1') — the start point in the
	// ellipse-aligned coordinate system, centered at the midpoint.
	dx := (x1 - x2) / 2
	dy := (y1 - y2) / 2
	x1p := cosPhi*dx + sinPhi*dy
	y1p := -sinPhi*dx + cosPhi*dy

	// Ensure radii are large enough. F.6.6.2.
	lambda := (x1p*x1p)/(rx*rx) + (y1p*y1p)/(ry*ry)
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s
		ry *= s
	}

	// Step 2: compute (cx', cy').
	signFactor := 1.0
	if largeArcFlag == sweepFlag {
		signFactor = -1.0
	}
	num := rx*rx*ry*ry - rx*rx*y1p*y1p - ry*ry*x1p*x1p
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	if den == 0 {
		return (x1 + x2) / 2, (y1 + y2) / 2, 0, 0
	}
	frac := num / den
	if frac < 0 {
		frac = 0
	}
	coef := signFactor * math.Sqrt(frac)
	cxp := coef * (rx * y1p) / ry
	cyp := coef * -(ry * x1p) / rx

	// Step 3: compute (cx, cy).
	cx = cosPhi*cxp - sinPhi*cyp + (x1+x2)/2
	cy = sinPhi*cxp + cosPhi*cyp + (y1+y2)/2

	// Step 4: compute theta1, deltaTheta.
	angleBetween := func(ux, uy, vx, vy float64) float64 {
		dot := ux*vx + uy*vy
		nu := math.Sqrt(ux*ux + uy*uy)
		nv := math.Sqrt(vx*vx + vy*vy)
		if nu == 0 || nv == 0 {
			return 0
		}
		c := dot / (nu * nv)
		if c > 1 {
			c = 1
		}
		if c < -1 {
			c = -1
		}
		ang := math.Acos(c)
		if ux*vy-uy*vx < 0 {
			ang = -ang
		}
		return ang
	}

	theta1 = angleBetween(1, 0, (x1p-cxp)/rx, (y1p-cyp)/ry)
	deltaTheta = angleBetween(
		(x1p-cxp)/rx, (y1p-cyp)/ry,
		(-x1p-cxp)/rx, (-y1p-cyp)/ry,
	)

	if !sweepFlag && deltaTheta > 0 {
		deltaTheta -= 2 * math.Pi
	} else if sweepFlag && deltaTheta < 0 {
		deltaTheta += 2 * math.Pi
	}
	return cx, cy, theta1, deltaTheta
}
