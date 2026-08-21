package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// TestApplyLabelOverlapKeepsEveryLabelOnAWellSpacedYAxis is the
// regression guard for the bug that produced "0 and 40, nothing else"
// on every vertical axis.
//
// A y axis carries ticks in ascending VALUE order, which is descending
// PIXEL order. The old overlap check compared each label's start
// against the previous label's end in slice order, so on a y axis
// `start < lastEnd` was true for every pair regardless of spacing, and
// parity-skip hid half the labels on an axis with 140px between them.
func TestApplyLabelOverlapKeepsEveryLabelOnAWellSpacedYAxis(t *testing.T) {
	// Ascending value, descending pixel — exactly what BuildAxis emits.
	ticks := []scene.Tick{
		{Value: 0.0, Pixel: 560, Label: "0"},
		{Value: 20.0, Pixel: 420, Label: "20"},
		{Value: 40.0, Pixel: 280, Label: "40"},
		{Value: 60.0, Pixel: 140, Label: "60"},
	}
	out := applyLabelOverlap(ticks, "parity", scene.AxisPositionLeft, DefaultLayoutStyle())
	for _, tk := range out {
		if tk.LabelHidden {
			t.Errorf("label %q hidden on an axis with 140px between ticks", tk.Label)
		}
	}
}

// TestApplyLabelOverlapStillHidesGenuineCollisions asserts the fix did
// not simply disable the pass.
func TestApplyLabelOverlapStillHidesGenuineCollisions(t *testing.T) {
	ticks := []scene.Tick{
		{Value: 0.0, Pixel: 560, Label: "0"},
		{Value: 1.0, Pixel: 554, Label: "1"},
		{Value: 2.0, Pixel: 548, Label: "2"},
		{Value: 3.0, Pixel: 542, Label: "3"},
	}
	out := applyLabelOverlap(ticks, "parity", scene.AxisPositionLeft, DefaultLayoutStyle())
	hidden := 0
	for _, tk := range out {
		if tk.LabelHidden {
			hidden++
		}
	}
	if hidden == 0 {
		t.Error("6px apart at an 11px label size must collide")
	}
}

// TestPlanGridGivesTheReferenceLinesToOneAxis pins the restraint rule:
// exactly one axis carries the grid, and it is the measure axis.
func TestPlanGridGivesTheReferenceLinesToOneAxis(t *testing.T) {
	band := &BandScale{Categories: []string{"a", "b"}, RangeMin: 0, RangeMax: 100}
	lin := &LinearScale{DomainMin: 0, DomainMax: 10, RangeMin: 100, RangeMax: 0}

	vertical := planGrid(band, lin)
	if !vertical.YGrid || vertical.XGrid {
		t.Errorf("vertical bars: want y grid only, got %+v", vertical)
	}
	if !vertical.YHideDomain || !vertical.YHideTicks {
		t.Errorf("the grid-carrying axis drops its domain and ticks, got %+v", vertical)
	}

	horizontal := planGrid(lin, band)
	if !horizontal.XGrid || horizontal.YGrid {
		t.Errorf("horizontal bars: want x grid only, got %+v", horizontal)
	}

	scatter := planGrid(lin, &LinearScale{DomainMin: 0, DomainMax: 1, RangeMin: 100, RangeMax: 0})
	if scatter.XGrid {
		t.Errorf("two continuous axes must not produce a mesh, got %+v", scatter)
	}

	heat := planGrid(band, &BandScale{Categories: []string{"x"}, RangeMin: 0, RangeMax: 10})
	if heat.XGrid || heat.YGrid {
		t.Errorf("two categorical axes need no grid, got %+v", heat)
	}
}

// TestNiceLinearDomainIsIdempotent guards the property the two-pass
// layout depends on: nicing runs once against the provisional rect and
// again against the final one, and the second call must not widen what
// the first produced. A version that added a step whenever an edge
// landed on a boundary turned 20-80 into 0-100 on the second pass.
func TestNiceLinearDomainIsIdempotent(t *testing.T) {
	cases := [][2]float64{{21, 66}, {0, 71}, {3.2, 3.6}, {-40, 55}, {1284000, 1755000}}
	for _, c := range cases {
		s := &LinearScale{DomainMin: c[0], DomainMax: c[1], RangeMin: 0, RangeMax: 500}
		niceLinearDomain(s, 4)
		first := [2]float64{s.DomainMin, s.DomainMax}
		niceLinearDomain(s, 4)
		if [2]float64{s.DomainMin, s.DomainMax} != first {
			t.Errorf("domain %v: first pass %v, second pass %v — not idempotent",
				c, first, [2]float64{s.DomainMin, s.DomainMax})
		}
	}
}

// TestNiceLinearDomainKeepsTicksNearTarget asserts the coarsening loop
// does its job: snapping outward must not multiply the tick count.
func TestNiceLinearDomainKeepsTicksNearTarget(t *testing.T) {
	s := &LinearScale{DomainMin: 3.196, DomainMax: 3.604, RangeMin: 0, RangeMax: 500}
	step := niceLinearDomain(s, 3)
	if step <= 0 {
		t.Fatal("expected a step")
	}
	n := (s.DomainMax - s.DomainMin) / step
	if n > 3*1.7 {
		t.Errorf("domain %g-%g at step %g gives %.0f intervals, target 3", s.DomainMin, s.DomainMax, step, n)
	}
}

// TestRelaxZeroBaselineOnlyForNonMeasuringMarks pins the honesty rule:
// a bar's axis includes zero because a bar's LENGTH is the value; a
// line's does not, because forcing zero flattens the variation the
// chart exists to show.
func TestRelaxZeroBaselineMarkRules(t *testing.T) {
	if !zeroBaselineMarks["bar"] || !zeroBaselineMarks["area"] {
		t.Error("bar and area measure from zero and must keep it")
	}
	if zeroBaselineMarks["line"] || zeroBaselineMarks["point"] {
		t.Error("line and point mark position, not magnitude")
	}
}

// TestBandShapeCapsAndCentres asserts the cap widens padding rather
// than shrinking the range, so bands stay under the labels that name
// them.
func TestBandShapeCapsAndCentres(t *testing.T) {
	b := &BandScale{Categories: []string{"only"}, RangeMin: 40, RangeMax: 780}
	applyBandShape(b, 0.28, 96)
	if b.RangeMin != 40 || b.RangeMax != 780 {
		t.Errorf("range moved: %+v — the cap must come from padding", b)
	}
	centre, err := b.BandCenter("only")
	if err != nil {
		t.Fatalf("BandCenter: %v", err)
	}
	if want := (40.0 + 780.0) / 2; centre < want-0.5 || centre > want+0.5 {
		t.Errorf("single band centre = %g, want %g", centre, want)
	}
}

// TestCentreBandForOnlyWrapsPointMarks is the regression guard for
// marks drawn half a band away from their own axis labels.
func TestCentreBandForOnlyWrapsPointMarks(t *testing.T) {
	b := &BandScale{Categories: []string{"Q1", "Q2"}, RangeMin: 0, RangeMax: 200, Padding: 0.28}

	lineScale := centreBandFor("line", b)
	got, err := lineScale.Apply("Q1")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want, _ := b.BandCenter("Q1")
	if got != want {
		t.Errorf("line on a band scale: Apply(Q1) = %g, want the band centre %g", got, want)
	}

	barScale := centreBandFor("bar", b)
	got, err = barScale.Apply("Q1")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	edge, _ := b.Apply("Q1")
	if got != edge {
		t.Errorf("bar on a band scale: Apply(Q1) = %g, want the left edge %g", got, edge)
	}
}
