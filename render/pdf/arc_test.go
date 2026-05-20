package pdf

import (
	"math"
	"testing"
)

// TestPrismPDFArcQuarterCircle — sweep of pi/2 yields exactly one
// segment with kappa-spaced control points (within float precision).
func TestPrismPDFArcQuarterCircle(t *testing.T) {
	segs := arcToBeziers(0, 0, 10, 10, 0, math.Pi/2, 0)
	if len(segs) != 1 {
		t.Fatalf("segment count = %d, want 1 for quarter circle", len(segs))
	}
	s := segs[0]
	// Start at (10, 0), end near (0, 10).
	if math.Abs(s.X0-10) > 1e-9 || math.Abs(s.Y0-0) > 1e-9 {
		t.Errorf("start = (%g, %g), want (10, 0)", s.X0, s.Y0)
	}
	if math.Abs(s.X1-0) > 1e-9 || math.Abs(s.Y1-10) > 1e-9 {
		t.Errorf("end = (%g, %g), want (0, 10)", s.X1, s.Y1)
	}
}

// TestPrismPDFArcFullCircle — 2pi sweep yields exactly four
// segments (four quarters).
func TestPrismPDFArcFullCircle(t *testing.T) {
	segs := arcToBeziers(0, 0, 5, 5, 0, 2*math.Pi, 0)
	if len(segs) != 4 {
		t.Fatalf("segment count = %d, want 4 for full circle", len(segs))
	}
}

// TestPrismPDFArcSemiCircle — pi sweep yields exactly two segments.
func TestPrismPDFArcSemiCircle(t *testing.T) {
	segs := arcToBeziers(0, 0, 5, 5, 0, math.Pi, 0)
	if len(segs) != 2 {
		t.Fatalf("segment count = %d, want 2 for semi-circle", len(segs))
	}
}

// TestPrismPDFArcDegenerateNoSegments — zero-radius / zero-sweep
// returns nil.
func TestPrismPDFArcDegenerateNoSegments(t *testing.T) {
	if segs := arcToBeziers(0, 0, 0, 5, 0, math.Pi, 0); segs != nil {
		t.Errorf("zero rx yielded %d segments, want nil", len(segs))
	}
	if segs := arcToBeziers(0, 0, 5, 5, 0, 0, 0); segs != nil {
		t.Errorf("zero sweep yielded %d segments, want nil", len(segs))
	}
}

// TestPrismPDFEndpointToCenterIdentity — a small known case round-
// trips: from (10, 0) to (0, 10) with rx=ry=10 along the unit
// rotation = a quarter arc centered at origin.
func TestPrismPDFEndpointToCenterIdentity(t *testing.T) {
	cx, cy, theta1, deltaTheta := endpointToCenter(10, 0, 10, 10, 0, false, true, 0, 10)
	// Center near origin; theta1 ≈ 0; deltaTheta ≈ pi/2 (sweep=true,
	// large=false means CCW short arc).
	if math.Abs(cx) > 0.01 || math.Abs(cy) > 0.01 {
		t.Errorf("center = (%g, %g), want ~(0, 0)", cx, cy)
	}
	if math.Abs(theta1) > 0.01 {
		t.Errorf("theta1 = %g, want ~0", theta1)
	}
	if math.Abs(deltaTheta-math.Pi/2) > 0.01 {
		t.Errorf("deltaTheta = %g, want ~pi/2", deltaTheta)
	}
}
