package theme

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// TestGradientDef_CloneMerge covers Clone/Merge round-trips for
// Theme.Gradients: aliasing must not leak, and Merge must replace
// entries key-by-key (wholesale per-entry, matching the Filters/
// Schemes precedent) rather than merging fields within an entry.
func TestGradientDef_CloneMerge(t *testing.T) {
	radius := 0.5
	base := &Theme{
		Gradients: map[string]GradientDef{
			"a": {
				Type:  "linear",
				Angle: floatPtr(45),
				Stops: []GradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}},
			},
			"shared": {
				Type:   "radial",
				Radius: &radius,
				Stops:  []GradientStop{{Offset: 0, Color: "#111"}, {Offset: 1, Color: "#222"}},
			},
		},
	}

	clone := base.Clone()
	clone.Gradients["a"] = GradientDef{Type: "radial", Stops: []GradientStop{{Offset: 0, Color: "#f00"}, {Offset: 1, Color: "#0f0"}}}
	if base.Gradients["a"].Type != "linear" {
		t.Fatalf("Theme.Clone: Gradients map aliasing leaked into base: %+v", base.Gradients["a"])
	}
	// Mutating a cloned stop slice must not leak back either.
	cloneShared := clone.Gradients["shared"]
	cloneShared.Stops[0].Color = "#mutated"
	if base.Gradients["shared"].Stops[0].Color != "#111" {
		t.Fatalf("GradientDef.Clone: Stops slice aliasing leaked into base")
	}

	override := &Theme{
		Gradients: map[string]GradientDef{
			"shared": {Type: "linear", Angle: floatPtr(180), Stops: []GradientStop{{Offset: 0, Color: "#a"}, {Offset: 1, Color: "#b"}}},
			"b":      {Type: "linear", Stops: []GradientStop{{Offset: 0, Color: "#c"}, {Offset: 1, Color: "#d"}}},
		},
	}
	merged := Merge(base, override)
	if len(merged.Gradients) != 3 {
		t.Fatalf("Merge: Gradients = %d entries, want 3 (key-by-key union): %+v", len(merged.Gradients), merged.Gradients)
	}
	if merged.Gradients["shared"].Type != "linear" {
		t.Fatalf("Merge: override entry did not win for shared key: %+v", merged.Gradients["shared"])
	}
	if merged.Gradients["a"].Type != "linear" {
		t.Fatalf("Merge: base-only entry not preserved: %+v", merged.Gradients["a"])
	}
	if merged.Gradients["b"].Type != "linear" {
		t.Fatalf("Merge: override-only entry not added: %+v", merged.Gradients["b"])
	}
}

// TestPatternDef_CloneMerge mirrors TestGradientDef_CloneMerge for
// Theme.Patterns.
func TestPatternDef_CloneMerge(t *testing.T) {
	spacing := 4.0
	base := &Theme{
		Patterns: map[string]PatternDef{
			"hatch":   {Type: "cross-hatch", Color: "#111", Spacing: &spacing},
			"bespoke": {Content: "<circle r=\"1\"/>"},
		},
	}

	clone := base.Clone()
	*clone.Patterns["hatch"].Spacing = 99
	if *base.Patterns["hatch"].Spacing != 4 {
		t.Fatalf("PatternDef.Clone: Spacing pointer aliasing leaked into base")
	}

	override := &Theme{
		Patterns: map[string]PatternDef{
			"hatch": {Type: "dots", Color: "#222"},
			"grid1": {Type: "grid"},
		},
	}
	merged := Merge(base, override)
	if len(merged.Patterns) != 3 {
		t.Fatalf("Merge: Patterns = %d entries, want 3: %+v", len(merged.Patterns), merged.Patterns)
	}
	if merged.Patterns["hatch"].Type != "dots" {
		t.Fatalf("Merge: override entry did not win for hatch key: %+v", merged.Patterns["hatch"])
	}
	if merged.Patterns["bespoke"].Content == "" {
		t.Fatalf("Merge: base-only entry not preserved: %+v", merged.Patterns["bespoke"])
	}
}

// TestValidate_Gradients_StructuralChecks covers the structural
// sanity checks in validateGradients: unknown type, too few stops,
// out-of-range offset, empty color.
func TestValidate_Gradients_StructuralChecks(t *testing.T) {
	cases := []struct {
		name string
		g    GradientDef
		ok   bool
	}{
		{"valid linear", GradientDef{Type: "linear", Stops: []GradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}}}, true},
		{"valid radial", GradientDef{Type: "radial", Stops: []GradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}}}, true},
		{"bad type", GradientDef{Type: "conic", Stops: []GradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}}}, false},
		{"too few stops", GradientDef{Type: "linear", Stops: []GradientStop{{Offset: 0, Color: "#000"}}}, false},
		{"no stops", GradientDef{Type: "linear"}, false},
		{"offset out of range", GradientDef{Type: "linear", Stops: []GradientStop{{Offset: -0.1, Color: "#000"}, {Offset: 1, Color: "#fff"}}}, false},
		{"empty color", GradientDef{Type: "linear", Stops: []GradientStop{{Offset: 0, Color: ""}, {Offset: 1, Color: "#fff"}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			th := &Theme{Gradients: map[string]GradientDef{"g": tc.g}}
			err := th.Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate: unexpected error: %v", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatalf("Validate: expected error, got nil")
				}
				requireCode(t, err, "PRISM_THEME_GRADIENT_INVALID")
			}
		})
	}
}

