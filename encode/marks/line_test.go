package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeLineBasic(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x":     []float64{0, 50, 100},
		"score": []float64{0.4, 0.6, 0.8},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 100, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("line", Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: xs},
		Y:      Channel{Field: "score", Scale: ys},
		Layout: plot,
		Style:  scene.Style{},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("len(marks) = %d, want 1", len(marks))
	}
	if marks[0].Type != scene.MarkLine || marks[0].Line == nil {
		t.Fatalf("expected MarkLine, got %s rect=%v", marks[0].Type, marks[0].Line)
	}
	if len(marks[0].Line.Points) != 3 {
		t.Errorf("Points len = %d, want 3", len(marks[0].Line.Points))
	}
	// First point at x=plot.X (left edge).
	if marks[0].Line.Points[0][0] != plot.X {
		t.Errorf("first point x = %g, want %g", marks[0].Line.Points[0][0], plot.X)
	}
}
