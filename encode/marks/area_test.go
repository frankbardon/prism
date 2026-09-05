package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeAreaBasic(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"day": []float64{1, 2, 3},
		"vol": []float64{100, 175, 240},
	})
	plot := plotRect()
	xs := &linScale{dmin: 1, dmax: 3, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 240, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "day", Scale: xs},
		Y:      Channel{Field: "vol", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("len(marks) = %d, want 1", len(marks))
	}
	if marks[0].Type != scene.MarkArea || marks[0].Area == nil {
		t.Fatalf("expected MarkArea, got %s area=%v", marks[0].Type, marks[0].Area)
	}
	if len(marks[0].Area.Upper) != 3 {
		t.Errorf("Upper len = %d, want 3", len(marks[0].Area.Upper))
	}
	// Lower is the y=0 baseline edge: one point per Upper x, all at the
	// pixel where the data value is 0. For an all-positive domain whose
	// scale min is 0, that pixel is the plot bottom.
	if len(marks[0].Area.Lower) != 3 {
		t.Fatalf("Lower len = %d, want 3 (baseline edge)", len(marks[0].Area.Lower))
	}
	wantBaseline := plot.Bottom()
	for i, p := range marks[0].Area.Lower {
		if p[0] != marks[0].Area.Upper[i][0] {
			t.Errorf("Lower[%d].x = %v, want %v (matching Upper x)", i, p[0], marks[0].Area.Upper[i][0])
		}
		if p[1] != wantBaseline {
			t.Errorf("Lower[%d].y = %v, want %v (y=0 baseline)", i, p[1], wantBaseline)
		}
	}
}

// TestPrismEncodeAreaZeroCrossing verifies the baseline edge sits at
// the mid-plot y=0 pixel for a domain spanning negative and positive
// values, so the fill renders above and below it.
func TestPrismEncodeAreaZeroCrossing(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"day": []float64{1, 2, 3, 4},
		"pnl": []float64{12, -8, 4, -14},
	})
	plot := plotRect()
	xs := &linScale{dmin: 1, dmax: 4, rmin: plot.X, rmax: plot.Right()}
	// Symmetric domain [-14, 14] → value 0 maps to the vertical center.
	ys := &linScale{dmin: -14, dmax: 14, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "day", Scale: xs},
		Y:      Channel{Field: "pnl", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	wantBaseline, _ := ys.Apply(float64(0))
	if wantBaseline <= plot.Y || wantBaseline >= plot.Bottom() {
		t.Fatalf("baseline %v should sit mid-plot, between %v and %v", wantBaseline, plot.Y, plot.Bottom())
	}
	for i, p := range marks[0].Area.Lower {
		if p[1] != wantBaseline {
			t.Errorf("Lower[%d].y = %v, want %v (mid-plot zero line)", i, p[1], wantBaseline)
		}
	}
}

// TestPrismEncodeAreaMultiSeries verifies that a color-encoded area
// mark splits into one scene.Mark per distinct category (mirrors
// TestPrismEncodeLineMultiSeries), each with its own group's Upper
// points sorted by x ascending, a Lower baseline edge matching that
// sorted x order, and its own resolved fill color from the
// Categories/Palette pair the legend uses.
func TestPrismEncodeAreaMultiSeries(t *testing.T) {
	// Interleaved + out-of-x-order per group, identity scales so
	// Scale.Apply returns the raw value.
	tbl := buildTable(t, map[string]any{
		"day":    []float64{2, 1, 1, 3, 3, 2},
		"vol":    []float64{20, 10, 100, 30, 300, 200},
		"series": []string{"a", "a", "b", "a", "b", "b"},
	})
	xs := &linScale{dmin: 1, dmax: 3, rmin: 1, rmax: 3}
	ys := &linScale{dmin: 0, dmax: 300, rmin: 0, rmax: 300}
	palette := []*scene.Color{mustColor("#3b82f6"), mustColor("#ef4444")}
	marks, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "day", Scale: xs},
		Y:      Channel{Field: "vol", Scale: ys},
		Layout: plotRect(),
		Color: &ColorChannel{
			Field:      "series",
			Categories: []string{"a", "b"},
			Palette:    palette,
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 2 {
		t.Fatalf("len(marks) = %d, want 2 (one ribbon per series)", len(marks))
	}

	wantUpperA := [][2]float64{{1, 10}, {2, 20}, {3, 30}}
	wantUpperB := [][2]float64{{1, 100}, {2, 200}, {3, 300}}
	wantLower := [][2]float64{{1, 0}, {2, 0}, {3, 0}}

	if marks[0].Type != scene.MarkArea || marks[0].Area == nil {
		t.Fatalf("marks[0] not an Area: type=%s area=%v", marks[0].Type, marks[0].Area)
	}
	if got := marks[0].Area.Upper; !pointsEqual(got, wantUpperA) {
		t.Errorf("marks[0] (series a) Upper = %v, want %v (sorted by x)", got, wantUpperA)
	}
	if got := marks[0].Area.Lower; !pointsEqual(got, wantLower) {
		t.Errorf("marks[0] (series a) Lower = %v, want %v", got, wantLower)
	}
	if marks[0].Style.Fill == nil || marks[0].Style.Fill.Hex() != "#3b82f6" {
		t.Errorf("marks[0].Style.Fill = %v, want #3b82f6", marks[0].Style.Fill)
	}
	if marks[0].ID != "area-0" {
		t.Errorf("marks[0].ID = %q, want area-0", marks[0].ID)
	}

	if marks[1].Type != scene.MarkArea || marks[1].Area == nil {
		t.Fatalf("marks[1] not an Area: type=%s area=%v", marks[1].Type, marks[1].Area)
	}
	if got := marks[1].Area.Upper; !pointsEqual(got, wantUpperB) {
		t.Errorf("marks[1] (series b) Upper = %v, want %v (sorted by x)", got, wantUpperB)
	}
	if marks[1].Style.Fill == nil || marks[1].Style.Fill.Hex() != "#ef4444" {
		t.Errorf("marks[1].Style.Fill = %v, want #ef4444", marks[1].Style.Fill)
	}
	if marks[1].ID != "area-1" {
		t.Errorf("marks[1].ID = %q, want area-1", marks[1].ID)
	}
}
