// Package format implements a small subset of the d3-format mini-DSL
// plus a strftime-style mini-mapping for time formats. Pure stdlib.
// See README.md for the supported specifier set.
//
// Number specifier grammar (subset):
//
//	[sign]?[,]?[.<int>]?[~]?[f|%|d|e|s|g]
//
//	sign  "-" (default: minus only), "+" (plus for zero and positives),
//	      " " (a space for zero and positives)
//	,     thousands separator
//	.N    precision (digits after decimal; significant digits for g)
//	~     trim insignificant trailing zeros ("1.50" → "1.5", "2.00" → "2")
//	f     fixed-point ("1234.50")
//	%     percent (multiply by 100, append "%")
//	d     integer
//	e     scientific ("1.234e+03")
//	s     SI prefix ("1k", "1.2M")
//	g     significant digits, d3's toPrecision rule ("1235", "0.0001235",
//	      "1.235e+7"); without a precision, the shortest round-trip form
//
// Time specifier grammar (subset of strftime):
//
//	%Y  4-digit year
//	%m  2-digit month
//	%d  2-digit day-of-month
//	%H  2-digit hour (24h)
//	%M  2-digit minute
//	%S  2-digit second
//	%L  3-digit millisecond
//	%j  3-digit day-of-year
//
// The discriminator between number/time format is the presence of a
// `%X` directive that is not a bare "%" or "N%" — i.e. anything
// containing `%Y`, `%m`, etc. is a time format.
package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	prismerrors "github.com/frankbardon/prism/errors"
)

// Spec is a parsed format specifier. Either Number or Time is set,
// never both.
type Spec struct {
	Raw    string
	Number *NumberSpec
	Time   *TimeSpec
}

// NumberSpec captures the parsed number format.
type NumberSpec struct {
	Thousands bool
	Precision int  // -1 = unset
	HasPrec   bool // distinguishes ".0f" from "f"
	Type      byte // 'f' | '%' | 'd' | 'e' | 's' | 'g'
	// Sign is the d3 sign flag: '-' (the default, a minus for
	// negatives only), '+' or ' ' (a plus / space for zero and
	// positives too).
	Sign byte
	// Trim is d3's "~": drop insignificant trailing zeros (and a
	// dangling decimal point) from the digits.
	Trim bool
}

// TimeSpec captures the parsed time format.
type TimeSpec struct {
	Layout string // converted to Go's reference-time layout
}

// Parse parses a format specifier. Returns PRISM_SPEC_011 on a
// malformed spec. An empty spec parses as a no-op number spec
// (callers fall back to default %g rendering).
func Parse(s string) (*Spec, error) {
	if s == "" {
		return &Spec{Raw: s, Number: &NumberSpec{Precision: -1, Type: 'g'}}, nil
	}
	// Detect time vs number: a time spec contains a `%X` where X is
	// one of YmdHMSLj. A bare `%` or `N%` is the number-percent type.
	if isTimeSpec(s) {
		layout, err := timeLayout(s)
		if err != nil {
			return nil, prismerrors.New("PRISM_SPEC_011",
				fmt.Sprintf("Format string %q is not a recognised d3-format specifier.", s),
				map[string]any{"Spec": s, "Reason": err.Error(), "Where": "<format>"},
			)
		}
		return &Spec{Raw: s, Time: &TimeSpec{Layout: layout}}, nil
	}
	num, err := parseNumber(s)
	if err != nil {
		return nil, prismerrors.New("PRISM_SPEC_011",
			fmt.Sprintf("Format string %q is not a recognised d3-format specifier.", s),
			map[string]any{"Spec": s, "Reason": err.Error(), "Where": "<format>"},
		)
	}
	return &Spec{Raw: s, Number: num}, nil
}

// MustParse is Parse + panic; for static fixture builders + tests.
func MustParse(s string) *Spec {
	sp, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return sp
}

