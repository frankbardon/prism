package scale

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// LogScale interpolates log(value)/log(base) linearly into the pixel
// range. Domain must be strictly positive — non-positive values raise
// PRISM_SPEC_010 at resolve time.
type LogScale struct {
	Base      float64
	DomainMin float64
	DomainMax float64
	RangeMin  float64
	RangeMax  float64
}

// Apply implements Scale.
func (s *LogScale) Apply(value any) (float64, error) {
	v, ok := ToFloat(value)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("LogScale.Apply: value %v (type %T) is not numeric.", value, value),
			map[string]any{"Field": "<log>", "Source": "<scale>", "Available": "positive numeric"},
		)
	}
	if v <= 0 {
		return 0, prismerrors.New(
			"PRISM_SPEC_010",
			fmt.Sprintf("LogScale.Apply: value %v is not positive.", v),
			map[string]any{"Value": v, "ScaleType": "log"},
		)
	}
	base := s.Base
	if base == 0 {
		base = 10
	}
	logBase := math.Log(base)
	num := math.Log(v) / logBase
	mn := math.Log(s.DomainMin) / logBase
	mx := math.Log(s.DomainMax) / logBase
	if mx == mn {
		return (s.RangeMin + s.RangeMax) / 2, nil
	}
	t := (num - mn) / (mx - mn)
	return s.RangeMin + t*(s.RangeMax-s.RangeMin), nil
}

// Domain implements Scale.
func (s *LogScale) Domain() []any { return []any{s.DomainMin, s.DomainMax} }

// Range implements Scale.
func (s *LogScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *LogScale) Type() scene.ScaleType { return scene.ScaleLog }
