package encode

import (
	"fmt"
	"math"
	"strconv"

	"github.com/frankbardon/prism/encode/scene"
)

// NiceTicks returns the canonical "nice" tick values for the domain
// [min, max] with approximately count ticks. Ported from D3's
// d3-array#ticks (commit 4f24b06, MIT licence). The shape:
//
//  1. If min == max, return [min].
//  2. Compute step ≈ (max - min) / count.
//  3. Round step to the nearest "nice" value in {1, 2, 5} × 10ⁿ.
//  4. Snap min to step * floor(min/step), max to step * ceil(max/step).
//  5. Walk from snapped-min to snapped-max by step, collecting values.
//
// Returns ticks in ascending order. Negative ranges work (min < max
// with negative values). When min > max, the result is reversed.
func NiceTicks(min, max float64, count int) []float64 {
	if count < 1 {
		count = 1
	}
	if min == max {
		return []float64{min}
	}
	reverse := min > max
	if reverse {
		min, max = max, min
	}
	step := tickIncrement(min, max, count)
	if step == 0 || math.IsInf(step, 0) || math.IsNaN(step) {
		return nil
	}
	var out []float64
	if step > 0 {
		// Positive step: snap to integer multiples of step.
		r0 := math.Round(min / step)
		r1 := math.Round(max / step)
		if r0*step < min {
			r0++
		}
		if r1*step > max {
			r1--
		}
		n := int(r1-r0) + 1
		if n < 1 {
			return nil
		}
		out = make([]float64, n)
		for i := 0; i < n; i++ {
			out[i] = (r0 + float64(i)) * step
		}
	} else {
		// Sub-1 step encoded as -1/step.
		step = -step
		r0 := math.Round(min * step)
		r1 := math.Round(max * step)
		if r0/step < min {
			r0++
		}
		if r1/step > max {
			r1--
		}
		n := int(r1-r0) + 1
		if n < 1 {
			return nil
		}
		out = make([]float64, n)
		for i := 0; i < n; i++ {
			out[i] = (r0 + float64(i)) / step
		}
	}
	if reverse {
		reverseFloats(out)
	}
	return out
}

// tickIncrement returns the step size for NiceTicks. Mirrors D3's
// d3-array#tickIncrement. Positive return = step in domain units;
// negative return = step is 1/abs(returned) (sub-1 steps). 0 / Inf /
// NaN signal failure to the caller.
func tickIncrement(start, stop float64, count int) float64 {
	step := (stop - start) / math.Max(0, float64(count))
	if step <= 0 || math.IsInf(step, 0) || math.IsNaN(step) {
		return step
	}
	power := math.Floor(math.Log10(step))
	error := step / math.Pow(10, power)

	// Round error to nearest "nice" multiplier: 1, 2, 5, 10.
	var multiplier float64
	switch {
	case error >= 7.071067811865475: // sqrt(50)
		multiplier = 10
	case error >= 3.1622776601683795: // sqrt(10)
		multiplier = 5
	case error >= 1.4142135623730951: // sqrt(2)
		multiplier = 2
	default:
		multiplier = 1
	}

	if power >= 0 {
		return multiplier * math.Pow(10, power)
	}
	return -math.Pow(10, -power) / multiplier
}

// reverseFloats reverses a slice in place.
func reverseFloats(xs []float64) {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
}

// TicksWithLabels formats raw tick values into scene.Tick entries.
// Each tick's Pixel is computed by applying scale.Apply to the
// value; the Label is formatted via the supplied format spec (use
// "" for the default %g rendering). Errors from scale.Apply are
// surfaced as a wrapped error tagged with the failing value.
func TicksWithLabels(values []float64, scale Scale, format string) ([]scene.Tick, error) {
	out := make([]scene.Tick, 0, len(values))
	for _, v := range values {
		pix, err := scale.Apply(v)
		if err != nil {
			return nil, fmt.Errorf("TicksWithLabels: scale.Apply(%g): %w", v, err)
		}
		out = append(out, scene.Tick{
			Value: v,
			Pixel: pix,
			Label: formatTick(v, format),
		})
	}
	return out, nil
}

// formatTick renders a numeric tick value. P05's default is %g (the
// Go stdlib's compact float format); explicit format overrides via
// fmt verbs.
func formatTick(v float64, format string) string {
	if format == "" {
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return fmt.Sprintf(format, v)
}

// BandTicks places one tick per category at the band's center. Used
// by axis tick placement so labels sit under (or beside) the band
// middle rather than its left edge.
func BandTicks(scale *BandScale) []scene.Tick {
	out := make([]scene.Tick, 0, len(scale.Categories))
	for _, c := range scale.Categories {
		pix, err := scale.BandCenter(c)
		if err != nil {
			continue
		}
		out = append(out, scene.Tick{
			Value: c,
			Pixel: pix,
			Label: c,
		})
	}
	return out
}
