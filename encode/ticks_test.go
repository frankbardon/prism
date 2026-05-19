package encode

import (
	"math"
	"reflect"
	"testing"
)

// TestPrismNiceTicksCanonical pins the port against D3's reference
// output. Cases verified by running d3-array's ticks() in a Node
// shell at the same versions documented in NiceTicks's source comment.
func TestPrismNiceTicksCanonical(t *testing.T) {
	cases := []struct {
		min, max float64
		count    int
		want     []float64
	}{
		{0, 10, 5, []float64{0, 2, 4, 6, 8, 10}},
		{0, 1, 5, []float64{0, 0.2, 0.4, 0.6, 0.8, 1.0}},
		{0, 100, 10, []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}},
		{0, 5, 5, []float64{0, 1, 2, 3, 4, 5}},
		{-50, 50, 5, []float64{-40, -20, 0, 20, 40}},
		{1.1, 10.7, 5, []float64{2, 4, 6, 8, 10}},
	}
	for _, tc := range cases {
		got := NiceTicks(tc.min, tc.max, tc.count)
		if !floatsAlmostEqual(got, tc.want, 1e-9) {
			t.Errorf("NiceTicks(%g, %g, %d) = %v, want %v", tc.min, tc.max, tc.count, got, tc.want)
		}
	}
}

// TestPrismNiceTicksDegenerate covers single-value and reversed domains.
func TestPrismNiceTicksDegenerate(t *testing.T) {
	// Single-value domain returns one tick.
	got := NiceTicks(5, 5, 5)
	if len(got) != 1 || got[0] != 5 {
		t.Errorf("NiceTicks(5,5,5) = %v, want [5]", got)
	}
	// Reversed-domain returns the ticks in descending order.
	asc := NiceTicks(0, 10, 5)
	desc := NiceTicks(10, 0, 5)
	if len(asc) != len(desc) {
		t.Fatalf("ascending/descending lengths differ: %v vs %v", asc, desc)
	}
	for i := range asc {
		if asc[i] != desc[len(desc)-1-i] {
			t.Errorf("descending tick %d = %v, want %v", i, desc[i], asc[len(asc)-1-i])
		}
	}
}

func TestPrismBandTicksCenter(t *testing.T) {
	scale := &BandScale{
		Categories: []string{"a", "b", "c"},
		RangeMin:   0,
		RangeMax:   300,
		Padding:    0,
	}
	ticks := BandTicks(scale)
	if len(ticks) != 3 {
		t.Fatalf("BandTicks len = %d, want 3", len(ticks))
	}
	// Centers: band width 100 → centers at 50, 150, 250.
	wantCenters := []float64{50, 150, 250}
	for i, tk := range ticks {
		if math.Abs(tk.Pixel-wantCenters[i]) > 1e-9 {
			t.Errorf("ticks[%d].Pixel = %g, want %g", i, tk.Pixel, wantCenters[i])
		}
		if tk.Label != scale.Categories[i] {
			t.Errorf("ticks[%d].Label = %q, want %q", i, tk.Label, scale.Categories[i])
		}
	}
}

func TestPrismTicksWithLabels(t *testing.T) {
	scale := &LinearScale{DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 800}
	ticks, err := TicksWithLabels([]float64{0, 25, 50, 75, 100}, scale, "")
	if err != nil {
		t.Fatalf("TicksWithLabels: %v", err)
	}
	wantPixels := []float64{0, 200, 400, 600, 800}
	wantLabels := []string{"0", "25", "50", "75", "100"}
	for i, tk := range ticks {
		if math.Abs(tk.Pixel-wantPixels[i]) > 1e-9 {
			t.Errorf("ticks[%d].Pixel = %g, want %g", i, tk.Pixel, wantPixels[i])
		}
		if tk.Label != wantLabels[i] {
			t.Errorf("ticks[%d].Label = %q, want %q", i, tk.Label, wantLabels[i])
		}
	}
}

func floatsAlmostEqual(a, b []float64, eps float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > eps {
			return false
		}
	}
	return true
}

// Sanity: reflect import survives the linter when only used for
// future expansion. Currently unused in assertion paths.
var _ = reflect.DeepEqual
