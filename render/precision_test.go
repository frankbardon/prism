package render

import (
	"math"
	"testing"
)

// TestPrismFormatFloatPrecision pins the float formatting contract
// every renderer depends on. Goldens drift if this changes; treat
// any change here as a wire-version bump.
func TestPrismFormatFloatPrecision(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{0.5, "0.5"},
		{1.234567, "1.235"},
		{1.2345, "1.234"}, // Go's banker's rounding at .5 boundary: 1.2345 -> 1.234
		{-1.999, "-1.999"},
		{1000, "1000"},
		{1000.5, "1000.5"},
		{math.Pi, "3.142"},
		{math.NaN(), "0"},
		{math.Inf(1), "0"},
		{math.Inf(-1), "0"},
		{0.0001, "0"}, // beneath precision threshold
		{0.0009, "0.001"},
	}
	for _, tc := range cases {
		got := FormatFloat(tc.in)
		if got != tc.want {
			t.Errorf("FormatFloat(%g) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestPrismRenderPrecisionConstant is a guard against accidental
// edits to the constant. Changing this implicitly bumps the SVG
// wire-format version (every committed golden would need regen).
func TestPrismRenderPrecisionConstant(t *testing.T) {
	if RenderPrecision != 3 {
		t.Fatalf("RenderPrecision = %d, want 3 (changing requires regenerating all SVG goldens + JS-port sync)", RenderPrecision)
	}
}
