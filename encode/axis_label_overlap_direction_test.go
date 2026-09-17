package encode

import (
	"fmt"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// Direction-independence of the label-overlap pass.
//
// Ticks arrive in DOMAIN order, not pixel order. A band scale on a
// vertical axis runs bottom-to-top, so category 0 carries the largest
// y and successive pixels DESCEND. The original scan compared signed
// extents (`start < lastEnd`), which reads every adjacent pair on a
// left/right axis as overlapping no matter how far apart they are, so
// parity mode hid every other label. Continuous vertical axes were hit
// too — the committed goldens carried y axes labelled 0/0.4/0.8 where
// all five ticks had 127px of clear room.
//
// These tests pin the invariant that fixes it: the SAME tick positions
// mirrored onto a descending axis must hide the SAME number of labels.

// mirrorTicks reverses a tick slice's pixel positions about `span`,
// producing the descending sequence a vertical band scale emits while
// keeping domain order intact.
func mirrorTicks(in []scene.Tick, span float64) []scene.Tick {
	out := make([]scene.Tick, len(in))
	for i, t := range in {
		t.Pixel = span - t.Pixel
		out[i] = t
	}
	return out
}

func countHidden(ticks []scene.Tick) int {
	n := 0
	for _, t := range ticks {
		if t.LabelHidden {
			n++
		}
	}
	return n
}

// TestPrismLabelOverlapIsDirectionIndependent asserts an axis whose
// pixels descend thins exactly as one whose pixels ascend, for both
// modes and across category counts. This is the assertion that would
// have caught the v0.15.0 horizontal-bar label defect; neither
// orientation alone catches it, because the ascending side was always
// correct.
func TestPrismLabelOverlapIsDirectionIndependent(t *testing.T) {
	const span = 600.0
	for _, mode := range []string{overlapParity, overlapGreedy} {
		for n := 2; n <= 12; n++ {
			t.Run(fmt.Sprintf("%s/n=%d", mode, n), func(t *testing.T) {
				// Evenly spaced across the span, labels short enough
				// that the horizontal case is uncrowded too.
				step := span / float64(n)
				asc := make([]scene.Tick, n)
				for i := range asc {
					asc[i] = scene.Tick{
						Value: float64(i),
						Pixel: float64(i) * step,
						Label: fmt.Sprintf("C%d", i),
					}
				}
				desc := mirrorTicks(asc, span)

				up := countHidden(applyLabelOverlap(asc, mode, scene.AxisPositionLeft))
				down := countHidden(applyLabelOverlap(desc, mode, scene.AxisPositionLeft))
				if up != down {
					t.Errorf("mode=%s n=%d: ascending hid %d labels, descending hid %d — the pass is reading pixel direction",
						mode, n, up, down)
				}
			})
		}
	}
}

// TestPrismVerticalBandLabelsAllShownWhenRoomy asserts the concrete
// regression: a vertical axis with far more room than it needs hides
// nothing. At n=8 over 600px the bands sit 67.5px apart against a
// 12px LabelLineHeight — overlap is impossible by 5.6x, yet the old
// scan hid four of the eight.
func TestPrismVerticalBandLabelsAllShownWhenRoomy(t *testing.T) {
	const span = 600.0
	for n := 2; n <= 8; n++ {
		step := span / float64(n)
		if step <= scene.LabelLineHeight {
			t.Fatalf("n=%d: test fixture is genuinely crowded (step %g <= lineH %g)", n, step, scene.LabelLineHeight)
		}
		// Descending, as a band scale on y emits.
		ticks := make([]scene.Tick, n)
		for i := range ticks {
			ticks[i] = scene.Tick{
				Value: float64(i),
				Pixel: span - float64(i)*step,
				Label: fmt.Sprintf("Brand%d", i),
			}
		}
		for _, pos := range []scene.AxisPosition{scene.AxisPositionLeft, scene.AxisPositionRight} {
			if got := countHidden(applyLabelOverlap(ticks, overlapParity, pos)); got != 0 {
				t.Errorf("n=%d pos=%v: hid %d of %d labels with %gpx of clear room each",
					n, pos, got, n, step)
			}
		}
	}
}

// TestPrismLabelOverlapStillThinsWhenCrowded guards the other
// direction: the fix must not disable the pass. A genuinely crowded
// vertical axis still thins, and greedy drops at least as much as
// parity.
func TestPrismLabelOverlapStillThinsWhenCrowded(t *testing.T) {
	// 40 ticks over 200px → 5px apart, well under the 12px line height.
	const n = 40
	ticks := make([]scene.Tick, n)
	for i := range ticks {
		ticks[i] = scene.Tick{
			Value: float64(i),
			Pixel: 200 - float64(i)*5,
			Label: fmt.Sprintf("C%d", i),
		}
	}
	parity := countHidden(applyLabelOverlap(ticks, overlapParity, scene.AxisPositionLeft))
	greedy := countHidden(applyLabelOverlap(ticks, overlapGreedy, scene.AxisPositionLeft))
	if parity == 0 {
		t.Error("parity hid nothing on a crowded axis — the pass is disabled")
	}
	if greedy < parity {
		t.Errorf("greedy hid %d, parity hid %d — greedy must drop at least as much", greedy, parity)
	}
}

// TestPrismOverlapModeNormalisesEverySchemaSpelling asserts every
// spelling schema/v1/axis.schema.json accepts lands on a mode
// applyLabelOverlap actually acts on. "greedy" shipped in that enum
// while the code only ever compared against "parity", so asking for it
// silently did nothing — the same silent-no-op class as the
// `sort: "descending"` bug.
func TestPrismOverlapModeNormalisesEverySchemaSpelling(t *testing.T) {
	known := map[string]bool{overlapParity: true, overlapGreedy: true, overlapNone: true}
	cases := []any{true, false, "parity", "greedy", "none", "auto"}
	for _, c := range cases {
		got, ok := overlapMode(c)
		if !ok {
			t.Errorf("overlapMode(%#v) returned ok=false", c)
			continue
		}
		if !known[got] {
			t.Errorf("overlapMode(%#v) = %q, which applyLabelOverlap does not act on", c, got)
		}
	}
	// A bare unknown string degrades to the default rather than
	// silently disabling overlap handling.
	if got, _ := overlapMode("nonsense"); got != overlapParity {
		t.Errorf("overlapMode(unknown) = %q, want %q", got, overlapParity)
	}
}
