package scale

import (
	"math"
	"testing"
)

const eps = 1e-9

func mustApply(t *testing.T, s Scale, cat string) float64 {
	t.Helper()
	got, err := s.Apply(cat)
	if err != nil {
		t.Fatalf("Apply(%q): %v", cat, err)
	}
	return got
}

// defaultBand is the band scale the resolver builds when the spec says
// nothing: Vega-Lite's 0.1 inner / 0.05 outer with a centred align.
func defaultBand(cats []string, min, max float64) *BandScale {
	return &BandScale{
		Categories:   cats,
		RangeMin:     min,
		RangeMax:     max,
		PaddingInner: 0.1,
		PaddingOuter: 0.05,
		Align:        0.5,
	}
}

// TestPrismBandDefaultsMatchHistoricLayout pins the one fact that keeps
// every committed bar / heatmap / boxplot golden byte-identical across
// the padding split: inner 0.1 + outer 0.05 + align 0.5 reproduces the
// pre-split "step = span/n, half an inner gap at each end" geometry
// exactly, for any category count.
func TestPrismBandDefaultsMatchHistoricLayout(t *testing.T) {
	for _, n := range []int{1, 2, 3, 5, 7, 12} {
		cats := make([]string, n)
		for i := range cats {
			cats[i] = string(rune('a' + i))
		}
		s := defaultBand(cats, 40, 760)
		span := 760.0 - 40.0
		step := span / float64(n)
		wantWidth := step * 0.9
		if math.Abs(s.BandWidth()-wantWidth) > eps {
			t.Fatalf("n=%d BandWidth() = %g, want %g", n, s.BandWidth(), wantWidth)
		}
		for i, c := range cats {
			want := 40 + float64(i)*step + step*0.1/2
			if got := mustApply(t, s, c); math.Abs(got-want) > eps {
				t.Fatalf("n=%d Apply(%q) = %g, want %g", n, c, got, want)
			}
		}
	}
}

// TestPrismBandInvertedRangeKeepsSignedStep guards the y-axis contract:
// a bottom-to-top range yields a negative step and a negative
// BandWidth, which the rect / heatmap encoders normalise.
func TestPrismBandInvertedRangeKeepsSignedStep(t *testing.T) {
	s := defaultBand([]string{"a", "b", "c"}, 560, 20)
	if s.BandWidth() >= 0 {
		t.Fatalf("inverted range must give a negative BandWidth, got %g", s.BandWidth())
	}
	// Mirror of the upright case: same magnitudes, opposite direction.
	up := defaultBand([]string{"a", "b", "c"}, 20, 560)
	if math.Abs(math.Abs(s.BandWidth())-up.BandWidth()) > eps {
		t.Fatalf("|BandWidth| differs between directions: %g vs %g", s.BandWidth(), up.BandWidth())
	}
	step := -(560.0 - 20.0) / 3
	for i, c := range []string{"a", "b", "c"} {
		want := 560 + float64(i)*step + step*0.1/2
		if got := mustApply(t, s, c); math.Abs(got-want) > eps {
			t.Fatalf("Apply(%q) = %g, want %g", c, got, want)
		}
	}
}

func TestPrismBandPaddingInnerWidensGaps(t *testing.T) {
	loose := &BandScale{
		Categories:   []string{"a", "b"},
		RangeMin:     0,
		RangeMax:     200,
		PaddingInner: 0.5,
		PaddingOuter: 0.05,
		Align:        0.5,
	}
	// step = 200 / (2 - 0.5 + 0.1) = 125, width = 62.5.
	if got := loose.BandWidth(); math.Abs(got-62.5) > eps {
		t.Fatalf("BandWidth() = %g, want 62.5", got)
	}
	// offset = (200 - 125*1.5) * 0.5 = 6.25.
	if got := mustApply(t, loose, "a"); math.Abs(got-6.25) > eps {
		t.Fatalf("Apply(a) = %g, want 6.25", got)
	}
	if got := mustApply(t, loose, "b"); math.Abs(got-131.25) > eps {
		t.Fatalf("Apply(b) = %g, want 131.25", got)
	}
}

