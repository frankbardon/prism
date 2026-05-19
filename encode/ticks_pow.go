package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// PowTicks returns ticks for a PowScale by generating linear ticks in
// the transformed (post-pow) domain, then inverting them back to the
// original space. This gives evenly-spaced ticks in pixel space while
// keeping label values readable in the original units.
func PowTicks(s *PowScale, count int) []scene.Tick {
	exp := s.Exp
	if exp == 0 {
		exp = 1
	}
	tMin := signedPow(s.DomainMin, exp)
	tMax := signedPow(s.DomainMax, exp)
	rawTicks := NiceTicks(tMin, tMax, count)
	out := make([]scene.Tick, 0, len(rawTicks))
	for _, t := range rawTicks {
		// Invert: original_value = sign(t) * |t|^(1/exp)
		orig := signedPow(t, 1.0/exp)
		pix, err := s.Apply(orig)
		if err != nil {
			continue
		}
		out = append(out, scene.Tick{
			Value: orig,
			Pixel: pix,
			Label: formatTick(orig, ""),
		})
	}
	return out
}

// SqrtTicks delegates to PowTicks via the inner PowScale.
func SqrtTicks(s *SqrtScale, count int) []scene.Tick {
	inner := s.Inner
	inner.Exp = 0.5
	return PowTicks(&inner, count)
}

// signedPow applies pow while preserving sign — duplicated from
// encode/scale/pow.go so encode/ tick code is self-contained.
func signedPow(v, exp float64) float64 {
	if v < 0 {
		return -math.Pow(-v, exp)
	}
	return math.Pow(v, exp)
}
