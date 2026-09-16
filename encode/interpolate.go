package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// The three interpolation spaces a continuous color ramp can be
// traversed in, named by `scale.interpolate`. Vega-Lite's `hcl` is
// intentionally not offered — the schema rejects it.
const (
	// InterpolateRGB blends the 8-bit sRGB components directly. The
	// default, and the only space that leaves pre-E4-S2 output byte
	// identical.
	InterpolateRGB = "rgb"
	// InterpolateHSL blends hue (along the shorter arc), saturation
	// and lightness. Keeps saturation up across a hue sweep where
	// sRGB blending would pass through grey.
	InterpolateHSL = "hsl"
	// InterpolateLab blends in CIELAB (D65), which is roughly
	// perceptually uniform, so a ramp's steps read as evenly spaced.
	InterpolateLab = "lab"
)

// InterpolatedRampStops is the number of stops a continuous ramp is
// resampled to when it is traversed in a non-sRGB space. The
// resampling happens once, at encode time, so everything downstream —
// the SVG renderer, the Scene IR's gradient stops, the vendored JS
// renderer — only ever blends adjacent stops in sRGB and still lands
// on the requested space's curve. That is what keeps colorspace math
// out of the JS side entirely.
//
// 33 stops puts at most ~8 sRGB units between neighbours on the worst
// ramp in the catalogue, which is below the visible banding threshold
// for a 130-px legend bar.
const InterpolatedRampStops = 33

// NormalizeInterpolate maps a `scale.interpolate` value onto one of
// the three supported spaces. An empty or unrecognised value falls
// back to InterpolateRGB; the schema is what rejects `hcl` and
// friends outright, so this never has to report an error.
func NormalizeInterpolate(s string) string {
	switch s {
	case InterpolateHSL:
		return InterpolateHSL
	case InterpolateLab:
		return InterpolateLab
	}
	return InterpolateRGB
}

// InterpolateColor blends a into b at t ∈ [0, 1] in the named space.
// Alpha always blends linearly, whatever the space. A nil endpoint
// yields the other endpoint.
func InterpolateColor(a, b *scene.Color, t float64, space string) *scene.Color {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return &scene.Color{R: b.R, G: b.G, B: b.B, A: b.A}
	case b == nil:
		return &scene.Color{R: a.R, G: a.G, B: a.B, A: a.A}
	}
	if t <= 0 {
		return &scene.Color{R: a.R, G: a.G, B: a.B, A: a.A}
	}
	if t >= 1 {
		return &scene.Color{R: b.R, G: b.G, B: b.B, A: b.A}
	}
	alpha := lerp(float64(a.A), float64(b.A), t)
	var r, g, bl float64
	switch NormalizeInterpolate(space) {
	case InterpolateHSL:
		r, g, bl = interpolateHSL(a, b, t)
	case InterpolateLab:
		r, g, bl = interpolateLab(a, b, t)
	default:
		r = lerp(float64(a.R), float64(b.R), t)
		g = lerp(float64(a.G), float64(b.G), t)
		bl = lerp(float64(a.B), float64(b.B), t)
	}
	return &scene.Color{R: byteOf(r), G: byteOf(g), B: byteOf(bl), A: byteOf(alpha)}
}

// ResampleRamp re-expresses a ramp's control points as n evenly
// spaced stops blended in the named space. Returns the input
// unchanged for the sRGB space (nothing to precompute), for n < 2,
// and for a ramp of fewer than two usable stops.
//
// The result is a plain sRGB stop list: a consumer that blends
// neighbouring stops linearly reproduces the requested space's curve
// to within InterpolatedRampStops' sampling error.
func ResampleRamp(stops []*scene.Color, space string, n int) []*scene.Color {
	space = NormalizeInterpolate(space)
	if space == InterpolateRGB || n < 2 || len(stops) < 2 {
		return stops
	}
	out := make([]*scene.Color, n)
	segs := float64(len(stops) - 1)
	for i := range out {
		pos := float64(i) / float64(n-1) * segs
		idx := int(pos)
		if idx >= len(stops)-1 {
			last := stops[len(stops)-1]
			out[i] = &scene.Color{R: last.R, G: last.G, B: last.B, A: last.A}
			continue
		}
		out[i] = InterpolateColor(stops[idx], stops[idx+1], pos-float64(idx), space)
	}
	return out
}

// ---------------------------------------------------------------- //
// sRGB ↔ HSL
// ---------------------------------------------------------------- //

