package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeViolinShape(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"group": []string{"a", "a", "a", "a", "a", "b", "b", "b", "b", "b"},
		"score": []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.5, 0.6, 0.7, 0.8, 0.9},
	})
	xBand := &bandScaleT{cats: []string{"a", "b"}, rmin: 40, rmax: 760, padding: 0.1}
	yLin := &linScale{dmin: 0, dmax: 1, rmin: 560, rmax: 20}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "group", Scale: xBand},
		Y:      Channel{Field: "score", Scale: yLin},
		Layout: scene.Rect{W: 800, H: 600},
		Style:  scene.Style{},
	}
	marks, err := encodeViolin(in)
	if err != nil {
		t.Fatalf("encodeViolin: %v", err)
	}
	if len(marks) != 2 {
		t.Fatalf("len marks = %d, want 2 (one per group)", len(marks))
	}
	for i, m := range marks {
		if m.Area == nil {
			t.Fatalf("mark[%d] missing Area geom", i)
		}
		if len(m.Area.Upper) != ViolinResolution {
			t.Errorf("mark[%d] Upper len = %d, want %d", i, len(m.Area.Upper), ViolinResolution)
		}
		if len(m.Area.Lower) != ViolinResolution {
			t.Errorf("mark[%d] Lower len = %d, want %d", i, len(m.Area.Lower), ViolinResolution)
		}
	}
}

func TestPrismEncodeViolinKDEEpanechnikov(t *testing.T) {
	// K(0) = 0.75
	if got := epanechnikovKernel(0); math.Abs(got-0.75) > 1e-9 {
		t.Errorf("K(0) = %g, want 0.75", got)
	}
	// K(1) = 0
	if got := epanechnikovKernel(1); got != 0 {
		t.Errorf("K(1) = %g, want 0", got)
	}
	// K(0.5) = 0.75 * (1 - 0.25) = 0.5625
	if got := epanechnikovKernel(0.5); math.Abs(got-0.5625) > 1e-9 {
		t.Errorf("K(0.5) = %g, want 0.5625", got)
	}
	// K(2) = 0
	if got := epanechnikovKernel(2); got != 0 {
		t.Errorf("K(2) = %g, want 0", got)
	}
	// K(-1) = 0
	if got := epanechnikovKernel(-1); got != 0 {
		t.Errorf("K(-1) = %g, want 0", got)
	}
}

func TestPrismEncodeViolinSilvermanBandwidth(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	mean := vmean(values)
	stdev := vstdev(values, mean)
	h := silverman(values, stdev)
	// 1.06 * stdev * n^(-1/5) for n=5, stdev = sqrt(2.5) ≈ 1.5811
	want := 1.06 * stdev * math.Pow(5.0, -1.0/5.0)
	if math.Abs(h-want) > 1e-9 {
		t.Errorf("silverman = %g, want %g", h, want)
	}
}

func TestPrismEncodeViolinDegenerateStdev(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"group": []string{"a"},
		"score": []float64{0.5},
	})
	xBand := &bandScaleT{cats: []string{"a"}, rmin: 0, rmax: 100, padding: 0}
	yLin := &linScale{dmin: 0, dmax: 1, rmin: 100, rmax: 0}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "group", Scale: xBand},
		Y:      Channel{Field: "score", Scale: yLin},
		Layout: scene.Rect{W: 100, H: 100},
	}
	marks, err := encodeViolin(in)
	if err != nil {
		t.Fatalf("encodeViolin: %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("len marks = %d, want 1", len(marks))
	}
	// No panic, AreaGeom still emitted.
}
