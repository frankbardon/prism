package theme

import "testing"

// TestToSceneTheme_GradientsPatterns covers E3-S3's ToSceneTheme
// extension: the Gradients/Patterns registries copy through
// (pre-resolved via sceneDef, independent maps — no aliasing) and a
// url(#name) View.Background resolves to the matching
// ViewBackgroundRef def id.
func TestToSceneTheme_GradientsPatterns(t *testing.T) {
	src := &Theme{
		Gradients: map[string]GradientDef{
			"brand_fade": {
				Type:  "linear",
				Angle: floatPtr(90),
				Stops: []GradientStop{{Offset: 0, Color: "#4c78a8"}, {Offset: 1, Color: "#f58518"}},
			},
		},
		Patterns: map[string]PatternDef{
			"hatch": {Type: "cross-hatch", Color: "#6b7280"},
		},
		View: &ViewStyle{Background: "url(#brand_fade)"},
	}
	got := src.ToSceneTheme()

	if got.Gradients["brand_fade"].Type != "linear" {
		t.Fatalf("ToSceneTheme: Gradients = %+v, want brand_fade entry copied", got.Gradients)
	}
	if got.Patterns["hatch"].Type != "cross-hatch" {
		t.Fatalf("ToSceneTheme: Patterns = %+v, want hatch entry copied", got.Patterns)
	}
	if got.ViewBackgroundRef != "prism-gradient-brand_fade" {
		t.Fatalf("ToSceneTheme: ViewBackgroundRef = %q, want prism-gradient-brand_fade", got.ViewBackgroundRef)
	}
}

// TestToSceneTheme_GradientsPatterns_NoAliasing mirrors the Filters
// aliasing check: mutating the returned scene-level maps must not
// leak back into the source theme.
func TestToSceneTheme_GradientsPatterns_NoAliasing(t *testing.T) {
	src := &Theme{
		Gradients: map[string]GradientDef{
			"a": {Type: "linear", Stops: []GradientStop{{Offset: 0, Color: "#000000"}, {Offset: 1, Color: "#ffffff"}}},
		},
		Patterns: map[string]PatternDef{
			"p": {Type: "dots", Color: "#4c78a8"},
		},
	}
	got := src.ToSceneTheme()
	delete(got.Gradients, "a")
	delete(got.Patterns, "p")
	if _, ok := src.Gradients["a"]; !ok {
		t.Fatalf("ToSceneTheme: Gradients map aliases the source theme")
	}
	if _, ok := src.Patterns["p"]; !ok {
		t.Fatalf("ToSceneTheme: Patterns map aliases the source theme")
	}
}

// TestToSceneTheme_NoGradientsPatterns covers the zero-value path: a
// theme with neither registry, and a literal (non-url) View.Background,
// produces a scene.Theme with both maps nil and ViewBackgroundRef
// empty — so the renderer's opt-in checks stay false for every
// existing built-in theme.
func TestToSceneTheme_NoGradientsPatterns(t *testing.T) {
	src := &Theme{View: &ViewStyle{Background: "transparent"}}
	got := src.ToSceneTheme()
	if got.Gradients != nil {
		t.Fatalf("ToSceneTheme: Gradients = %+v, want nil", got.Gradients)
	}
	if got.Patterns != nil {
		t.Fatalf("ToSceneTheme: Patterns = %+v, want nil", got.Patterns)
	}
	if got.ViewBackgroundRef != "" {
		t.Fatalf("ToSceneTheme: ViewBackgroundRef = %q, want empty for a literal background", got.ViewBackgroundRef)
	}
}
