package encode

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func bptr(v bool) *bool { return &v }

func TestPrismScaleOptsLiftsGeometryKnobs(t *testing.T) {
	sc := &spec.Scale{
		Clamp:        bptr(true),
		Reverse:      bptr(true),
		Round:        bptr(true),
		Padding:      fptr(0.2),
		PaddingInner: fptr(0.3),
		PaddingOuter: fptr(0.4),
		Align:        fptr(0.25),
	}
	opts := ScaleOptsFromSpec(sc)
	if !opts.clampEnabled() || !opts.reverseEnabled() || !opts.roundEnabled() {
		t.Fatalf("clamp/reverse/round did not lift: %+v", opts)
	}
	inner, outer, align := opts.bandPadding()
	if inner != 0.3 || outer != 0.4 || align != 0.25 {
		t.Fatalf("bandPadding() = (%g, %g, %g), want (0.3, 0.4, 0.25)", inner, outer, align)
	}
	// The lift detaches from the spec block, so mutating the spec
	// afterwards cannot reach a resolved scale.
	*sc.PaddingInner = 0.9
	if inner, _, _ := opts.bandPadding(); inner != 0.3 {
		t.Fatalf("ScaleOpts aliased the spec block: inner = %g", inner)
	}
}

func TestPrismScalePaddingShorthandPrecedence(t *testing.T) {
	// Nothing declared: the Vega-Lite defaults, which are also the
	// values that keep Prism's historic band layout byte-identical.
	inner, outer, align := ScaleOptsFromSpec(nil).bandPadding()
	if inner != 0.1 || outer != 0.05 || align != 0.5 {
		t.Fatalf("defaults = (%g, %g, %g), want (0.1, 0.05, 0.5)", inner, outer, align)
	}
	// The shorthand sets both band paddings.
	inner, outer, _ = ScaleOptsFromSpec(&spec.Scale{Padding: fptr(0.2)}).bandPadding()
	if inner != 0.2 || outer != 0.2 {
		t.Fatalf("padding shorthand = (%g, %g), want (0.2, 0.2)", inner, outer)
	}
	// An explicit padding_inner outranks it, leaving the shorthand's
	// outer value standing.
	inner, outer, _ = ScaleOptsFromSpec(&spec.Scale{
		Padding:      fptr(0.2),
		PaddingInner: fptr(0.6),
	}).bandPadding()
	if inner != 0.6 || outer != 0.2 {
		t.Fatalf("padding_inner override = (%g, %g), want (0.6, 0.2)", inner, outer)
	}
	// On a point scale the shorthand is the outer padding; there is
	// no inner padding for padding_inner to reach.
	pOuter, _ := ScaleOptsFromSpec(&spec.Scale{
		Padding:      fptr(0.2),
		PaddingInner: fptr(0.6),
	}).pointPadding()
	if pOuter != 0.2 {
		t.Fatalf("point padding = %g, want 0.2", pOuter)
	}
	if def, _ := ScaleOptsFromSpec(nil).pointPadding(); def != 0.5 {
		t.Fatalf("point padding default = %g, want 0.5", def)
	}
}

func TestPrismScaleGeometryPinsOutOfRangeValues(t *testing.T) {
	inner, outer, align := ScaleOptsFromSpec(&spec.Scale{
		PaddingInner: fptr(5),
		PaddingOuter: fptr(-1),
		Align:        fptr(9),
	}).bandPadding()
	if inner >= 1 {
		t.Fatalf("inner padding = %g, want it pinned below 1", inner)
	}
	if outer != 0 || align != 1 {
		t.Fatalf("(outer, align) = (%g, %g), want (0, 1)", outer, align)
	}
}

// TestPrismScaleReverseFlipsContinuousRange documents the axis-relative
// reading of `reverse`: it flips the pixel range the scale maps into,
// so on x the domain minimum moves from left to right, and on y — whose
// range is already handed over bottom-to-top — the minimum moves from
// the bottom to the top.
func TestPrismScaleReverseFlipsContinuousRange(t *testing.T) {
	vals := []any{0.0, 100.0}
	opts := ScaleOptsFromSpec(&spec.Scale{Reverse: bptr(true)})

	// x: range handed over as (left, right).
	sx, _, err := ResolveScaleTyped(scene.ScaleLinear, vals, 40, 760, opts)
	if err != nil {
		t.Fatalf("resolve x: %v", err)
	}
	lo, _ := sx.Apply(0.0)
	hi, _ := sx.Apply(100.0)
	if math.Abs(lo-760) > 1e-9 || math.Abs(hi-40) > 1e-9 {
		t.Fatalf("reversed x: min at %g, max at %g; want 760 and 40", lo, hi)
	}

	// y: range handed over as (bottom, top), so reversing puts the
	// domain minimum at the top.
	sy, _, err := ResolveScaleTyped(scene.ScaleLinear, vals, 560, 20, opts)
	if err != nil {
		t.Fatalf("resolve y: %v", err)
	}
	ylo, _ := sy.Apply(0.0)
	yhi, _ := sy.Apply(100.0)
	if math.Abs(ylo-20) > 1e-9 || math.Abs(yhi-560) > 1e-9 {
		t.Fatalf("reversed y: min at %g, max at %g; want 20 and 560", ylo, yhi)
	}

	// Without reverse the defaults stand.
	plain, _, err := ResolveScaleTyped(scene.ScaleLinear, vals, 560, 20, ScaleOpts{})
	if err != nil {
		t.Fatalf("resolve plain y: %v", err)
	}
	if got, _ := plain.Apply(0.0); math.Abs(got-560) > 1e-9 {
		t.Fatalf("unreversed y min at %g, want 560", got)
	}
}

