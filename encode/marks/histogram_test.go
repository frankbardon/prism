package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func TestPrismEncodeHistogramAutoBin(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.1, 0.2, 0.25, 0.4, 0.45, 0.6, 0.62, 0.75},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "score"},
		Layout: plotRect(),
		Style:  scene.Style{},
	}
	hr, err := EncodeHistogram(in)
	if err != nil {
		t.Fatalf("EncodeHistogram: %v", err)
	}
	if len(hr.BinEdges) < 2 {
		t.Fatalf("len(BinEdges) = %d, want ≥ 2", len(hr.BinEdges))
	}
	if len(hr.Counts) != len(hr.BinEdges)-1 {
		t.Errorf("len(Counts) = %d, want %d", len(hr.Counts), len(hr.BinEdges)-1)
	}
	// Sum of counts must equal row count.
	sum := 0
	for _, c := range hr.Counts {
		sum += c
	}
	if sum != 8 {
		t.Errorf("Σ counts = %d, want 8", sum)
	}
	// Bin width must be a "nice" round number (multiple of 0.1, 0.25,
	// 0.5, or 1.0 — niceStep emits those).
	w := hr.BinEdges[1] - hr.BinEdges[0]
	nice := false
	for _, candidate := range []float64{0.1, 0.2, 0.25, 0.5, 1.0} {
		if abs(w-candidate) < 1e-9 {
			nice = true
			break
		}
	}
	if !nice {
		t.Errorf("bin width %g is not a nice round value", w)
	}
	if len(hr.Marks) != len(hr.Counts) {
		t.Errorf("len(Marks) = %d, want %d", len(hr.Marks), len(hr.Counts))
	}
}

func TestPrismEncodeHistogramMaxbinsOverride(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8},
	})
	four := 4
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "score"},
		Layout: plotRect(),
		Mark:   &spec.MarkDef{Maxbins: &four},
	}
	hr, err := EncodeHistogram(in)
	if err != nil {
		t.Fatalf("EncodeHistogram: %v", err)
	}
	// With maxbins=4 on [0.1, 0.8] range, niceStep should emit width
	// 0.2 → 4 bins.
	if len(hr.Counts) > 5 {
		t.Errorf("maxbins=4 yielded %d bins; expected ≤ 5", len(hr.Counts))
	}
}

func TestPrismEncodeHistogramEmptyTable(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "score"},
		Layout: plotRect(),
	}
	hr, err := EncodeHistogram(in)
	if err != nil {
		t.Fatalf("EncodeHistogram: %v", err)
	}
	if len(hr.Marks) != 0 {
		t.Errorf("len(Marks) = %d, want 0", len(hr.Marks))
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
