package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeBoxplotShape(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"group": []string{"a", "a", "a", "b", "b", "b"},
		"score": []float64{0.1, 0.3, 0.45, 0.5, 0.7, 0.9},
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
	marks, err := encodeBoxplot(in)
	if err != nil {
		t.Fatalf("encodeBoxplot: %v", err)
	}
	// Per group with no outliers: 1 rect + 1 median rule + 2 stems + 2 caps = 6.
	// 2 groups → 12 marks.
	if len(marks) != 12 {
		t.Fatalf("len marks = %d, want 12 (6 per group × 2 groups, no outliers)", len(marks))
	}
}

func TestPrismEncodeBoxplotOutliers(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"group": []string{"a", "a", "a", "a", "a", "a", "a", "a", "a", "a"},
		"score": []float64{1, 2, 2, 3, 3, 3, 4, 4, 5, 100},
	})
	xBand := &bandScaleT{cats: []string{"a"}, rmin: 0, rmax: 100, padding: 0}
	yLin := &linScale{dmin: 0, dmax: 100, rmin: 100, rmax: 0}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "group", Scale: xBand},
		Y:      Channel{Field: "score", Scale: yLin},
		Layout: scene.Rect{W: 100, H: 100},
	}
	marks, err := encodeBoxplot(in)
	if err != nil {
		t.Fatalf("encodeBoxplot: %v", err)
	}
	// Box (1) + median (1) + 4 whisker = 6; outlier point = 7.
	outlierCount := 0
	for _, m := range marks {
		if m.Point != nil {
			outlierCount++
		}
	}
	if outlierCount != 1 {
		t.Errorf("outlier count = %d, want 1", outlierCount)
	}
}

func TestPrismEncodeBoxplotQuantileMath(t *testing.T) {
	// 10 known values; q1/median/q3 via R-7 linear interpolation.
	tbl := buildTable(t, map[string]any{
		"group": []string{"a", "a", "a", "a", "a", "a", "a", "a", "a", "a"},
		"score": []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	})
	xBand := &bandScaleT{cats: []string{"a"}, rmin: 0, rmax: 100, padding: 0}
	yLin := &linScale{dmin: 0, dmax: 10, rmin: 100, rmax: 0}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "group", Scale: xBand},
		Y:      Channel{Field: "score", Scale: yLin},
		Layout: scene.Rect{W: 100, H: 100},
	}
	summaries, err := ComputeBoxplotSummaries(in)
	if err != nil {
		t.Fatalf("ComputeBoxplotSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("len summaries = %d, want 1", len(summaries))
	}
	s := summaries[0]
	// R-7 quantiles on [1..10]: q1 = 3.25, median = 5.5, q3 = 7.75.
	if math.Abs(s.Q1-3.25) > 1e-9 {
		t.Errorf("Q1 = %g, want 3.25", s.Q1)
	}
	if math.Abs(s.Median-5.5) > 1e-9 {
		t.Errorf("Median = %g, want 5.5", s.Median)
	}
	if math.Abs(s.Q3-7.75) > 1e-9 {
		t.Errorf("Q3 = %g, want 7.75", s.Q3)
	}
}

func TestPrismQuantileBasic(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5}
	if got := Quantile(sorted, 0.5); got != 3 {
		t.Errorf("Quantile(0.5) = %g, want 3", got)
	}
	if got := Quantile(sorted, 0); got != 1 {
		t.Errorf("Quantile(0) = %g, want 1", got)
	}
	if got := Quantile(sorted, 1); got != 5 {
		t.Errorf("Quantile(1) = %g, want 5", got)
	}
}
