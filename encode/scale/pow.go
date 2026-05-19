package scale

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// PowScale interpolates sign(v)*|v|^exp linearly into the pixel
// range. Default exp = 1 reduces to a linear scale. The signed-power
// shape handles negative inputs correctly (Vega-Lite parity).
type PowScale struct {
	Exp       float64
	DomainMin float64
	DomainMax float64
	RangeMin  float64
	RangeMax  float64
}

// Apply implements Scale.
func (s *PowScale) Apply(value any) (float64, error) {
	v, ok := ToFloat(value)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("PowScale.Apply: value %v (type %T) is not numeric.", value, value),
			map[string]any{"Field": "<pow>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	exp := s.Exp
	if exp == 0 {
		exp = 1
	}
	transformed := signedPow(v, exp)
	mn := signedPow(s.DomainMin, exp)
	mx := signedPow(s.DomainMax, exp)
	if mx == mn {
		return (s.RangeMin + s.RangeMax) / 2, nil
	}
	t := (transformed - mn) / (mx - mn)
	return s.RangeMin + t*(s.RangeMax-s.RangeMin), nil
}

// Domain implements Scale.
func (s *PowScale) Domain() []any { return []any{s.DomainMin, s.DomainMax} }

// Range implements Scale.
func (s *PowScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *PowScale) Type() scene.ScaleType { return scene.ScalePow }

// signedPow applies pow while preserving sign (so negative inputs
// don't NaN when the exponent is fractional).
func signedPow(v, exp float64) float64 {
	if v < 0 {
		return -math.Pow(-v, exp)
	}
	return math.Pow(v, exp)
}
