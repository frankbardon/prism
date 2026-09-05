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

// TestPrismEncodeLineMultiSeries verifies that a color-encoded line
// mark splits into one scene.Mark per distinct category (Vega-Lite
// semantics), each carrying only its own group's points sorted by x
// ascending (upstream rows interleave the two series out of x-order,
// mirroring the gallery's multi_series_line fixture) and its own
// resolved stroke color from the same Categories/Palette pair the
// legend uses.
func TestPrismEncodeLineMultiSeries(t *testing.T) {
	// Interleaved + out-of-x-order per group: series "a" rows appear
	// at x = 2, 1, 3 (upstream order); series "b" rows appear at
	// x = 1, 3, 2. Identity scales (domain == range) so Scale.Apply
	// returns the raw value, letting the test assert exact points.
	tbl := buildTable(t, map[string]any{
		"x":      []float64{2, 1, 1, 3, 3, 2},
		"y":      []float64{20, 10, 100, 30, 300, 200},
		"series": []string{"a", "a", "b", "a", "b", "b"},
	})
	xs := &linScale{dmin: 1, dmax: 3, rmin: 1, rmax: 3}
	ys := &linScale{dmin: 0, dmax: 300, rmin: 0, rmax: 300}
	palette := []*scene.Color{mustColor("#3b82f6"), mustColor("#ef4444")}
	marks, _, err := Encode("line", Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: xs},
		Y:      Channel{Field: "y", Scale: ys},
		Layout: plotRect(),
		Style:  scene.Style{StrokeWidth: 1.5},
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
		t.Fatalf("len(marks) = %d, want 2 (one polyline per series)", len(marks))
	}

	wantA := [][2]float64{{1, 10}, {2, 20}, {3, 30}}
	wantB := [][2]float64{{1, 100}, {2, 200}, {3, 300}}

	if marks[0].Type != scene.MarkLine || marks[0].Line == nil {
		t.Fatalf("marks[0] not a Line: type=%s line=%v", marks[0].Type, marks[0].Line)
	}
	if got := marks[0].Line.Points; !pointsEqual(got, wantA) {
		t.Errorf("marks[0] (series a) points = %v, want %v (sorted by x)", got, wantA)
	}
	if marks[0].Style.Stroke == nil || marks[0].Style.Stroke.Hex() != "#3b82f6" {
		t.Errorf("marks[0].Style.Stroke = %v, want #3b82f6", marks[0].Style.Stroke)
	}
	if marks[0].Style.StrokeWidth != 1.5 {
		t.Errorf("marks[0].Style.StrokeWidth = %g, want 1.5 (preserved from in.Style)", marks[0].Style.StrokeWidth)
	}
	if marks[0].ID != "line-0" {
		t.Errorf("marks[0].ID = %q, want line-0", marks[0].ID)
	}

	if marks[1].Type != scene.MarkLine || marks[1].Line == nil {
		t.Fatalf("marks[1] not a Line: type=%s line=%v", marks[1].Type, marks[1].Line)
	}
	if got := marks[1].Line.Points; !pointsEqual(got, wantB) {
		t.Errorf("marks[1] (series b) points = %v, want %v (sorted by x)", got, wantB)
	}
	if marks[1].Style.Stroke == nil || marks[1].Style.Stroke.Hex() != "#ef4444" {
		t.Errorf("marks[1].Style.Stroke = %v, want #ef4444", marks[1].Style.Stroke)
	}
	if marks[1].ID != "line-1" {
		t.Errorf("marks[1].ID = %q, want line-1", marks[1].ID)
	}
}

func pointsEqual(got, want [][2]float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
