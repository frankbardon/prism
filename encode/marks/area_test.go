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
	if marks[0].Area.Lower != nil {
		t.Errorf("Lower should be nil (baseline 0), got %v", marks[0].Area.Lower)
	}
}
