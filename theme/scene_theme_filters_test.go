package theme

import "testing"

// TestToSceneTheme_Filters covers E1-S2's ToSceneTheme extension: the
// Filters registry copies through (independent map, no aliasing) and
// each structural block's resolved Filter reference lands on the
// matching scene.Theme field.
func TestToSceneTheme_Filters(t *testing.T) {
	src := &Theme{
		Filters: map[string]string{"glow": "<feGaussianBlur/>"},
		Axis:    &AxisStyle{Filter: "glow"},
		Legend:  &LegendStyle{Filter: "glow"},
		Title:   &TitleStyle{Filter: "glow"},
		View:    &ViewStyle{Filter: "glow"},
	}
	got := src.ToSceneTheme()

	if got.Filters["glow"] != "<feGaussianBlur/>" {
		t.Fatalf("ToSceneTheme: Filters = %+v, want glow entry copied", got.Filters)
	}
	got.Filters["glow"] = "mutated"
	if src.Filters["glow"] != "<feGaussianBlur/>" {
		t.Fatalf("ToSceneTheme: Filters map aliases the source theme")
	}
	if got.AxisFilter != "glow" || got.LegendFilter != "glow" ||
		got.TitleFilter != "glow" || got.ViewFilter != "glow" {
		t.Fatalf("ToSceneTheme: block filter fields = %+v, want all \"glow\"", got)
	}
}

// TestToSceneTheme_NoFilters covers the zero-value path: a theme with
// no Filters registry and no block-level Filter references produces
// a scene.Theme with all filter fields at their zero value, so the
// renderer's opt-in checks (len(Filters) == 0, "" filter names) stay
// false for every existing built-in theme.
func TestToSceneTheme_NoFilters(t *testing.T) {
	src := &Theme{
		Axis:   &AxisStyle{},
		Legend: &LegendStyle{},
		Title:  &TitleStyle{},
		View:   &ViewStyle{},
	}
	got := src.ToSceneTheme()
	if got.Filters != nil {
		t.Fatalf("ToSceneTheme: Filters = %+v, want nil", got.Filters)
	}
	if got.AxisFilter != "" || got.LegendFilter != "" || got.TitleFilter != "" || got.ViewFilter != "" {
		t.Fatalf("ToSceneTheme: block filter fields = %+v, want all empty", got)
	}
}
