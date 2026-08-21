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

// step returns the SIGNED step per category (band + gap).
//
// Signed because a y-axis band scale is built with an inverted range
// (RangeMin = the plot's bottom, RangeMax = its top) so that low
// values land low on screen. Everything below has to survive that.
func (s *BandScale) step() float64 {
	if len(s.Categories) == 0 {
		return 0
	}
	return (s.RangeMax - s.RangeMin) / float64(len(s.Categories))
}

// BandWidth returns the pixel extent of one band (post-padding),
// always positive.
//
// It used to return the signed value, so a categorical Y axis handed
// the rect encoder a NEGATIVE height and every heatmap-lite cell was
// emitted as `height="-177.9"` — which an SVG renderer draws as
// nothing at all. The chart rendered its axes, its labels and its
// legend around an empty plot.
func (s *BandScale) BandWidth() float64 {
	w := s.step() * (1 - s.Padding)
	if w < 0 {
		return -w
	}
	return w
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
			start := s.RangeMin + float64(i)*step + pad
			// Return the band's LOW coordinate, which is what a rect's
			// x/y means. On an inverted range the band runs backwards
			// from start, so the low edge is the far one.
			if step < 0 {
				return start + step*(1-s.Padding), nil
			}
			return start, nil
		}
	}
	return 0, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("BandScale.Apply: category %q not in domain.", cat),
		map[string]any{"Field": "<band>", "Source": "<scale>", "Available": joinCats(s.Categories)},
	)
}

// BandCenter returns the centre of the band for category cat. Correct
// for both range directions because Apply returns the low edge and
// BandWidth is unsigned.
func (s *BandScale) BandCenter(cat string) (float64, error) {
	low, err := s.Apply(cat)
	if err != nil {
		return 0, err
	}
	return low + s.BandWidth()/2, nil
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
