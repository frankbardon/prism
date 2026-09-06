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

// TestGradientDef_SceneDef covers the E3-S3 angle/center-radius math
// that converts a GradientDef into the scene-level Gradient shape
// render/svg emits <linearGradient>/<radialGradient> defs from.
func TestGradientDef_SceneDef(t *testing.T) {
	t.Run("linear angle 0 defaults to left-to-right vector", func(t *testing.T) {
		g := GradientDef{
			Type:  "linear",
			Stops: []GradientStop{{Offset: 0, Color: "#4c78a8"}, {Offset: 1, Color: "#f58518"}},
		}
		got := g.sceneDef()
		if got.Type != "linear" {
			t.Fatalf("sceneDef: Type = %q, want linear", got.Type)
		}
		// angle 0 => dx=1,dy=0 => x1=0,y1=0.5,x2=1,y2=0.5.
		if !almostEqual(got.X1, 0) || !almostEqual(got.Y1, 0.5) || !almostEqual(got.X2, 1) || !almostEqual(got.Y2, 0.5) {
			t.Fatalf("sceneDef: linear vector = (%v,%v)-(%v,%v), want (0,0.5)-(1,0.5)", got.X1, got.Y1, got.X2, got.Y2)
		}
		if len(got.Stops) != 2 || got.Stops[0].Color.Hex() != "#4c78a8" || got.Stops[1].Color.Hex() != "#f58518" {
			t.Fatalf("sceneDef: Stops = %+v, want the two hex colors preserved in order", got.Stops)
		}
	})

	t.Run("linear angle 90 points top-to-bottom (clockwise)", func(t *testing.T) {
		g := GradientDef{
			Type:  "linear",
			Angle: floatPtr(90),
			Stops: []GradientStop{{Offset: 0, Color: "#000000"}, {Offset: 1, Color: "#ffffff"}},
		}
		got := g.sceneDef()
		if !almostEqual(got.X1, 0.5) || !almostEqual(got.Y1, 0) || !almostEqual(got.X2, 0.5) || !almostEqual(got.Y2, 1) {
			t.Fatalf("sceneDef: linear vector = (%v,%v)-(%v,%v), want (0.5,0)-(0.5,1)", got.X1, got.Y1, got.X2, got.Y2)
		}
	})

	t.Run("radial defaults center/radius to 0.5 when unset", func(t *testing.T) {
		g := GradientDef{
			Type:  "radial",
			Stops: []GradientStop{{Offset: 0, Color: "#000000"}, {Offset: 1, Color: "#ffffff"}},
		}
		got := g.sceneDef()
		if got.Type != "radial" {
			t.Fatalf("sceneDef: Type = %q, want radial", got.Type)
		}
		// Radial packs cx/cy/r into X1/Y1/X2 (see scene.Gradient's doc).
		if !almostEqual(got.X1, 0.5) || !almostEqual(got.Y1, 0.5) || !almostEqual(got.X2, 0.5) {
			t.Fatalf("sceneDef: radial (cx,cy,r) = (%v,%v,%v), want (0.5,0.5,0.5)", got.X1, got.Y1, got.X2)
		}
	})

	t.Run("radial honors explicit cx/cy/radius", func(t *testing.T) {
		g := GradientDef{
			Type:   "radial",
			CX:     floatPtr(0.25),
			CY:     floatPtr(0.75),
			Radius: floatPtr(0.6),
			Stops:  []GradientStop{{Offset: 0, Color: "#000000"}, {Offset: 1, Color: "#ffffff"}},
		}
		got := g.sceneDef()
		if !almostEqual(got.X1, 0.25) || !almostEqual(got.Y1, 0.75) || !almostEqual(got.X2, 0.6) {
			t.Fatalf("sceneDef: radial (cx,cy,r) = (%v,%v,%v), want (0.25,0.75,0.6)", got.X1, got.Y1, got.X2)
		}
	})

	t.Run("unparsable stop color degrades to opaque black rather than dropping the stop", func(t *testing.T) {
		g := GradientDef{
			Type:  "linear",
			Stops: []GradientStop{{Offset: 0, Color: "not-a-hex-color"}, {Offset: 1, Color: "#ffffff"}},
		}
		got := g.sceneDef()
		if len(got.Stops) != 2 {
			t.Fatalf("sceneDef: Stops = %+v, want 2 (malformed stop preserved, not dropped)", got.Stops)
		}
		if got.Stops[0].Color.Hex() != "#000000" {
			t.Fatalf("sceneDef: malformed stop color = %s, want #000000 fallback", got.Stops[0].Color.Hex())
		}
	})
}

// TestPatternDef_SceneDef covers the E3-S3 default-resolution logic
// that converts a PatternDef into the scene-level Pattern shape
// render/svg emits <pattern> defs from.
func TestPatternDef_SceneDef(t *testing.T) {
	t.Run("built-in type defaults spacing/size/color", func(t *testing.T) {
		p := PatternDef{Type: "dots"}
		got := p.sceneDef()
		if got.Type != "dots" {
			t.Fatalf("sceneDef: Type = %q, want dots", got.Type)
		}
		if got.Spacing != defaultPatternSpacing || got.Size != defaultPatternSize || got.Color != defaultPatternColor {
			t.Fatalf("sceneDef: (Spacing,Size,Color) = (%v,%v,%q), want (%v,%v,%q)",
				got.Spacing, got.Size, got.Color, defaultPatternSpacing, defaultPatternSize, defaultPatternColor)
		}
	})

	t.Run("explicit spacing/size/color pass through unchanged", func(t *testing.T) {
		p := PatternDef{Type: "grid", Color: "#e45756", Spacing: floatPtr(10), Size: floatPtr(2)}
		got := p.sceneDef()
		if got.Spacing != 10 || got.Size != 2 || got.Color != "#e45756" {
			t.Fatalf("sceneDef: (Spacing,Size,Color) = (%v,%v,%q), want (10,2,#e45756)", got.Spacing, got.Size, got.Color)
		}
	})

	t.Run("raw-content pattern leaves Color empty when unset", func(t *testing.T) {
		p := PatternDef{Content: "<circle/>"}
		got := p.sceneDef()
		if got.Type != "" {
			t.Fatalf("sceneDef: Type = %q, want empty (raw content)", got.Type)
		}
		if got.Color != "" {
			t.Fatalf("sceneDef: Color = %q, want empty for raw content (author supplies its own colors)", got.Color)
		}
		if got.Content != "<circle/>" {
			t.Fatalf("sceneDef: Content = %q, want verbatim passthrough", got.Content)
		}
		// Spacing/Size still default even for raw content (they size
		// the <pattern> tile itself, independent of Content).
		if got.Spacing != defaultPatternSpacing || got.Size != defaultPatternSize {
			t.Fatalf("sceneDef: (Spacing,Size) = (%v,%v), want defaults even for raw content", got.Spacing, got.Size)
		}
	})
}

func almostEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func floatPtr(v float64) *float64 { return &v }
