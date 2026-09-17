package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// stubBand is a minimal band scale: Apply returns a slot's LEADING
// edge, which is d3's scaleBand() contract and what the real
// encode/scale.BandScale does.
type stubBand struct {
	cats  []string
	min   float64
	step  float64
	width float64
}

func (s stubBand) Apply(v any) (float64, error) {
	for i, c := range s.cats {
		if c == v.(string) {
			return s.min + float64(i)*s.step, nil
		}
	}
	return 0, nil
}
func (s stubBand) Domain() []any      { return nil }
func (s stubBand) BandWidth() float64 { return s.width }

// stubLinear has no band width, standing in for any continuous scale.
type stubLinear struct{}

func (stubLinear) Apply(v any) (float64, error) { return v.(float64) * 2, nil }
func (stubLinear) Domain() []any                { return nil }

// TestPrismPointPixelCentresOnBand asserts a mark with no extent along
// a band axis lands on the slot MIDPOINT, not its leading edge.
//
// This is the defect PR #50 reported and that shipped for months: a
// four-category line put its vertices a little over half a band left
// of the tick labels naming those categories, so every point sat
// against the wrong label.
func TestPrismPointPixelCentresOnBand(t *testing.T) {
	sc := stubBand{cats: []string{"Q1", "Q2", "Q3", "Q4"}, min: 40, step: 185, width: 185}
	ch := Channel{Field: "q", Scale: sc}
	want := []float64{132.5, 317.5, 502.5, 687.5}
	for i, cat := range sc.cats {
		got, err := PointPixel(ch, cat)
		if err != nil {
			t.Fatalf("PointPixel(%q): %v", cat, err)
		}
		if math.Abs(got-want[i]) > 1e-9 {
			t.Errorf("PointPixel(%q) = %g, want %g (slot midpoint, not leading edge %g)",
				cat, got, want[i], want[i]-sc.width/2)
		}
	}
}

// TestPrismPointPixelHandlesNegativeBandWidth pins the orientation
// half. A y band scale built with an inverted range reports a NEGATIVE
// BandWidth because its slots run bottom-to-top. Half a signed width is
// still the midpoint, so the correction must be applied as an offset
// from Apply rather than recomputed from a slot index — if this ever
// grows an abs() the horizontal-bar case silently lands a full band
// away.
func TestPrismPointPixelHandlesNegativeBandWidth(t *testing.T) {
	sc := stubBand{cats: []string{"A", "B"}, min: 560, step: -260, width: -260}
	ch := Channel{Field: "c", Scale: sc}
	want := []float64{430, 170}
	for i, cat := range sc.cats {
		got, _ := PointPixel(ch, cat)
		if math.Abs(got-want[i]) > 1e-9 {
			t.Errorf("PointPixel(%q) = %g, want %g", cat, got, want[i])
		}
	}
}

// TestPrismPointPixelLeavesContinuousAlone asserts the helper is inert
// on any scale with no band width, which is what keeps every
// continuous-axis golden byte-identical.
func TestPrismPointPixelLeavesContinuousAlone(t *testing.T) {
	ch := Channel{Field: "v", Scale: stubLinear{}}
	got, err := PointPixel(ch, 21.0)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Errorf("PointPixel on a continuous scale = %g, want 42 (plain Apply)", got)
	}
}

// TestPrismStrokeStyleForPaintsALine asserts a sub-mark drawn as a
// <line> ends up with a stroke. A composite hands its children the
// parent's style, which is a FILL; an SVG <line> has no fill area, so
// before this the median, whiskers and caps of every box plot were
// emitted at correct coordinates and drew nothing — including in the
// committed gallery goldens.
func TestPrismStrokeStyleForPaintsALine(t *testing.T) {
	fill, err := scene.ColorFromHex("#4c78a8")
	if err != nil {
		t.Fatal(err)
	}
	got := StrokeStyleFor(scene.Style{Fill: fill})
	if got.Stroke == nil {
		t.Fatal("StrokeStyleFor left Stroke nil — the line would draw nothing")
	}
	if got.Stroke.CSS() != fill.CSS() {
		t.Errorf("Stroke = %s, want the parent's fill %s", got.Stroke.CSS(), fill.CSS())
	}
	if got.StrokeWidth == 0 {
		t.Error("StrokeWidth left at 0 — the line would draw nothing")
	}
}

// TestPrismStrokeStyleForKeepsAnExplicitStroke asserts the helper
// never overrides a stroke the parent already set.
func TestPrismStrokeStyleForKeepsAnExplicitStroke(t *testing.T) {
	fill, _ := scene.ColorFromHex("#4c78a8")
	stroke, _ := scene.ColorFromHex("#ff0000")
	got := StrokeStyleFor(scene.Style{Fill: fill, Stroke: stroke, StrokeWidth: 3})
	if got.Stroke.CSS() != stroke.CSS() {
		t.Errorf("Stroke = %s, want the explicit %s", got.Stroke.CSS(), stroke.CSS())
	}
	if got.StrokeWidth != 3 {
		t.Errorf("StrokeWidth = %g, want the explicit 3", got.StrokeWidth)
	}
}

// TestPrismStrokeStyleForCarriesPaintIndirection asserts a themed
// gradient/pattern ref or a dark-variant CSS variable reaches the
// stroke. Fill is nil in exactly those cases, so copying it alone
// would paint nothing.
func TestPrismStrokeStyleForCarriesPaintIndirection(t *testing.T) {
	got := StrokeStyleFor(scene.Style{FillRef: "prism-gradient-brand"})
	if got.StrokeRef != "prism-gradient-brand" {
		t.Errorf("StrokeRef = %q, want the fill's ref", got.StrokeRef)
	}
	got = StrokeStyleFor(scene.Style{FillVar: "prism-resolved-0"})
	if got.StrokeVar != "prism-resolved-0" {
		t.Errorf("StrokeVar = %q, want the fill's var", got.StrokeVar)
	}
}
