package scale

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// PointScale places each category at the center of an evenly-divided
// step. Unlike BandScale it has no bandwidth — Apply returns the point
// itself, not a band's left edge. Used for line/point fixtures where
// the x-axis is categorical but the marks need a single coordinate.
type PointScale struct {
	Categories []string
	RangeMin   float64
	RangeMax   float64
	Padding    float64 // [0,1) outer padding (fraction of step)
}

func (s *PointScale) step() float64 {
	n := float64(len(s.Categories))
	if n == 0 {
		return 0
	}
	// Outer padding eats from both ends.
	return (s.RangeMax - s.RangeMin) / (n - 1 + 2*s.Padding)
}

// Apply implements Scale.
func (s *PointScale) Apply(value any) (float64, error) {
	cat, ok := value.(string)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("PointScale.Apply: value %v (type %T) is not a string category.", value, value),
			map[string]any{"Field": "<point>", "Source": "<scale>", "Available": "string"},
		)
	}
	for i, c := range s.Categories {
		if c == cat {
			step := s.step()
			return s.RangeMin + step*(s.Padding+float64(i)), nil
		}
	}
	return 0, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("PointScale.Apply: category %q not in domain.", cat),
		map[string]any{"Field": "<point>", "Source": "<scale>", "Available": joinCats(s.Categories)},
	)
}

// Domain implements Scale.
func (s *PointScale) Domain() []any {
	out := make([]any, len(s.Categories))
	for i, c := range s.Categories {
		out[i] = c
	}
	return out
}

// Range implements Scale.
func (s *PointScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *PointScale) Type() scene.ScaleType { return scene.ScalePoint }
