package encode

import "testing"

// E3-S2 unit coverage for the two pieces of new pure logic: the
// label_limit truncation heuristic and the per-component side reserve.

func TestPrismTruncateToWidth(t *testing.T) {
	// The heuristic is axisLabelCharWidth (6px) per character.
	for _, tc := range []struct {
		label string
		max   float64
		want  string
	}{
		{"", 12, ""},
		{"abc", 18, "abc"},        // exactly fits, untouched
		{"abcd", 18, "ab…"},       // 3 chars of room, one spent on the ellipsis
		{"abcdefgh", 30, "abcd…"}, // 5 chars of room
		{"abc", 11, ""},           // under two characters' room: unreadable, so dropped
		{"日本語です", 24, "日本語…"},     // runes, never bytes
	} {
		if got := truncateToWidth(tc.label, tc.max); got != tc.want {
			t.Errorf("truncateToWidth(%q, %v) = %q, want %q", tc.label, tc.max, got, tc.want)
		}
	}
}

func TestPrismApplyLabelLimitIgnoresUnsetAndZero(t *testing.T) {
	ticks := BandTicks(&BandScale{
		Categories: []string{"a-very-long-category-name"},
		RangeMin:   0, RangeMax: 100,
	})
	if got := applyLabelLimit(ticks, nil); &got[0] != &ticks[0] {
		t.Error("a nil limit should return the ticks untouched, not a copy")
	}
	zero := 0.0
	if got := applyLabelLimit(ticks, &zero); &got[0] != &ticks[0] {
		t.Error("a zero limit means no limit and should return the ticks untouched")
	}
}

func TestPrismAxisSideReserveReleasesSuppressedComponents(t *testing.T) {
	full := DefaultAxisOpts("score")
	if got := AxisSideReserve(full); got != layoutAxisReserve {
		t.Errorf("default reserve = %v, want the historical %v", got, layoutAxisReserve)
	}

	noLabels := full
	noLabels.Labels = false
	if got := AxisSideReserve(noLabels); got != layoutAxisTickReserve {
		t.Errorf("labels-off reserve = %v, want %v", got, layoutAxisTickReserve)
	}

	noTicks := full
	noTicks.Ticks = false
	if got := AxisSideReserve(noTicks); got != layoutAxisLabelReserve {
		t.Errorf("ticks-off reserve = %v, want %v", got, layoutAxisLabelReserve)
	}

	bare := full
	bare.Labels, bare.Ticks = false, false
	if got := AxisSideReserve(bare); got != 0 {
		t.Errorf("both-off reserve = %v, want 0", got)
	}

	// The domain line rides on the plot edge and costs nothing.
	noDomain := full
	noDomain.Domain = false
	if got := AxisSideReserve(noDomain); got != layoutAxisReserve {
		t.Errorf("domain-off reserve = %v, want the full %v", got, layoutAxisReserve)
	}
}

// A bare AxisPlacement literal — what a caller that knows nothing
// about the channel's axis block writes — must still reserve the full
// depth.
func TestPrismAxisPlacementZeroValueReservesFullDepth(t *testing.T) {
	sides := DefaultAxisPlacement().Sides()
	if sides.Bottom.AxisReserve != layoutAxisReserve {
		t.Errorf("bottom reserve = %v, want %v", sides.Bottom.AxisReserve, layoutAxisReserve)
	}
	if got := (LayoutOpts{Width: 800, Height: 600, Sides: sides}).Padding(); got.Bottom != 40 || got.Left != 40 {
		t.Errorf("default padding = %+v, want Bottom/Left 40", got)
	}
}
