package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeBarBasic(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"brand_id": []string{"alpha", "beta", "gamma"},
		"score":    []float64{0.42, 0.71, 0.58},
	})
	plot := plotRect()
	xs := &bandScaleT{cats: []string{"alpha", "beta", "gamma"}, rmin: plot.X, rmax: plot.Right(), padding: 0}
	// y inverted: low data values → high pixels.
	ys := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("bar", Inputs{
		Table:  tbl,
		X:      Channel{Field: "brand_id", Scale: xs},
		Y:      Channel{Field: "score", Scale: ys},
		Layout: plot,
		Style:  scene.Style{},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 3 {
		t.Fatalf("len(marks) = %d, want 3", len(marks))
	}
	// Every mark is a RectGeom.
	for i, m := range marks {
		if m.Type != scene.MarkRect || m.Rect == nil {
			t.Errorf("marks[%d] not a Rect mark: type=%s rect=%v", i, m.Type, m.Rect)
		}
	}
	// Bar widths match the band width.
	wantWidth := xs.BandWidth()
	if math.Abs(marks[0].Rect.W-wantWidth) > 1e-9 {
		t.Errorf("bar width = %g, want %g", marks[0].Rect.W, wantWidth)
	}
	// Bar 0 (score 0.42) should be shorter than bar 1 (score 0.71).
	if marks[0].Rect.H >= marks[1].Rect.H {
		t.Errorf("bar0.H=%g should be < bar1.H=%g", marks[0].Rect.H, marks[1].Rect.H)
	}
}
