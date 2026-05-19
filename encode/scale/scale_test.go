package scale

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPointScalePositions(t *testing.T) {
	s := &PointScale{
		Categories: []string{"a", "b", "c"},
		RangeMin:   0,
		RangeMax:   400,
		Padding:    0,
	}
	// step = 400/2 = 200; positions = 0, 200, 400.
	cases := []struct {
		in   string
		want float64
	}{{"a", 0}, {"b", 200}, {"c", 400}}
	for _, tc := range cases {
		got, err := s.Apply(tc.in)
		if err != nil {
			t.Fatalf("Apply(%q): %v", tc.in, err)
		}
		if math.Abs(got-tc.want) > 1e-6 {
			t.Errorf("Apply(%q) = %g, want %g", tc.in, got, tc.want)
		}
	}
	if s.Type() != scene.ScalePoint {
		t.Errorf("Type() = %q, want %q", s.Type(), scene.ScalePoint)
	}
}

func TestPointScalePadding(t *testing.T) {
	s := &PointScale{
		Categories: []string{"a", "b", "c"},
		RangeMin:   0,
		RangeMax:   400,
		Padding:    0.5,
	}
	// step = 400 / (3-1 + 2*0.5) = 400/3 ≈ 133.33
	// positions: padding*step, padding*step + step, padding*step + 2*step
	gotA, _ := s.Apply("a")
	if math.Abs(gotA-66.667) > 0.01 {
		t.Errorf("Apply(a) = %g, want ~66.667", gotA)
	}
	gotC, _ := s.Apply("c")
	if math.Abs(gotC-333.333) > 0.01 {
		t.Errorf("Apply(c) = %g, want ~333.333", gotC)
	}
}

func TestLogScaleApply(t *testing.T) {
	s := &LogScale{Base: 10, DomainMin: 1, DomainMax: 1000, RangeMin: 0, RangeMax: 300}
	cases := []struct {
		in   float64
		want float64
	}{
		{1, 0},
		{10, 100},
		{100, 200},
		{1000, 300},
	}
	for _, tc := range cases {
		got, err := s.Apply(tc.in)
		if err != nil {
			t.Fatalf("Apply(%v): %v", tc.in, err)
		}
		if math.Abs(got-tc.want) > 1e-6 {
			t.Errorf("Apply(%v) = %g, want %g", tc.in, got, tc.want)
		}
	}
	if _, err := s.Apply(0); err == nil {
		t.Error("expected error on Apply(0)")
	}
	if _, err := s.Apply(-1); err == nil {
		t.Error("expected error on Apply(-1)")
	}
}

func TestSqrtScaleApply(t *testing.T) {
	s := &SqrtScale{Inner: PowScale{Exp: 0.5, DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 100}}
	// sqrt(0)=0 -> 0; sqrt(25)=5 -> 50; sqrt(100)=10 -> 100.
	got, _ := s.Apply(25.0)
	if math.Abs(got-50) > 1e-6 {
		t.Errorf("Apply(25) = %g, want 50", got)
	}
}

func TestOrdinalApplyColor(t *testing.T) {
	red, _ := scene.ColorFromHex("#ff0000")
	blue, _ := scene.ColorFromHex("#0000ff")
	s := &OrdinalScale{Categories: []string{"a", "b"}, Positions: []float64{0, 10}}
	if c := s.ApplyColor("a", []*scene.Color{red, blue}); c == nil || c.Hex() != "#ff0000" {
		t.Errorf("ApplyColor(a) = %v, want red", c)
	}
	if c := s.ApplyColor("b", []*scene.Color{red, blue}); c == nil || c.Hex() != "#0000ff" {
		t.Errorf("ApplyColor(b) = %v, want blue", c)
	}
}