// Apply renders v with the parsed spec. v may be a numeric type, a
// time.Time, or a string (passed through).
func (sp *Spec) Apply(v any) string {
	if sp == nil {
		return fmt.Sprintf("%v", v)
	}
	if sp.Time != nil {
		switch t := v.(type) {
		case time.Time:
			return t.UTC().Format(sp.Time.Layout)
		case float64:
			return time.UnixMilli(int64(t)).UTC().Format(sp.Time.Layout)
		case int64:
			return time.UnixMilli(t).UTC().Format(sp.Time.Layout)
		}
		return fmt.Sprintf("%v", v)
	}
	if sp.Number != nil {
		f, ok := toFloat(v)
		if !ok {
			return fmt.Sprintf("%v", v)
		}
		return sp.Number.format(f)
	}
	return fmt.Sprintf("%v", v)
}

// format renders f with the number spec: the digits, then d3's "~"
// trim, then the sign flag.
func (n *NumberSpec) format(f float64) string {
	out := n.digits(f)
	if n.Trim {
		out = trimZeros(out)
	}
	if n.Sign == '+' || n.Sign == ' ' {
		// A value that rounds to zero prints unsigned, as d3 does.
		if !strings.HasPrefix(out, "-") {
			out = string(n.Sign) + out
		}
	}
	return out
}

// digits renders f with the number spec's type, precision and
// thousands flag.
func (n *NumberSpec) digits(f float64) string {
	switch n.Type {
	case 'f':
		prec := n.Precision
		if !n.HasPrec {
			prec = 6
		}
		out := strconv.FormatFloat(f, 'f', prec, 64)
		if n.Thousands {
			out = insertThousands(out)
		}
		return out
	case '%':
		prec := n.Precision
		if !n.HasPrec {
			prec = 1
		}
		out := strconv.FormatFloat(f*100, 'f', prec, 64) + "%"
		if n.Thousands {
			out = insertThousands(out)
		}
		return out
	case 'd':
		out := strconv.FormatInt(int64(math.Round(f)), 10)
		if n.Thousands {
			out = insertThousands(out)
		}
		return out
	case 'e':
		prec := n.Precision
		if !n.HasPrec {
			prec = 6
		}
		return strconv.FormatFloat(f, 'e', prec, 64)
	case 's':
		return siPrefix(f, n.Precision, n.HasPrec)
	case 'g':
		if !n.HasPrec {
			break
		}
		out := toPrecision(f, n.Precision)
		if n.Thousands {
			out = insertThousands(out)
		}
		return out
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// toPrecision is JavaScript's Number.prototype.toPrecision, the rule
// d3's "g" type uses: p significant digits, fixed-point unless the
// decimal exponent is below -6 or at least p, then exponential with
// an unpadded exponent ("1.2e+7", never "1.2e+07").
func toPrecision(f float64, p int) string {
	if p < 1 {
		p = 1
	}
	if f == 0 {
		return strconv.FormatFloat(0, 'f', p-1, 64)
	}
	// Round to p significant digits first, so the exponent is that of
	// the rounded value (9.99 at 2 digits is 10, exponent 1).
	e := strconv.FormatFloat(f, 'e', p-1, 64)
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	if exp < -6 || exp >= p {
		sign := "+"
		if exp < 0 {
			sign = "-"
			exp = -exp
		}
		return mant + "e" + sign + strconv.Itoa(exp)
	}
	return strconv.FormatFloat(f, 'f', p-1-exp, 64)
}

// trimZeros drops insignificant trailing zeros from the fractional
// digits of a rendered number, and the decimal point when nothing is
// left after it. Any suffix after the digits (an exponent, an SI
// prefix, a percent sign) is kept: "1.500e+3" → "1.5e+3", "2.00k" →
// "2k", "12.50%" → "12.5%".
func trimZeros(s string) string {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return s
	}
	end := dot + 1
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	frac := strings.TrimRight(s[dot+1:end], "0")
	if frac == "" {
		return s[:dot] + s[end:]
	}
	return s[:dot+1] + frac + s[end:]
}

// parseNumber walks the spec body and builds a NumberSpec.
func parseNumber(s string) (*NumberSpec, error) {
	out := &NumberSpec{Precision: -1, Sign: '-'}
	i := 0
	if i < len(s) && (s[i] == '-' || s[i] == '+' || s[i] == ' ') {
		out.Sign = s[i]
		i++
	}
	if i < len(s) && s[i] == ',' {
		out.Thousands = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i {
			return nil, fmt.Errorf("missing precision digits after '.'")
		}
		p, err := strconv.Atoi(s[i:j])
		if err != nil {
			return nil, fmt.Errorf("bad precision: %w", err)
		}
		out.Precision = p
		out.HasPrec = true
		i = j
	}
	if i < len(s) && s[i] == '~' {
		out.Trim = true
		i++
	}
	if i >= len(s) {
		// Bare "," or no type — default to 'f'.
		out.Type = 'f'
		return out, nil
	}
	switch c := s[i]; c {
	case 'f', '%', 'd', 'e', 's', 'g':
		out.Type = c
	default:
		return nil, fmt.Errorf("unknown type specifier %q", string(c))
	}
	if i+1 != len(s) {
		return nil, fmt.Errorf("trailing characters after type %q", s[i+1:])
	}
	return out, nil
}

// insertThousands inserts comma separators into the integer portion
// of a numeric string. Negatives and decimal portions are preserved.
func insertThousands(s string) string {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}
	dot := strings.IndexByte(s, '.')
	pct := strings.IndexByte(s, '%')
	intPart := s
	rest := ""
	switch {
	case dot >= 0:
		intPart = s[:dot]
		rest = s[dot:]
	case pct >= 0:
		intPart = s[:pct]
		rest = s[pct:]
	}
	if len(intPart) <= 3 {
		return sign + intPart + rest
	}
	var b strings.Builder
	rem := len(intPart) % 3
	if rem > 0 {
		b.WriteString(intPart[:rem])
		if len(intPart) > rem {
			b.WriteByte(',')
		}
	}
	for j := rem; j < len(intPart); j += 3 {
		b.WriteString(intPart[j : j+3])
		if j+3 < len(intPart) {
			b.WriteByte(',')
		}
	}
	return sign + b.String() + rest
}

