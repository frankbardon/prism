package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

const epsilon = 1e-9

func nearly(got, want float64) bool { return math.Abs(got-want) < epsilon }

// A ranged bar: categorical y slot, x→x2 interval (the Gantt shape).
func TestPrismEncodeBarSpanX(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"task":  []string{"design", "build"},
		"start": []float64{0, 4},
		"end":   []float64{4, 10},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &bandScaleT{cats: []string{"design", "build"}, rmin: plot.Y, rmax: plot.Bottom(), padding: 0.2}
	out, _, err := Encode("bar", Inputs{
		Table:  tbl,
		X:      Channel{Field: "start", Scale: xs},
		X2:     Channel{Field: "end", Scale: xs},
		Y:      Channel{Field: "task", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(marks) = %d, want 2", len(out))
	}
	for i, m := range out {
		if m.Rect == nil {
			t.Fatalf("marks[%d] has no rect", i)
		}
		if !nearly(m.Rect.H, ys.BandWidth()) {
			t.Errorf("marks[%d].H = %g, want band width %g", i, m.Rect.H, ys.BandWidth())
		}
	}
	// Row 0 spans 0→4 on a 0..10 domain: a tenth of the plot per unit.
	unit := (plot.Right() - plot.X) / 10
	if !nearly(out[0].Rect.X, plot.X) || !nearly(out[0].Rect.W, 4*unit) {
		t.Errorf("marks[0] x span = (%g, w=%g), want (%g, w=%g)", out[0].Rect.X, out[0].Rect.W, plot.X, 4*unit)
	}
	if !nearly(out[1].Rect.X, plot.X+4*unit) || !nearly(out[1].Rect.W, 6*unit) {
		t.Errorf("marks[1] x span = (%g, w=%g), want (%g, w=%g)", out[1].Rect.X, out[1].Rect.W, plot.X+4*unit, 6*unit)
	}
}

// The real y band scale runs bottom-to-top, so its band width is
// negative and Apply returns the band's lower edge. A ranged bar must
// still come out with a positive height and a top-left origin.
func TestPrismEncodeBarSpanInvertedBand(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"task":  []string{"design", "build"},
		"start": []float64{0, 4},
		"end":   []float64{4, 10},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	// Inverted: rmin is the plot bottom, as encode.resolveChannel builds it.
	ys := &bandScaleT{cats: []string{"design", "build"}, rmin: plot.Bottom(), rmax: plot.Y, padding: 0.2}
	out, _, err := Encode("bar", Inputs{
		Table:  tbl,
		X:      Channel{Field: "start", Scale: xs},
		X2:     Channel{Field: "end", Scale: xs},
		Y:      Channel{Field: "task", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i, m := range out {
		if m.Rect.H <= 0 {
			t.Errorf("marks[%d].H = %g, want positive", i, m.Rect.H)
		}
		if !nearly(m.Rect.H, -ys.BandWidth()) {
			t.Errorf("marks[%d].H = %g, want %g", i, m.Rect.H, -ys.BandWidth())
		}
		if m.Rect.Y < plot.Y-epsilon || m.Rect.Y+m.Rect.H > plot.Bottom()+epsilon {
			t.Errorf("marks[%d] spans y %g..%g, outside the plot %g..%g",
				i, m.Rect.Y, m.Rect.Y+m.Rect.H, plot.Y, plot.Bottom())
		}
	}
}

// y2 on a bar replaces the baseline anchor with an explicit floor.
func TestPrismEncodeBarSpanY(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"region": []string{"north", "south"},
		"low":    []float64{2, 5},
		"high":   []float64{8, 9},
	})
	plot := plotRect()
	xs := &bandScaleT{cats: []string{"north", "south"}, rmin: plot.X, rmax: plot.Right(), padding: 0.2}
	ys := &linScale{dmin: 0, dmax: 10, rmin: plot.Bottom(), rmax: plot.Y}
	out, _, err := Encode("bar", Inputs{
		Table:  tbl,
		X:      Channel{Field: "region", Scale: xs},
		Y:      Channel{Field: "low", Scale: ys},
		Y2:     Channel{Field: "high", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(marks) = %d, want 2", len(out))
	}
	// Row 0 runs 2→8; a baseline-anchored bar would have run 0→8.
	lowPx, _ := ys.Apply(2.0)
	highPx, _ := ys.Apply(8.0)
	if !nearly(out[0].Rect.Y, highPx) {
		t.Errorf("marks[0].Y = %g, want the upper bound at %g", out[0].Rect.Y, highPx)
	}
	if !nearly(out[0].Rect.H, lowPx-highPx) {
		t.Errorf("marks[0].H = %g, want %g", out[0].Rect.H, lowPx-highPx)
	}
	if !nearly(out[0].Rect.W, xs.BandWidth()) {
		t.Errorf("marks[0].W = %g, want band width %g", out[0].Rect.W, xs.BandWidth())
	}
}

// A reversed pair still yields a positive extent.
func TestPrismEncodeBarSpanReversed(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"task":  []string{"only"},
		"start": []float64{9},
		"end":   []float64{3},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &bandScaleT{cats: []string{"only"}, rmin: plot.Y, rmax: plot.Bottom(), padding: 0}
	out, _, err := Encode("bar", Inputs{
		Table:  tbl,
		X:      Channel{Field: "start", Scale: xs},
		X2:     Channel{Field: "end", Scale: xs},
		Y:      Channel{Field: "task", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	unit := (plot.Right() - plot.X) / 10
	if out[0].Rect.W <= 0 {
		t.Fatalf("W = %g, want positive", out[0].Rect.W)
	}
	if !nearly(out[0].Rect.W, 6*unit) || !nearly(out[0].Rect.X, plot.X+3*unit) {
		t.Errorf("reversed span = (%g, w=%g), want (%g, w=%g)", out[0].Rect.X, out[0].Rect.W, plot.X+3*unit, 6*unit)
	}
}

// A ranged bar with no categorical axis left has nowhere to sit.
func TestPrismEncodeBarSpanNeedsBandOnOtherAxis(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"start": []float64{0, 4},
		"end":   []float64{4, 10},
		"depth": []float64{1, 2},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 4, rmin: plot.Bottom(), rmax: plot.Y}
	_, _, err := Encode("bar", Inputs{
		Table:  tbl,
		X:      Channel{Field: "start", Scale: xs},
		X2:     Channel{Field: "end", Scale: xs},
		Y:      Channel{Field: "depth", Scale: ys},
		Layout: plot,
	})
	if err == nil {
		t.Fatal("expected an error for a ranged bar with no band axis, got nil")
	}
}

// rect ranges on both axes at once.
func TestPrismEncodeRectSpanBothAxes(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x0": []float64{0, 5},
		"x1": []float64{5, 10},
		"y0": []float64{0, 2},
		"y1": []float64{2, 4},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 4, rmin: plot.Bottom(), rmax: plot.Y}
	out, _, err := Encode("rect", Inputs{
		Table:  tbl,
		X:      Channel{Field: "x0", Scale: xs},
		X2:     Channel{Field: "x1", Scale: xs},
		Y:      Channel{Field: "y0", Scale: ys},
		Y2:     Channel{Field: "y1", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(marks) = %d, want 2", len(out))
	}
	halfW := (plot.Right() - plot.X) / 2
	halfH := (plot.Bottom() - plot.Y) / 2
	if !nearly(out[0].Rect.W, halfW) || !nearly(out[0].Rect.H, halfH) {
		t.Errorf("marks[0] = %gx%g, want %gx%g", out[0].Rect.W, out[0].Rect.H, halfW, halfH)
	}
}

// rule becomes an interval segment: x→x2 at a fixed y.
func TestPrismEncodeRuleSpanHorizontal(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"lo":    []float64{1, 3},
		"hi":    []float64{4, 9},
		"score": []float64{2, 3},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 4, rmin: plot.Bottom(), rmax: plot.Y}
	out, _, err := Encode("rule", Inputs{
		Table:  tbl,
		X:      Channel{Field: "lo", Scale: xs},
		X2:     Channel{Field: "hi", Scale: xs},
		Y:      Channel{Field: "score", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(marks) = %d, want 2", len(out))
	}
	for i, m := range out {
		if m.Type != scene.MarkRule || m.Rule == nil {
			t.Fatalf("marks[%d] is not a rule", i)
		}
		if !nearly(m.Rule.Y1, m.Rule.Y2) {
			t.Errorf("marks[%d] not horizontal: Y1=%g Y2=%g", i, m.Rule.Y1, m.Rule.Y2)
		}
		// It must stop at x2, not run to the plot edge.
		if nearly(m.Rule.X2, plot.Right()) && !nearly(m.Rule.X1, plot.X) {
			t.Errorf("marks[%d] still spans the plot width", i)
		}
	}
	unit := (plot.Right() - plot.X) / 10
	if !nearly(out[0].Rule.X1, plot.X+unit) || !nearly(out[0].Rule.X2, plot.X+4*unit) {
		t.Errorf("marks[0] = (%g → %g), want (%g → %g)", out[0].Rule.X1, out[0].Rule.X2, plot.X+unit, plot.X+4*unit)
	}
}

// Endpoints keep their authored order: a descending interval stays
// descending rather than being normalised.
func TestPrismEncodeRuleSpanKeepsDirection(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"lo":    []float64{8},
		"hi":    []float64{2},
		"score": []float64{2},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 4, rmin: plot.Bottom(), rmax: plot.Y}
	out, _, err := Encode("rule", Inputs{
		Table:  tbl,
		X:      Channel{Field: "lo", Scale: xs},
		X2:     Channel{Field: "hi", Scale: xs},
		Y:      Channel{Field: "score", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if out[0].Rule.X1 <= out[0].Rule.X2 {
		t.Errorf("X1=%g X2=%g, want X1 > X2 for a descending interval", out[0].Rule.X1, out[0].Rule.X2)
	}
}

// An interval rule needs both base channels to know where to sit.
func TestPrismEncodeRuleSpanNeedsBothBases(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"lo": []float64{1, 3},
		"hi": []float64{4, 9},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	_, _, err := Encode("rule", Inputs{
		Table:  tbl,
		X:      Channel{Field: "lo", Scale: xs},
		X2:     Channel{Field: "hi", Scale: xs},
		Layout: plot,
	})
	if err == nil {
		t.Fatal("expected an error for an interval rule with no y, got nil")
	}
}

// y2 on an area replaces the implicit baseline with a read lower edge.
func TestPrismEncodeAreaSpanLowerEdge(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"t":  []float64{0, 5, 10},
		"lo": []float64{1, 2, 3},
		"hi": []float64{4, 6, 8},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 10, rmin: plot.Bottom(), rmax: plot.Y}
	out, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "t", Scale: xs},
		Y:      Channel{Field: "hi", Scale: ys},
		Y2:     Channel{Field: "lo", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(out) != 1 || out[0].Area == nil {
		t.Fatalf("want one area mark, got %d", len(out))
	}
	area := out[0].Area
	if len(area.Lower) != 3 {
		t.Fatalf("len(Lower) = %d, want 3", len(area.Lower))
	}
	baseline, _ := ys.Apply(0.0)
	for i, want := range []float64{1, 2, 3} {
		px, _ := ys.Apply(want)
		if !nearly(area.Lower[i][1], px) {
			t.Errorf("Lower[%d].y = %g, want %g (baseline would be %g)", i, area.Lower[i][1], px, baseline)
		}
	}
}

// Without y2 the area keeps its baseline lower edge untouched.
func TestPrismEncodeAreaWithoutSpanKeepsBaseline(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"t":  []float64{0, 5, 10},
		"hi": []float64{4, 6, 8},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 10, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 10, rmin: plot.Bottom(), rmax: plot.Y}
	out, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "t", Scale: xs},
		Y:      Channel{Field: "hi", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	baseline, _ := ys.Apply(0.0)
	for i, p := range out[0].Area.Lower {
		if !nearly(p[1], baseline) {
			t.Errorf("Lower[%d].y = %g, want baseline %g", i, p[1], baseline)
		}
	}
}

// A span channel with no scale (polar / geo / specialty marks resolve
// none) is inert — the mark keeps its unranged geometry.
func TestPrismSpanChannelWithoutScaleIsUnbound(t *testing.T) {
	if spanBound(Channel{Field: "end"}) {
		t.Error("spanBound should be false without a scale")
	}
	if spanBound(Channel{Scale: &linScale{}}) {
		t.Error("spanBound should be false without a field")
	}
	if !spanBound(Channel{Field: "end", Scale: &linScale{}}) {
		t.Error("spanBound should be true with both")
	}
}
