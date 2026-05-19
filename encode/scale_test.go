package encode

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
)

func TestPrismLinearScale(t *testing.T) {
	s := &LinearScale{DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 800}
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{50, 400},
		{100, 800},
		{25, 200},
	}
	for _, tc := range cases {
		got, err := s.Apply(tc.in)
		if err != nil {
			t.Fatalf("Apply(%g): %v", tc.in, err)
		}
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("Apply(%g) = %g, want %g", tc.in, got, tc.want)
		}
	}
	if s.Type() != scene.ScaleLinear {
		t.Errorf("Type() = %q, want %q", s.Type(), scene.ScaleLinear)
	}
}

func TestPrismBandScale(t *testing.T) {
	s := &BandScale{
		Categories: []string{"a", "b", "c"},
		RangeMin:   0,
		RangeMax:   300,
		Padding:    0,
	}
	if s.BandWidth() != 100 {
		t.Errorf("BandWidth() = %g, want 100", s.BandWidth())
	}
	cases := []struct {
		in   string
		want float64
	}{
		{"a", 0},
		{"b", 100},
		{"c", 200},
	}
	for _, tc := range cases {
		got, err := s.Apply(tc.in)
		if err != nil {
			t.Fatalf("Apply(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("Apply(%q) = %g, want %g", tc.in, got, tc.want)
		}
	}
	// Out-of-domain category errors.
	if _, err := s.Apply("z"); err == nil {
		t.Error("expected error for out-of-domain category, got nil")
	}
}

func TestPrismBandScalePadding(t *testing.T) {
	s := &BandScale{
		Categories: []string{"a", "b"},
		RangeMin:   0,
		RangeMax:   200,
		Padding:    0.1,
	}
	// Step = 100, pad = 100 * 0.1 / 2 = 5; first band at 5, second at 105.
	gotA, _ := s.Apply("a")
	if math.Abs(gotA-5) > 1e-9 {
		t.Errorf("Apply(a) = %g, want 5", gotA)
	}
	gotB, _ := s.Apply("b")
	if math.Abs(gotB-105) > 1e-9 {
		t.Errorf("Apply(b) = %g, want 105", gotB)
	}
	if math.Abs(s.BandWidth()-90) > 1e-9 {
		t.Errorf("BandWidth() = %g, want 90", s.BandWidth())
	}
}

func TestPrismOrdinalScale(t *testing.T) {
	s := &OrdinalScale{
		Categories: []string{"x", "y", "z"},
		Positions:  []float64{0, 50, 100},
	}
	cases := []struct {
		in   string
		want float64
	}{{"x", 0}, {"y", 50}, {"z", 100}}
	for _, tc := range cases {
		got, _ := s.Apply(tc.in)
		if got != tc.want {
			t.Errorf("Apply(%q) = %g, want %g", tc.in, got, tc.want)
		}
	}
	r := s.Range()
	if r[0] != 0 || r[1] != 100 {
		t.Errorf("Range() = %v, want [0,100]", r)
	}
}

func TestPrismTimeScaleStub(t *testing.T) {
	values := []any{"2026-01-01", "2026-01-03"}
	scale, warn, err := ResolveScale("temporal", table.KindString, values, 0, 800)
	if err != nil {
		t.Fatalf("ResolveScale: %v", err)
	}
	if warn == nil || warn.Code != scene.WarnTimeScaleStubbed {
		t.Fatalf("expected time-stubbed warning, got %+v", warn)
	}
	if scale.Type() != scene.ScaleTime {
		t.Errorf("Type() = %q, want %q", scale.Type(), scene.ScaleTime)
	}
	// "2026-01-01" maps to RangeMin (the lower domain bound).
	got, err := scale.Apply("2026-01-01")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if math.Abs(got-0) > 1e-6 {
		t.Errorf("Apply(2026-01-01) = %g, want 0", got)
	}
	// "2026-01-03" maps to RangeMax.
	got, _ = scale.Apply("2026-01-03")
	if math.Abs(got-800) > 1e-6 {
		t.Errorf("Apply(2026-01-03) = %g, want 800", got)
	}
}

func TestPrismResolveScaleQuantitativeIncludesZero(t *testing.T) {
	// All-positive numbers should produce a domain starting at 0
	// (matches Vega-Lite's "zero" default for quantitative scales).
	values := []any{int64(40), int64(50), int64(70)}
	scale, _, err := ResolveScale("quantitative", table.KindInt, values, 0, 100)
	if err != nil {
		t.Fatalf("ResolveScale: %v", err)
	}
	dom := scale.Domain()
	if dom[0].(float64) != 0 {
		t.Errorf("domain min = %v, want 0 (zero-include)", dom[0])
	}
	if dom[1].(float64) != 70 {
		t.Errorf("domain max = %v, want 70", dom[1])
	}
}

func TestPrismResolveScaleNominal(t *testing.T) {
	values := []any{"a", "b", "a", "c", "b"}
	scale, _, err := ResolveScale("nominal", table.KindString, values, 0, 300)
	if err != nil {
		t.Fatalf("ResolveScale: %v", err)
	}
	band, ok := scale.(*BandScale)
	if !ok {
		t.Fatalf("expected *BandScale, got %T", scale)
	}
	// De-duped + insertion-ordered.
	want := []string{"a", "b", "c"}
	if len(band.Categories) != 3 {
		t.Fatalf("Categories = %v, want %v", band.Categories, want)
	}
	for i, c := range want {
		if band.Categories[i] != c {
			t.Errorf("Categories[%d] = %q, want %q", i, band.Categories[i], c)
		}
	}
}