// siPrefix renders f using SI prefix notation (k/M/G/T/...).
func siPrefix(f float64, prec int, hasPrec bool) string {
	if f == 0 {
		if hasPrec {
			return strconv.FormatFloat(0, 'f', prec, 64)
		}
		return "0"
	}
	abs := math.Abs(f)
	exp := int(math.Floor(math.Log10(abs)) / 3)
	if exp < 0 {
		exp = 0
	}
	prefixes := []string{"", "k", "M", "G", "T", "P", "E"}
	if exp >= len(prefixes) {
		exp = len(prefixes) - 1
	}
	scaled := f / math.Pow(10, float64(exp*3))
	p := prec
	if !hasPrec {
		p = 0
	}
	return strconv.FormatFloat(scaled, 'f', p, 64) + prefixes[exp]
}

// isTimeSpec reports whether s contains a strftime directive.
func isTimeSpec(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] != '%' {
			continue
		}
		switch s[i+1] {
		case 'Y', 'm', 'd', 'H', 'M', 'S', 'L', 'j':
			return true
		}
	}
	return false
}

// timeLayout converts a strftime-subset spec to Go's reference-time
// layout. Anything not in the supported set passes through verbatim.
func timeLayout(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case 'Y':
			b.WriteString("2006")
		case 'm':
			b.WriteString("01")
		case 'd':
			b.WriteString("02")
		case 'H':
			b.WriteString("15")
		case 'M':
			b.WriteString("04")
		case 'S':
			b.WriteString("05")
		case 'L':
			b.WriteString("000") // milliseconds (Go uses 000 inside fractional)
		case 'j':
			b.WriteString("002")
		case '%':
			b.WriteByte('%')
		default:
			return "", fmt.Errorf("unsupported time directive %%%c", s[i+1])
		}
		i++
	}
	return b.String(), nil
}

// toFloat is a tiny coercion helper. Lives here to avoid a circular
// import with encode/scale.
func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, !math.IsNaN(x) && !math.IsInf(x, 0)
	case float32:
		f := float64(x)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	}
	return 0, false
}