// TestValidate_Patterns_StructuralChecks covers the structural
// sanity checks in validatePatterns: both/neither type+content, an
// unknown catalogue type, and non-positive spacing/size.
func TestValidate_Patterns_StructuralChecks(t *testing.T) {
	cases := []struct {
		name string
		p    PatternDef
		ok   bool
	}{
		{"valid catalogue", PatternDef{Type: "dots"}, true},
		{"valid content", PatternDef{Content: "<circle/>"}, true},
		{"both set", PatternDef{Type: "dots", Content: "<circle/>"}, false},
		{"neither set", PatternDef{}, false},
		{"unknown type", PatternDef{Type: "polka"}, false},
		{"non-positive spacing", PatternDef{Type: "grid", Spacing: floatPtr(0)}, false},
		{"non-positive size", PatternDef{Type: "grid", Size: floatPtr(-1)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			th := &Theme{Patterns: map[string]PatternDef{"p": tc.p}}
			err := th.Validate()
			if tc.ok && err != nil {
				t.Fatalf("Validate: unexpected error: %v", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatalf("Validate: expected error, got nil")
				}
				requireCode(t, err, "PRISM_THEME_PATTERN_INVALID")
			}
		})
	}
}

// TestLoadBytes_GradientsPatterns_Valid exercises the JSON load path
// end-to-end for both new maps.
func TestLoadBytes_GradientsPatterns_Valid(t *testing.T) {
	body := []byte(`{
		"name": "brand",
		"gradients": {
			"fade": {"type": "linear", "angle": 90, "stops": [{"offset": 0, "color": "#000"}, {"offset": 1, "color": "#fff"}]}
		},
		"patterns": {
			"hatch": {"type": "cross-hatch", "color": "#333", "spacing": 4}
		}
	}`)
	got, err := LoadBytes(body)
	if err != nil {
		t.Fatalf("LoadBytes: unexpected error: %v", err)
	}
	if got.Gradients["fade"].Type != "linear" {
		t.Fatalf("Gradients[fade] not populated: %+v", got.Gradients)
	}
	if got.Patterns["hatch"].Type != "cross-hatch" {
		t.Fatalf("Patterns[hatch] not populated: %+v", got.Patterns)
	}
}

// TestLoadBytes_GradientsPatterns_Invalid covers the fail-loud path
// through the JSON loader for both maps.
func TestLoadBytes_GradientsPatterns_Invalid(t *testing.T) {
	t.Run("gradient", func(t *testing.T) {
		body := []byte(`{"gradients": {"bad": {"type": "linear", "stops": [{"offset": 0, "color": "#000"}]}}}`)
		_, err := LoadBytes(body)
		if err == nil {
			t.Fatalf("LoadBytes: expected error, got nil")
		}
		requireCode(t, err, "PRISM_THEME_GRADIENT_INVALID")
	})
	t.Run("pattern", func(t *testing.T) {
		body := []byte(`{"patterns": {"bad": {"type": "not-a-real-type"}}}`)
		_, err := LoadBytes(body)
		if err == nil {
			t.Fatalf("LoadBytes: expected error, got nil")
		}
		requireCode(t, err, "PRISM_THEME_PATTERN_INVALID")
	})
}

// TestRegister_RejectsInvalidGradientsAndPatterns mirrors the filter
// registration coverage for the new maps.
func TestRegister_RejectsInvalidGradientsAndPatterns(t *testing.T) {
	bad := &Theme{Patterns: map[string]PatternDef{"p": {}}}
	err := Register("gradient-pattern-test-invalid", bad)
	if err == nil {
		t.Fatalf("Register: expected error, got nil")
	}
	requireCode(t, err, "PRISM_THEME_PATTERN_INVALID")
	if _, ok := Get("gradient-pattern-test-invalid"); ok {
		t.Fatalf("Register: registry mutated despite validation failure")
	}
}

// TestApplyOverride_GradientsPatterns confirms the spec-level
// theme.gradients / theme.patterns override block reaches
// theme.Theme through ApplyOverride, mirroring the existing Filters
// coverage in override_test.go-adjacent behavior.
func TestApplyOverride_GradientsPatterns(t *testing.T) {
	base := &Theme{}
	override := &spec.ThemeOverride{
		Gradients: map[string]spec.GradientDef{
			"fade": {
				Type:  "radial",
				CX:    floatPtr(0.5),
				CY:    floatPtr(0.5),
				Stops: []spec.GradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}},
			},
		},
		Patterns: map[string]spec.PatternDef{
			"dots1": {Type: "dots", Color: "#4c78a8"},
		},
	}
	got := ApplyOverride(base, override)
	if got.Gradients["fade"].Type != "radial" || got.Gradients["fade"].CX == nil || *got.Gradients["fade"].CX != 0.5 {
		t.Fatalf("ApplyOverride: Gradients not translated: %+v", got.Gradients)
	}
	if got.Patterns["dots1"].Type != "dots" {
		t.Fatalf("ApplyOverride: Patterns not translated: %+v", got.Patterns)
	}
}

func floatPtr(v float64) *float64 { return &v }