func TestPrismBandPaddingOuterIsIndependent(t *testing.T) {
	// No outer padding: the first band starts flush at the range edge
	// and the last one ends flush at the far edge.
	flush := &BandScale{
		Categories:   []string{"a", "b"},
		RangeMin:     0,
		RangeMax:     190,
		PaddingInner: 0.1,
		PaddingOuter: 0,
		Align:        0.5,
	}
	if got := mustApply(t, flush, "a"); math.Abs(got) > eps {
		t.Fatalf("Apply(a) = %g, want 0 with no outer padding", got)
	}
	last := mustApply(t, flush, "b") + flush.BandWidth()
	if math.Abs(last-190) > eps {
		t.Fatalf("last band ends at %g, want 190", last)
	}
	// Outer padding pushes both ends inward without changing the
	// band-to-band ratio.
	padded := &BandScale{
		Categories:   []string{"a", "b"},
		RangeMin:     0,
		RangeMax:     190,
		PaddingInner: 0.1,
		PaddingOuter: 0.25,
		Align:        0.5,
	}
	if got := mustApply(t, padded, "a"); got <= 0 {
		t.Fatalf("Apply(a) = %g, want a positive inset with outer padding", got)
	}
}

func TestPrismBandAlignShiftsWithinSlack(t *testing.T) {
	mk := func(align float64) *BandScale {
		return &BandScale{
			Categories:   []string{"a", "b"},
			RangeMin:     0,
			RangeMax:     200,
			PaddingInner: 0.1,
			PaddingOuter: 0.25,
			Align:        align,
		}
	}
	start := mustApply(t, mk(0), "a")
	mid := mustApply(t, mk(0.5), "a")
	end := mustApply(t, mk(1), "a")
	if !(start < mid && mid < end) {
		t.Fatalf("align must move the leading edge: %g < %g < %g", start, mid, end)
	}
	if math.Abs(start) > eps {
		t.Fatalf("align 0 must pack against the range start, got %g", start)
	}
	// Align never changes the band width, only where the slack sits.
	if math.Abs(mk(0).BandWidth()-mk(1).BandWidth()) > eps {
		t.Fatal("align changed BandWidth")
	}
}

func TestPrismBandRoundQuantisesLayout(t *testing.T) {
	s := &BandScale{
		Categories:   []string{"a", "b", "c"},
		RangeMin:     0,
		RangeMax:     305,
		PaddingInner: 0.1,
		PaddingOuter: 0.05,
		Align:        0.5,
		Round:        true,
	}
	if w := s.BandWidth(); w != math.Trunc(w) {
		t.Fatalf("BandWidth() = %g, want a whole pixel", w)
	}
	for _, c := range []string{"a", "b", "c"} {
		if got := mustApply(t, s, c); got != math.Trunc(got) {
			t.Fatalf("Apply(%q) = %g, want a whole pixel", c, got)
		}
	}
	// Rounding is opt-in: the same scale without it is fractional.
	loose := *s
	loose.Round = false
	if w := loose.BandWidth(); w == math.Trunc(w) {
		t.Fatalf("expected a fractional BandWidth without round, got %g", w)
	}
}

func TestPrismBandReverseSwapsSlotsNotWidths(t *testing.T) {
	fwd := defaultBand([]string{"a", "b", "c"}, 0, 300)
	rev := defaultBand([]string{"a", "b", "c"}, 0, 300)
	rev.Reverse = true
	if math.Abs(fwd.BandWidth()-rev.BandWidth()) > eps {
		t.Fatalf("reverse changed BandWidth: %g vs %g", fwd.BandWidth(), rev.BandWidth())
	}
	if got, want := mustApply(t, rev, "a"), mustApply(t, fwd, "c"); math.Abs(got-want) > eps {
		t.Fatalf("reversed Apply(a) = %g, want the forward slot of c (%g)", got, want)
	}
	if got, want := mustApply(t, rev, "c"), mustApply(t, fwd, "a"); math.Abs(got-want) > eps {
		t.Fatalf("reversed Apply(c) = %g, want the forward slot of a (%g)", got, want)
	}
	// The middle category is its own mirror.
	if got, want := mustApply(t, rev, "b"), mustApply(t, fwd, "b"); math.Abs(got-want) > eps {
		t.Fatalf("reversed Apply(b) = %g, want %g", got, want)
	}
}

func TestPrismPointDefaultsMatchHistoricLayout(t *testing.T) {
	for _, n := range []int{1, 2, 4, 9} {
		cats := make([]string, n)
		for i := range cats {
			cats[i] = string(rune('a' + i))
		}
		s := &PointScale{Categories: cats, RangeMin: 0, RangeMax: 400, Padding: 0.5, Align: 0.5}
		divisor := float64(n) - 1 + 1
		step := 400 / divisor
		for i, c := range cats {
			want := step * (0.5 + float64(i))
			if got := mustApply(t, s, c); math.Abs(got-want) > eps {
				t.Fatalf("n=%d Apply(%q) = %g, want %g", n, c, got, want)
			}
		}
	}
}