// interpolateHSL blends in HSL, taking the shorter arc around the hue
// circle. An achromatic endpoint has no meaningful hue, so it borrows
// the other endpoint's — blending grey into red stays on the red hue
// rather than sweeping the whole wheel.
func interpolateHSL(a, b *scene.Color, t float64) (float64, float64, float64) {
	h0, s0, l0 := toHSL(a)
	h1, s1, l1 := toHSL(b)
	switch {
	case s0 == 0 && s1 != 0:
		h0 = h1
	case s1 == 0 && s0 != 0:
		h1 = h0
	}
	dh := h1 - h0
	if dh > 180 {
		dh -= 360
	} else if dh < -180 {
		dh += 360
	}
	h := math.Mod(h0+t*dh, 360)
	if h < 0 {
		h += 360
	}
	return fromHSL(h, lerp(s0, s1, t), lerp(l0, l1, t))
}

// toHSL converts an 8-bit sRGB color to hue (degrees), saturation and
// lightness (both 0..1).
func toHSL(c *scene.Color) (float64, float64, float64) {
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	l := (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	var s float64
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	var h float64
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l
}

// fromHSL converts hue (degrees), saturation and lightness back to
// 8-bit-scaled sRGB components (still floats, 0..255).
func fromHSL(h, s, l float64) (float64, float64, float64) {
	if s <= 0 {
		v := l * 255
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hk := h / 360
	return hueToRGB(p, q, hk+1.0/3.0) * 255,
		hueToRGB(p, q, hk) * 255,
		hueToRGB(p, q, hk-1.0/3.0) * 255
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// ---------------------------------------------------------------- //
// sRGB ↔ linear ↔ CIEXYZ ↔ CIELAB (D65)
// ---------------------------------------------------------------- //
//
// This mirrors the structure of static/vendor/prism/oklab.mjs (the
// animator's tween helper) but not its matrices: that module
// implements OKLab, a different space from the CIELAB that
// `scale.interpolate: "lab"` names. See the FOLLOWUPS note — OKLab is
// a small addition on top of this scaffolding if it is ever wanted.

// D65 reference white, 2° observer.
const (
	labWhiteX = 0.95047
	labWhiteY = 1.0
	labWhiteZ = 1.08883
	labDelta  = 6.0 / 29.0
)

func interpolateLab(a, b *scene.Color, t float64) (float64, float64, float64) {
	l0, a0, b0 := toLab(a)
	l1, a1, b1 := toLab(b)
	return fromLab(lerp(l0, l1, t), lerp(a0, a1, t), lerp(b0, b1, t))
}

func toLab(c *scene.Color) (float64, float64, float64) {
	r := srgbToLinear(float64(c.R) / 255)
	g := srgbToLinear(float64(c.G) / 255)
	b := srgbToLinear(float64(c.B) / 255)
	x := (0.4124564*r + 0.3575761*g + 0.1804375*b) / labWhiteX
	y := (0.2126729*r + 0.7151522*g + 0.0721750*b) / labWhiteY
	z := (0.0193339*r + 0.1191920*g + 0.9503041*b) / labWhiteZ
	fx, fy, fz := labF(x), labF(y), labF(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)
}

func fromLab(l, a, b float64) (float64, float64, float64) {
	fy := (l + 16) / 116
	fx := fy + a/500
	fz := fy - b/200
	x := labFInv(fx) * labWhiteX
	y := labFInv(fy) * labWhiteY
	z := labFInv(fz) * labWhiteZ
	lr := 3.2404542*x - 1.5371385*y - 0.4985314*z
	lg := -0.9692660*x + 1.8760108*y + 0.0415560*z
	lb := 0.0556434*x - 0.2040259*y + 1.0572252*z
	return linearToSrgb(lr) * 255, linearToSrgb(lg) * 255, linearToSrgb(lb) * 255
}

func labF(t float64) float64 {
	if t > labDelta*labDelta*labDelta {
		return math.Cbrt(t)
	}
	return t/(3*labDelta*labDelta) + 4.0/29.0
}

func labFInv(t float64) float64 {
	if t > labDelta {
		return t * t * t
	}
	return 3 * labDelta * labDelta * (t - 4.0/29.0)
}

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linearToSrgb(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1.0/2.4) - 0.055
}

// ---------------------------------------------------------------- //

func lerp(a, b, t float64) float64 { return a + t*(b-a) }

// byteOf rounds and clamps a 0..255-scaled component. Out-of-gamut
// Lab blends land outside the cube; clamping is what keeps them
// renderable.
func byteOf(v float64) uint8 {
	n := math.Round(v)
	if n <= 0 {
		return 0
	}
	if n >= 255 {
		return 255
	}
	return uint8(n)
}
