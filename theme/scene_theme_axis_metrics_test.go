package theme

import "testing"

// TestToSceneTheme_AxisMetrics covers E3-S2's ToSceneTheme extension:
// the axis tick_size / label_padding tokens now ride on the Scene IR
// as well as in the CSS-variable manifest, because a CSS variable
// cannot move an SVG line endpoint. The copies must be independent of
// the source theme.
func TestToSceneTheme_AxisMetrics(t *testing.T) {
	tick, pad := 9.0, 10.0
	src := &Theme{Axis: &AxisStyle{TickSize: &tick, LabelPadding: &pad}}
	got := src.ToSceneTheme()

	if got.AxisTickSize == nil || *got.AxisTickSize != 9 {
		t.Fatalf("ToSceneTheme: AxisTickSize = %v, want 9", got.AxisTickSize)
	}
	if got.AxisLabelPadding == nil || *got.AxisLabelPadding != 10 {
		t.Fatalf("ToSceneTheme: AxisLabelPadding = %v, want 10", got.AxisLabelPadding)
	}
	*got.AxisTickSize = 99
	if tick != 9 {
		t.Fatal("ToSceneTheme: AxisTickSize aliases the source theme")
	}
}

// A theme that states no axis geometry leaves the Scene IR silent, so
// the renderer falls through to its built-in metrics.
func TestToSceneTheme_AxisMetricsAbsent(t *testing.T) {
	got := (&Theme{Axis: &AxisStyle{}}).ToSceneTheme()
	if got.AxisTickSize != nil || got.AxisLabelPadding != nil {
		t.Fatalf("ToSceneTheme: unset tokens produced %v / %v, want both nil",
			got.AxisTickSize, got.AxisLabelPadding)
	}
}