func TestPrismPointReverseAndRound(t *testing.T) {
	fwd := &PointScale{Categories: []string{"a", "b", "c"}, RangeMin: 0, RangeMax: 401, Padding: 0.5, Align: 0.5}
	rev := *fwd
	rev.Reverse = true
	if got, want := mustApply(t, &rev, "a"), mustApply(t, fwd, "c"); math.Abs(got-want) > eps {
		t.Fatalf("reversed Apply(a) = %g, want %g", got, want)
	}
	rounded := *fwd
	rounded.Round = true
	for _, c := range []string{"a", "b", "c"} {
		if got := mustApply(t, &rounded, c); got != math.Trunc(got) {
			t.Fatalf("Apply(%q) = %g, want a whole pixel", c, got)
		}
	}
}

func TestPrismContinuousClampPinsToRangeEdge(t *testing.T) {
	open := &LinearScale{DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 200}
	closed := &LinearScale{
		DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 200,
		ContinuousOutput: ContinuousOutput{Clamp: true},
	}
	over, err := open.Apply(150.0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if math.Abs(over-300) > eps {
		t.Fatalf("unclamped Apply(150) = %g, want 300 (overflow is the default)", over)
	}
	pinned, err := closed.Apply(150.0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if math.Abs(pinned-200) > eps {
		t.Fatalf("clamped Apply(150) = %g, want 200", pinned)
	}
	under, err := closed.Apply(-50.0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if math.Abs(under) > eps {
		t.Fatalf("clamped Apply(-50) = %g, want 0", under)
	}
	// Clamping never disturbs an in-domain value.
	mid, err := closed.Apply(25.0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if math.Abs(mid-50) > eps {
		t.Fatalf("clamped Apply(25) = %g, want 50", mid)
	}
}

func TestPrismContinuousClampOnInvertedRange(t *testing.T) {
	// A y scale runs bottom-to-top: RangeMin is the *bottom* pixel.
	s := &LinearScale{
		DomainMin: 0, DomainMax: 100, RangeMin: 560, RangeMax: 20,
		ContinuousOutput: ContinuousOutput{Clamp: true},
	}
	top, _ := s.Apply(400.0)
	if math.Abs(top-20) > eps {
		t.Fatalf("clamped Apply(400) = %g, want the top pixel 20", top)
	}
	bottom, _ := s.Apply(-5.0)
	if math.Abs(bottom-560) > eps {
		t.Fatalf("clamped Apply(-5) = %g, want the bottom pixel 560", bottom)
	}
}

func TestPrismContinuousRoundQuantisesOutput(t *testing.T) {
	s := &LinearScale{
		DomainMin: 0, DomainMax: 3, RangeMin: 0, RangeMax: 100,
		ContinuousOutput: ContinuousOutput{Round: true},
	}
	got, _ := s.Apply(1.0)
	if got != math.Trunc(got) {
		t.Fatalf("Apply(1) = %g, want a whole pixel", got)
	}
	loose := &LinearScale{DomainMin: 0, DomainMax: 3, RangeMin: 0, RangeMax: 100}
	raw, _ := loose.Apply(1.0)
	if raw == math.Trunc(raw) {
		t.Fatalf("expected a fractional pixel without round, got %g", raw)
	}
}

func TestPrismLogAndPowHonourClamp(t *testing.T) {
	lg := &LogScale{
		Base: 10, DomainMin: 1, DomainMax: 100, RangeMin: 0, RangeMax: 200,
		ContinuousOutput: ContinuousOutput{Clamp: true},
	}
	got, err := lg.Apply(10000.0)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if math.Abs(got-200) > eps {
		t.Fatalf("clamped log Apply(10000) = %g, want 200", got)
	}
	// A non-positive value still has no image on a log scale.
	if _, err := lg.Apply(0.0); err == nil {
		t.Fatal("expected PRISM_SPEC_010 for a non-positive value even with clamp on")
	}
	pw := &PowScale{
		Exp: 2, DomainMin: 0, DomainMax: 10, RangeMin: 0, RangeMax: 100,
		ContinuousOutput: ContinuousOutput{Clamp: true},
	}
	if got, _ := pw.Apply(50.0); math.Abs(got-100) > eps {
		t.Fatalf("clamped pow Apply(50) = %g, want 100", got)
	}
	sq := &SqrtScale{Inner: PowScale{
		DomainMin: 0, DomainMax: 10, RangeMin: 0, RangeMax: 100,
		ContinuousOutput: ContinuousOutput{Clamp: true},
	}}
	if got, _ := sq.Apply(50.0); math.Abs(got-100) > eps {
		t.Fatalf("clamped sqrt Apply(50) = %g, want 100", got)
	}
}
