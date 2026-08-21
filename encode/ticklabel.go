package encode

import (
	"math"
	"strconv"
	"strings"
)

// AutoTickFormat picks the number format for an axis whose spec gave
// none.
//
// The old default was strconv 'g', which renders 1500000 as "1.5e+06"
// and 0.30000000000000004 in full. Neither belongs on a chart anyone
// reads. The replacement decides from the tick STEP and the domain
// MAGNITUDE, which together are exactly what a reader needs the label
// to communicate:
//
//   - step fixes the precision. Ticks 0.25 apart need two decimals;
//     ticks 10 apart need none, and printing "40.0" for them is noise.
//   - magnitude fixes the notation. Past 10,000 the digits stop
//     carrying meaning in a chat-column chart and the label becomes a
//     shape to compare, so it compacts to "1.5M". Below that the exact
//     figure fits and is more useful, with thousands separators so
//     "24000" reads as 24 thousand at a glance.
//
// Values are the resolved major tick values; an empty slice yields the
// identity format.
func AutoTickFormat(values []float64) TickFormatter {
	finite := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			finite = append(finite, v)
		}
	}
	if len(finite) == 0 {
		return TickFormatter{decimals: 0}
	}

	maxAbs := 0.0
	for _, v := range finite {
		if a := math.Abs(v); a > maxAbs {
			maxAbs = a
		}
	}

	step := tickStepOf(finite)
	decimals := decimalsForStep(step)

	// Past the compact threshold, precision is re-derived against the
	// SCALED value: ticks 250_000 apart compact to 0.25M-steps, which
	// still needs two decimals to stay distinct.
	if maxAbs >= compactThreshold {
		div, suffix := siUnit(maxAbs)
		return TickFormatter{
			decimals: decimalsForStep(step / div),
			divisor:  div,
			suffix:   suffix,
			group:    false,
		}
	}
	return TickFormatter{decimals: decimals, divisor: 1, group: true}
}

// compactThreshold is where labels switch from grouped digits
// ("24,000") to SI-compact ("24k").
//
// 10,000 rather than 1,000: four grouped digits still read instantly
// and carry their exact value, and a y-axis of "2k / 4k / 6k" throws
// away precision a reader of a revenue chart wants. Five digits is
// where the label starts to dominate the left margin instead.
const compactThreshold = 10000

// TickFormatter renders one numeric tick value. Zero value formats
// integers with no grouping, which is the correct degenerate case for
// an axis with a single tick.
type TickFormatter struct {
	decimals int
	divisor  float64
	suffix   string
	group    bool
}

// Format renders v.
func (f TickFormatter) Format(v float64) string {
	if math.IsNaN(v) {
		return ""
	}
	if math.IsInf(v, 1) {
		return "∞"
	}
	if math.IsInf(v, -1) {
		return "-∞"
	}
	div := f.divisor
	if div == 0 {
		div = 1
	}
	scaled := v / div
	// Zero is spelt "0": never "-0" (which is what FormatFloat gives
	// for a negative zero), and never "0M" — an axis whose origin
	// reads "zero million" is a strange thing to print on a chart.
	if scaled == 0 {
		return "0"
	}
	s := strconv.FormatFloat(scaled, 'f', f.decimals, 64)
	if f.decimals > 0 {
		s = trimTrailingZeros(s)
	}
	if f.group {
		s = groupThousands(s)
	}
	return s + f.suffix
}

// trimTrailingZeros drops a fractional tail that carries no
// information ("3.50" → "3.5", "4.00" → "4"). Applied per label
// rather than per axis on purpose: an axis stepping by 0.5 shows
// "0 / 0.5 / 1 / 1.5", which is how the values would be written down.
func trimTrailingZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// groupThousands inserts a comma every three digits left of the
// decimal point.
func groupThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	if len(intPart) > 3 {
		var b strings.Builder
		lead := len(intPart) % 3
		if lead > 0 {
			b.WriteString(intPart[:lead])
		}
		for i := lead; i < len(intPart); i += 3 {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(intPart[i : i+3])
		}
		intPart = b.String()
	}
	out := intPart
	if hasFrac {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

// siUnit returns the divisor and suffix for a magnitude. Stops at T:
// past that the exponent is the honest rendering and the chart has
// bigger problems than its axis labels.
func siUnit(maxAbs float64) (float64, string) {
	switch {
	case maxAbs >= 1e12:
		return 1e12, "T"
	case maxAbs >= 1e9:
		return 1e9, "B"
	case maxAbs >= 1e6:
		return 1e6, "M"
	default:
		return 1e3, "k"
	}
}

// tickStepOf returns the smallest gap between consecutive tick
// values, which is the resolution the labels must distinguish.
func tickStepOf(values []float64) float64 {
	if len(values) < 2 {
		if len(values) == 1 {
			return math.Abs(values[0])
		}
		return 1
	}
	step := math.Inf(1)
	for i := 1; i < len(values); i++ {
		if d := math.Abs(values[i] - values[i-1]); d > 0 && d < step {
			step = d
		}
	}
	if math.IsInf(step, 1) || step == 0 {
		return 1
	}
	return step
}

// decimalsForStep returns how many decimal places distinguish two
// values one step apart, capped at 6 so a float artefact in the step
// cannot produce a twelve-digit label.
func decimalsForStep(step float64) int {
	step = math.Abs(step)
	if step == 0 || math.IsNaN(step) || math.IsInf(step, 0) {
		return 0
	}
	if step >= 1 {
		return 0
	}
	d := int(math.Ceil(-math.Log10(step)))
	if d < 0 {
		d = 0
	}
	if d > 6 {
		d = 6
	}
	return d
}
