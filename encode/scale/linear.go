package scale

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// LinearScale is the canonical quantitative scale: linear
// interpolation from [DomainMin, DomainMax] to [RangeMin, RangeMax].
// Range may be inverted (RangeMin > RangeMax) for y-axes where data
// 0 sits at the bottom of the SVG.
type LinearScale struct {
	DomainMin float64
	DomainMax float64
	RangeMin  float64
	RangeMax  float64
}

// Apply implements Scale.
func (s *LinearScale) Apply(value any) (float64, error) {
	v, ok := ToFloat(value)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("LinearScale.Apply: value %v (type %T) is not numeric.", value, value),
			map[string]any{"Field": "<linear>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	if s.DomainMax == s.DomainMin {
		return (s.RangeMin + s.RangeMax) / 2, nil
	}
	t := (v - s.DomainMin) / (s.DomainMax - s.DomainMin)
	return s.RangeMin + t*(s.RangeMax-s.RangeMin), nil
}

// Domain implements Scale.
func (s *LinearScale) Domain() []any { return []any{s.DomainMin, s.DomainMax} }

// Range implements Scale.
func (s *LinearScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *LinearScale) Type() scene.ScaleType { return scene.ScaleLinear }
