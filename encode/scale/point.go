package scale

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// PointScale places each category at the center of an evenly-divided
// step. Unlike BandScale it has no bandwidth — Apply returns the point
// itself, not a band's leading edge. Used for line/point fixtures where
// the x-axis is categorical but the marks need a single coordinate.
//
// A point scale is a band scale with an inner padding of exactly 1
// (every band collapses to a point), so the layout shares the same
// shape:
//
//	step    = span / max(1, n - 1 + 2*Padding)
//	offset  = (span - step*(n-1)) * Align
//	pos(i)  = RangeMin + offset + step*i
type PointScale struct {
	Categories []string
	RangeMin   float64
	RangeMax   float64
	// Padding is the outer padding — the gap before the first and
	// after the last point, as a fraction of the step. Defaults to
	// 0.5 at resolve time. A point scale has no inner padding, so
	// `scale.padding_inner` never reaches it.
	Padding float64
	// Align in [0,1] distributes the leftover slack; 0.5 (the
	// default) centres the points in the range.
	Align float64
	// Round quantises the step and the leading offset to whole
	// pixels. Layout quantisation, independent of render/precision.go.
	Round bool
	// Reverse hands the computed slots to the categories back to
	// front without flipping the direction the range itself runs.
	Reverse bool
}

// layout returns the signed step and the signed offset of the first
// point from RangeMin.
func (s *PointScale) layout() (step, offset float64) {
	n := float64(len(s.Categories))
	if n == 0 {
		return 0, 0
	}
	span := s.RangeMax - s.RangeMin
	sign := 1.0
	if span < 0 {
		sign = -1
	}
	mag := math.Abs(span)
	divisor := n - 1 + 2*s.Padding
	if divisor < 1 {
		divisor = 1
	}
	step = mag / divisor
	if s.Round {
		step = math.Floor(step)
	}
	offset = (mag - step*(n-1)) * s.Align
	if s.Round {
		offset = math.Round(offset)
	}
	return sign * step, sign * offset
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
			step, offset := s.layout()
			slot := i
			if s.Reverse {
				slot = len(s.Categories) - 1 - i
			}
			return s.RangeMin + offset + float64(slot)*step, nil
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
