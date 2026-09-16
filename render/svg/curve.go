package svg

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// Curve path emission.
//
// scene.CurveLinear is by design NOT promoted to a <path> here: the
// line renderer keeps emitting <polyline> for it, which is what every
// committed linear golden and cross-impl fixture pins. Only non-linear
// curves become <path d="...">. The area renderer already emits a
// <path> for every curve, and writeCurveBody reproduces the historic
// " Lx,y" run byte-for-byte when the curve is linear.
//
// Every coordinate — control points included — goes through
// render.FormatFloat so host Go and TinyGo-via-WASM agree to the
// pinned 3 decimals.

// isCurved reports whether c needs <path> geometry rather than the
// straight-segment fast paths.
func isCurved(c scene.CurveType) bool {
	return c != "" && c != scene.CurveLinear
}

// writeSegTo emits cmd followed by "<x>,<y>" (e.g. " L12,4").
func writeSegTo(w *Writer, cmd string, x, y float64) {
	w.Raw(cmd)
	w.Raw(render.FormatFloat(x))
	w.Raw(",")
	w.Raw(render.FormatFloat(y))
}

// writeCurveBody emits every path command AFTER the initial move onto
// pts[0] — the caller has already placed the pen there (via "M", or
// via the connector "L" the area renderer uses to step from the upper
// edge down onto the lower edge). Fewer than two points emit nothing.
func writeCurveBody(w *Writer, pts [][2]float64, curve scene.CurveType, tension float64) {
	if len(pts) < 2 {
		return
	}
	switch curve {
	case scene.CurveStep:
		writeStepBody(w, pts, 0.5)
	case scene.CurveStepBefore:
		writeStepBody(w, pts, 0)
	case scene.CurveStepAfter:
		writeStepBody(w, pts, 1)
	case scene.CurveMonotone:
		writeMonotoneBody(w, pts)
	case scene.CurveCardinal:
		writeCardinalBody(w, pts, tension)
	default:
		for _, p := range pts[1:] {
			writeSegTo(w, " L", p[0], p[1])
		}
	}
}

// writeStepBody emits d3-shape's curveStep family. t is the fraction
// of each x interval at which the riser sits: 0 = step-before (riser
// at the previous x), 1 = step-after (riser at the next x), 0.5 =
// step (midway). Mirrors d3's Step.point / Step.lineEnd exactly.
func writeStepBody(w *Writer, pts [][2]float64, t float64) {
	for i := 1; i < len(pts); i++ {
		x0, y0 := pts[i-1][0], pts[i-1][1]
		x1, y1 := pts[i][0], pts[i][1]
		if t <= 0 {
			writeSegTo(w, " L", x0, y1)
			writeSegTo(w, " L", x1, y1)
			continue
		}
		xm := x0*(1-t) + x1*t
		writeSegTo(w, " L", xm, y0)
		writeSegTo(w, " L", xm, y1)
	}
	// d3 closes the midpoint variant with a final segment onto the
	// true last point; the t<=0 and t>=1 variants already land there.
	if t > 0 && t < 1 {
		last := pts[len(pts)-1]
		writeSegTo(w, " L", last[0], last[1])
	}
}

// curveSign mirrors d3's `x < 0 ? -1 : 1` (so +1 for both 0 and NaN).
func curveSign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// monotoneSlope3 is d3-shape's slope3 — the finite-difference tangent
// at the middle of three points, limited so the cubic stays monotone.
// The zero-denominator dance reproduces d3's `h0 || h1 < 0 && -0`
// idiom, which divides by a signed zero on purpose to reach a signed
// infinity rather than a NaN.
func monotoneSlope3(x0, y0, x1, y1, x2, y2 float64) float64 {
	h0 := x1 - x0
	h1 := x2 - x1
	d0, d1 := h0, h1
	if d0 == 0 && h1 < 0 {
		d0 = math.Copysign(0, -1)
	}
	if d1 == 0 && h0 < 0 {
		d1 = math.Copysign(0, -1)
	}
	s0 := (y1 - y0) / d0
	s1 := (y2 - y1) / d1
	p := (s0*h1 + s1*h0) / (h0 + h1)
	v := (curveSign(s0) + curveSign(s1)) *
		math.Min(math.Min(math.Abs(s0), math.Abs(s1)), 0.5*math.Abs(p))
	if math.IsNaN(v) {
		return 0
	}
	return v
}

// monotoneSlope2 is d3-shape's slope2 — the one-sided endpoint tangent
// derived from the adjacent interior tangent.
func monotoneSlope2(x0, y0, x1, y1, t float64) float64 {
	h := x1 - x0
	if h != 0 && !math.IsNaN(h) {
		return (3*(y1-y0)/h - t) / 2
	}
	return t
}

// writeMonotoneBody emits d3-shape's curveMonotoneX: a Fritsch-Carlson
// monotone cubic Hermite spline expressed as cubic Beziers, so the
// interpolant never overshoots the data.
func writeMonotoneBody(w *Writer, pts [][2]float64) {
	n := len(pts)
	if n == 2 {
		writeSegTo(w, " L", pts[1][0], pts[1][1])
		return
	}
	// Tangent per point: interior from slope3, endpoints from slope2.
	tan := make([]float64, n)
	for i := 1; i < n-1; i++ {
		tan[i] = monotoneSlope3(
			pts[i-1][0], pts[i-1][1],
			pts[i][0], pts[i][1],
			pts[i+1][0], pts[i+1][1],
		)
	}
	tan[0] = monotoneSlope2(pts[0][0], pts[0][1], pts[1][0], pts[1][1], tan[1])
	tan[n-1] = monotoneSlope2(pts[n-2][0], pts[n-2][1], pts[n-1][0], pts[n-1][1], tan[n-2])
	for i := 0; i < n-1; i++ {
		x0, y0 := pts[i][0], pts[i][1]
		x1, y1 := pts[i+1][0], pts[i+1][1]
		dx := (x1 - x0) / 3
		writeSegTo(w, " C", x0+dx, y0+dx*tan[i])
		writeSegTo(w, " ", x1-dx, y1-dx*tan[i+1])
		writeSegTo(w, " ", x1, y1)
	}
}

// writeCardinalBody emits d3-shape's curveCardinal.tension(t). The
// control-point scale is k = (1-t)/6; t=0 (the d3 and Vega-Lite
// default) is the loosest spline and t=1 collapses to straight
// segments. The first segment's leading control point and the last
// segment's trailing control point degenerate onto their endpoints,
// matching d3's NaN-primed streaming state.
func writeCardinalBody(w *Writer, pts [][2]float64, tension float64) {
	n := len(pts)
	if n == 2 {
		writeSegTo(w, " L", pts[1][0], pts[1][1])
		return
	}
	k := (1 - tension) / 6
	for i := 0; i < n-1; i++ {
		x0, y0 := pts[i][0], pts[i][1]
		x1, y1 := pts[i+1][0], pts[i+1][1]
		c1x, c1y := x0, y0
		if i > 0 {
			c1x = x0 + k*(x1-pts[i-1][0])
			c1y = y0 + k*(y1-pts[i-1][1])
		}
		c2x, c2y := x1, y1
		if i < n-2 {
			c2x = x1 + k*(x0-pts[i+2][0])
			c2y = y1 + k*(y0-pts[i+2][1])
		}
		writeSegTo(w, " C", c1x, c1y)
		writeSegTo(w, " ", c2x, c2y)
		writeSegTo(w, " ", x1, y1)
	}
}
