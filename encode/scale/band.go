package scale

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// BandScale is the categorical scale used by bar / rect marks. Each
// category owns one step of the range: PaddingInner opens a gap
// between adjacent bands, PaddingOuter opens a gap before the first
// and after the last band, and Align decides where the slack that is
// left over sits.
//
// The geometry mirrors d3-scale's band layout:
//
//	step      = span / max(1, n - PaddingInner + 2*PaddingOuter)
//	offset    = (span - step*(n - PaddingInner)) * Align
//	width     = step * (1 - PaddingInner)
//	left(i)   = RangeMin + offset + step*i
//
// The span is *signed*. A y band scale runs bottom-to-top, so
// RangeMin > RangeMax and step, offset and BandWidth all come back
// negative; mark encoders normalise that through rectAxisExtent (see
// encode/marks/orient.go). Do not "fix" the sign here — the inverted
// step is what horizontal bars and heatmap rows are built on.
type BandScale struct {
	Categories []string
	RangeMin   float64
	RangeMax   float64
	// PaddingInner is the gap between adjacent bands as a fraction of
	// the step, in [0,1). Defaults to 0.1 at resolve time.
	PaddingInner float64
	// PaddingOuter is the gap before the first and after the last
	// band, again as a fraction of the step. Defaults to 0.05 at
	// resolve time, which together with PaddingInner 0.1 and Align
	// 0.5 reproduces Prism's historic half-inner-gap-at-each-end
	// layout exactly.
	PaddingOuter float64
	// Align in [0,1] distributes the leftover slack: 0 pushes the
	// bands to the range start, 1 to the range end, 0.5 (the default)
	// centres them.
	Align float64
	// Round quantises the step, the leading offset and the band width
	// to whole pixels so band edges land on device pixels. It is a
	// *layout* quantisation, unrelated to the 3-decimal serialisation
	// pinning in render/precision.go — both can apply.
	Round bool
	// Reverse hands the computed slots to the categories back to
	// front, so the last category sits where the first otherwise
	// would. Widths and the step keep their sign: reversing changes
	// which slot a category lands in, never the direction the range
	// itself runs.
	Reverse bool
}

// layout returns the signed step, the signed offset of the first
// band's leading edge from RangeMin, and the signed band width.
func (s *BandScale) layout() (step, offset, width float64) {
	n := float64(len(s.Categories))
	if n == 0 {
		return 0, 0, 0
	}
	span := s.RangeMax - s.RangeMin
	sign := 1.0
	if span < 0 {
		sign = -1
	}
	mag := math.Abs(span)
	divisor := n - s.PaddingInner + 2*s.PaddingOuter
	if divisor < 1 {
		divisor = 1
	}
	step = mag / divisor
	if s.Round {
		step = math.Floor(step)
	}
	offset = (mag - step*(n-s.PaddingInner)) * s.Align
	width = step * (1 - s.PaddingInner)
	if s.Round {
		offset = math.Round(offset)
		width = math.Round(width)
	}
	return sign * step, sign * offset, sign * width
}

// BandWidth returns the pixel width of one band (post-padding). It is
// negative on an inverted (bottom-to-top) range; callers normalise.
func (s *BandScale) BandWidth() float64 {
	_, _, width := s.layout()
	return width
}

// Apply implements Scale. Returns the leading edge of the band for the
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
			step, offset, _ := s.layout()
			slot := i
			if s.Reverse {
				slot = len(s.Categories) - 1 - i
			}
			return s.RangeMin + offset + float64(slot)*step, nil
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
