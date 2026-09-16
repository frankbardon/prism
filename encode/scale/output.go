package scale

import "math"

// ContinuousOutput carries the two output policies every continuous
// scale shares: `scale.clamp` and `scale.round`. It is embedded rather
// than repeated so linear / log / pow / sqrt / time cannot drift.
type ContinuousOutput struct {
	// Clamp pins a value outside the resolved domain to the nearest
	// domain edge, so it lands on the range edge instead of past it.
	// Off by default — the default is to let the value overflow the
	// range and be clipped by the plot rect.
	Clamp bool
	// Round quantises the resolved pixel to a whole number. This is a
	// layout quantisation and composes with (rather than replaces)
	// the 3-decimal serialisation pinning in render/precision.go.
	Round bool
}

// interpolate maps a normalised position t into [rangeMin, rangeMax],
// applying the clamp and round policies. t is the domain-relative
// fraction the caller computed; clamping it to [0,1] is equivalent to
// pinning the input value to the domain edge, and keeps the policy in
// one place for every continuous family.
func (o ContinuousOutput) interpolate(t, rangeMin, rangeMax float64) float64 {
	if o.Clamp {
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
	}
	return o.quantise(rangeMin + t*(rangeMax-rangeMin))
}

// quantise applies the round policy to an already-resolved pixel.
func (o ContinuousOutput) quantise(pixel float64) float64 {
	if o.Round {
		return math.Round(pixel)
	}
	return pixel
}
