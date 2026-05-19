package scale

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// BandScale is the categorical scale used by bar / rect marks. Each
// category gets a band of equal width; padding leaves an inner gap.
type BandScale struct {
	Categories []string
	RangeMin   float64
	RangeMax   float64
	Padding    float64 // [0,1) inner padding (fraction of step)
}

// step returns the full step width per category (band + gap).
func (s *BandScale) step() float64 {
	if len(s.Categories) == 0 {
		return 0
	}
	return (s.RangeMax - s.RangeMin) / float64(len(s.Categories))
}

// BandWidth returns the pixel width of one band (post-padding).
func (s *BandScale) BandWidth() float64 {
	step := s.step()
	return step * (1 - s.Padding)
}

// Apply implements Scale. Returns the left edge of the band for the
// given category.
func (s *BandScale) Apply(value any) (float64, error) {
	cat, ok := value.(string)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("BandScale.Apply: value %v (type %T) is not a string category.", value, value),
			map[string]any{"Field": "<band>", "Source": "<scale>", "Available": "string"},
		)
	}
	for i, c := range s.Categories {
		if c == cat {
			step := s.step()
			pad := step * s.Padding / 2
			return s.RangeMin + float64(i)*step + pad, nil
		}
	}
	return 0, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("BandScale.Apply: category %q not in domain.", cat),
		map[string]any{"Field": "<band>", "Source": "<scale>", "Available": joinCats(s.Categories)},
	)
}

// BandCenter returns the center x of the band for category cat.
func (s *BandScale) BandCenter(cat string) (float64, error) {
	left, err := s.Apply(cat)
	if err != nil {
		return 0, err
	}
	return left + s.BandWidth()/2, nil
}

// Domain implements Scale.
func (s *BandScale) Domain() []any {
	out := make([]any, len(s.Categories))
	for i, c := range s.Categories {
		out[i] = c
	}
	return out
}

// Range implements Scale.
func (s *BandScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *BandScale) Type() scene.ScaleType { return scene.ScaleBand }

func joinCats(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	out := xs[0]
	for _, s := range xs[1:] {
		out += ", " + s
	}
	return out
}