// TestPrismScaleReverseBandKeepsPositiveWidth is the discrete half of
// the contract: a band scale reverses the slot assignment rather than
// the range, so the band width keeps the sign the range direction
// gives it and inverted-y marks stay drawable.
func TestPrismScaleReverseBandKeepsPositiveWidth(t *testing.T) {
	vals := []any{"a", "b", "c"}
	plain, _, err := ResolveScaleTyped(scene.ScaleBand, vals, 0, 300, ScaleOpts{})
	if err != nil {
		t.Fatalf("resolve band: %v", err)
	}
	rev, _, err := ResolveScaleTyped(scene.ScaleBand, vals, 0, 300,
		ScaleOptsFromSpec(&spec.Scale{Reverse: bptr(true)}))
	if err != nil {
		t.Fatalf("resolve reversed band: %v", err)
	}
	pb, ok := plain.(*scale.BandScale)
	if !ok {
		t.Fatalf("want a BandScale, got %T", plain)
	}
	rb, ok := rev.(*scale.BandScale)
	if !ok {
		t.Fatalf("want a BandScale, got %T", rev)
	}
	if rb.BandWidth() <= 0 || math.Abs(rb.BandWidth()-pb.BandWidth()) > 1e-9 {
		t.Fatalf("reversed BandWidth = %g, want the unreversed %g", rb.BandWidth(), pb.BandWidth())
	}
	first, _ := rb.Apply("a")
	last, _ := pb.Apply("c")
	if math.Abs(first-last) > 1e-9 {
		t.Fatalf("reversed Apply(a) = %g, want the forward slot of c (%g)", first, last)
	}
	// The resolved category order is untouched — only the slots move,
	// so the axis still lists the domain in declaration order.
	if got := rb.Categories; len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("reverse reordered the domain: %v", got)
	}
}

func TestPrismScaleReverseOrdinalFlipsPositions(t *testing.T) {
	vals := []any{"a", "b", "c"}
	rev, _, err := ResolveScaleTyped(scene.ScaleOrdinal, vals, 0, 300,
		ScaleOptsFromSpec(&spec.Scale{Reverse: bptr(true)}))
	if err != nil {
		t.Fatalf("resolve ordinal: %v", err)
	}
	got, _ := rev.Apply("a")
	if math.Abs(got-300) > 1e-9 {
		t.Fatalf("reversed ordinal Apply(a) = %g, want 300", got)
	}
}

// TestPrismScaleRoundReachesEveryFamily checks the knob is threaded,
// not just implemented.
func TestPrismScaleRoundReachesEveryFamily(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{Round: bptr(true)})
	band, _, err := ResolveScaleTyped(scene.ScaleBand, []any{"a", "b", "c"}, 0, 305, opts)
	if err != nil {
		t.Fatalf("band: %v", err)
	}
	if w := band.(*scale.BandScale).BandWidth(); w != math.Trunc(w) {
		t.Fatalf("rounded band width = %g, want a whole pixel", w)
	}
	point, _, err := ResolveScaleTyped(scene.ScalePoint, []any{"a", "b", "c"}, 0, 401, opts)
	if err != nil {
		t.Fatalf("point: %v", err)
	}
	if p, _ := point.Apply("b"); p != math.Trunc(p) {
		t.Fatalf("rounded point position = %g, want a whole pixel", p)
	}
	lin, _, err := ResolveScaleTyped(scene.ScaleLinear, []any{0.0, 3.0}, 0, 100, opts)
	if err != nil {
		t.Fatalf("linear: %v", err)
	}
	if p, _ := lin.Apply(1.0); p != math.Trunc(p) {
		t.Fatalf("rounded linear pixel = %g, want a whole pixel", p)
	}
}

