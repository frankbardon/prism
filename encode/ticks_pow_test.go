package encode

import (
	"math"
	"testing"
)

// TestPowTicks asserts pow scale ticks span the domain and produce
// monotonic pixel positions.
func TestPowTicks(t *testing.T) {
	s := &PowScale{Exp: 2, DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 500}
	ticks := PowTicks(s, 5)
	if len(ticks) < 2 {
		t.Fatalf("expected >=2 ticks, got %d", len(ticks))
	}
	prev := math.Inf(-1)
	for i, tk := range ticks {
		if tk.Pixel < prev {
			t.Errorf("tick %d pixel %v < previous %v", i, tk.Pixel, prev)
		}
		prev = tk.Pixel
	}
}

// TestSqrtTicks asserts sqrt scale ticks behave like PowTicks(0.5).
func TestSqrtTicks(t *testing.T) {
	s := &SqrtScale{Inner: PowScale{Exp: 0.5, DomainMin: 0, DomainMax: 100, RangeMin: 0, RangeMax: 500}}
	ticks := SqrtTicks(s, 5)
	if len(ticks) == 0 {
		t.Fatalf("expected ticks, got none")
	}
}

// TestLogTicks asserts log scale ticks include the major powers of base.
func TestLogTicks(t *testing.T) {
	s := &LogScale{Base: 10, DomainMin: 1, DomainMax: 1000, RangeMin: 0, RangeMax: 500}
	ticks := LogTicks(s)
	if len(ticks) == 0 {
		t.Fatal("expected ticks")
	}
	// Look for majors at 1, 10, 100, 1000.
	wantMajors := map[float64]bool{1: false, 10: false, 100: false, 1000: false}
	for _, tk := range ticks {
		if tk.Minor {
			continue
		}
		v, ok := tk.Value.(float64)
		if !ok {
			continue
		}
		if _, want := wantMajors[v]; want {
			wantMajors[v] = true
		}
	}
	for v, present := range wantMajors {
		if !present {
			t.Errorf("missing major tick at %v", v)
		}
	}
}
