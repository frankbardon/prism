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