// TestPrismScaleClampReachesEveryContinuousFamily guards the threading
// for clamp the same way.
func TestPrismScaleClampReachesEveryContinuousFamily(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{Clamp: bptr(true), Domain: []any{0.0, 100.0}})
	for _, ty := range []scene.ScaleType{scene.ScaleLinear, scene.ScalePow, scene.ScaleSqrt} {
		s, _, err := ResolveScaleTyped(ty, []any{0.0, 100.0}, 0, 200, opts)
		if err != nil {
			t.Fatalf("%s: %v", ty, err)
		}
		got, err := s.Apply(500.0)
		if err != nil {
			t.Fatalf("%s Apply: %v", ty, err)
		}
		if math.Abs(got-200) > 1e-9 {
			t.Fatalf("%s clamped Apply(500) = %g, want 200", ty, got)
		}
	}
	logOpts := ScaleOptsFromSpec(&spec.Scale{Clamp: bptr(true), Domain: []any{1.0, 100.0}})
	lg, _, err := ResolveScaleTyped(scene.ScaleLog, []any{1.0, 100.0}, 0, 200, logOpts)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if got, _ := lg.Apply(1e6); math.Abs(got-200) > 1e-9 {
		t.Fatalf("log clamped Apply(1e6) = %g, want 200", got)
	}
	tmOpts := ScaleOptsFromSpec(&spec.Scale{Clamp: bptr(true)})
	tm, _, err := ResolveScaleTyped(scene.ScaleTime,
		[]any{"2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z"}, 0, 200, tmOpts)
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	if got, _ := tm.Apply("2030-01-01T00:00:00Z"); math.Abs(got-200) > 1e-9 {
		t.Fatalf("time clamped Apply = %g, want 200", got)
	}
}

// TestPrismSharedScaleHonoursGeometryKnobs pins the composite path:
// the shared-scale resolver must build its band scale through the same
// constructor, or a knob would apply to a flat chart and not a layered
// one.
func TestPrismSharedScaleHonoursGeometryKnobs(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{PaddingInner: fptr(0.5), Reverse: bptr(true)})
	s, err := scaleFromUnified(scene.ScaleBand, []any{"a", "b"}, 0, 200, opts)
	if err != nil {
		t.Fatalf("scaleFromUnified: %v", err)
	}
	b, ok := s.(*scale.BandScale)
	if !ok {
		t.Fatalf("want a BandScale, got %T", s)
	}
	if b.PaddingInner != 0.5 || !b.Reverse {
		t.Fatalf("shared band scale dropped the knobs: %+v", b)
	}
	lin, err := scaleFromUnified(scene.ScaleLinear, []any{0.0, 100.0}, 40, 760,
		ScaleOptsFromSpec(&spec.Scale{Reverse: bptr(true)}))
	if err != nil {
		t.Fatalf("scaleFromUnified linear: %v", err)
	}
	if got, _ := lin.Apply(0.0); math.Abs(got-760) > 1e-9 {
		t.Fatalf("shared reversed linear min at %g, want 760", got)
	}
}

// TestPrismAxisFollowsReversedScale is the "axes/ticks/gridlines
// follow" half of the acceptance: the axis builder places every tick
// through Scale.Apply, so reversing the scale reverses the chrome too.
func TestPrismAxisFollowsReversedScale(t *testing.T) {
	plot := scene.Rect{X: 40, Y: 20, W: 720, H: 540}
	mk := func(rev bool) scene.Axis {
		sc := &spec.Scale{Domain: []any{0.0, 100.0}}
		if rev {
			sc.Reverse = bptr(true)
		}
		s, _, err := ResolveScaleTyped(scene.ScaleLinear, []any{0.0, 100.0}, 40, 760, ScaleOptsFromSpec(sc))
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		return BuildAxis(s, scene.ChannelX, scene.AxisPositionBottom, plot, "v")
	}
	fwd := mk(false)
	rev := mk(true)
	if len(fwd.Ticks) == 0 || len(rev.Ticks) != len(fwd.Ticks) {
		t.Fatalf("tick counts differ: %d vs %d", len(fwd.Ticks), len(rev.Ticks))
	}
	last := len(fwd.Ticks) - 1
	if math.Abs(rev.Ticks[0].Pixel-fwd.Ticks[last].Pixel) > 1e-9 {
		t.Fatalf("first reversed tick at %g, want the forward last tick at %g",
			rev.Ticks[0].Pixel, fwd.Ticks[last].Pixel)
	}
	if len(rev.Grid) != len(fwd.Grid) {
		t.Fatalf("grid line counts differ: %d vs %d", len(rev.Grid), len(fwd.Grid))
	}
	if rev.Scale.Range != [2]float64{760, 40} {
		t.Fatalf("reversed ScaleSpec.Range = %v, want [760 40]", rev.Scale.Range)
	}
}
