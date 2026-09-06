package theme

import "testing"

func validFillRefTheme() *Theme {
	return &Theme{
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
	}
}

// TestParseFillRefName covers the url(#name) detection helper: the
// well-formed case, and every shape that must fall through to literal
// color handling (empty, hex, keyword, malformed url(...) forms).
func TestParseFillRefName(t *testing.T) {
	cases := []struct {
		value    string
		wantName string
		wantOK   bool
	}{
		{"url(#brand_fade)", "brand_fade", true},
		{"", "", false},
		{"#4c78a8", "", false},
		{"steelblue", "", false},
		{"url(#)", "", false},
		{"url(brand_fade)", "", false},
		{"url(#brand_fade", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			name, ok := ParseFillRefName(tc.value)
			if ok != tc.wantOK || name != tc.wantName {
				t.Fatalf("ParseFillRefName(%q) = (%q, %v), want (%q, %v)", tc.value, name, ok, tc.wantName, tc.wantOK)
			}
		})
	}
}

// TestResolveFillRef_GradientPatternLiteralUnknown covers the four
// AC-required cases: a valid gradient reference, a valid pattern
// reference, an unresolved reference, and a plain literal color (no
// behavior change).
func TestResolveFillRef_GradientPatternLiteralUnknown(t *testing.T) {
	th := validFillRefTheme()

	t.Run("valid gradient reference", func(t *testing.T) {
		ref := th.ResolveFillRef("url(#brand_fade)")
		if ref.Kind != FillRefGradient || ref.Name != "brand_fade" {
			t.Fatalf("ResolveFillRef = %+v, want Kind=FillRefGradient Name=brand_fade", ref)
		}
	})

	t.Run("valid pattern reference", func(t *testing.T) {
		ref := th.ResolveFillRef("url(#hatch)")
		if ref.Kind != FillRefPattern || ref.Name != "hatch" {
			t.Fatalf("ResolveFillRef = %+v, want Kind=FillRefPattern Name=hatch", ref)
		}
	})

	t.Run("unresolved reference", func(t *testing.T) {
		ref := th.ResolveFillRef("url(#nope)")
		if ref.Kind != FillRefUnknown || ref.Name != "nope" {
			t.Fatalf("ResolveFillRef = %+v, want Kind=FillRefUnknown Name=nope", ref)
		}
	})

	t.Run("plain literal color unaffected", func(t *testing.T) {
		ref := th.ResolveFillRef("#4c78a8")
		if ref.Kind != FillRefNone || ref.Name != "" {
			t.Fatalf("ResolveFillRef = %+v, want Kind=FillRefNone Name=\"\"", ref)
		}
	})

	t.Run("gradient wins on name collision", func(t *testing.T) {
		collide := &Theme{
			Gradients: map[string]GradientDef{"dup": th.Gradients["brand_fade"]},
			Patterns:  map[string]PatternDef{"dup": th.Patterns["hatch"]},
		}
		ref := collide.ResolveFillRef("url(#dup)")
		if ref.Kind != FillRefGradient {
			t.Fatalf("ResolveFillRef = %+v, want Kind=FillRefGradient (Gradients checked first)", ref)
		}
	})

	t.Run("nil theme", func(t *testing.T) {
		var nilTheme *Theme
		if ref := nilTheme.ResolveFillRef("#4c78a8"); ref.Kind != FillRefNone {
			t.Fatalf("nil Theme: ResolveFillRef(literal) = %+v, want FillRefNone", ref)
		}
		if ref := nilTheme.ResolveFillRef("url(#anything)"); ref.Kind != FillRefUnknown {
			t.Fatalf("nil Theme: ResolveFillRef(url ref) = %+v, want FillRefUnknown", ref)
		}
	})
}

// TestValidate_FillRef_MarkStyleFillStroke covers Validate's
// cross-reference check on theme.Mark.fill/stroke.
func TestValidate_FillRef_MarkStyleFillStroke(t *testing.T) {
	t.Run("valid gradient fill", func(t *testing.T) {
		th := validFillRefTheme()
		th.Mark = &MarkStyle{Fill: "url(#brand_fade)"}
		if err := th.Validate(); err != nil {
			t.Fatalf("Validate: unexpected error: %v", err)
		}
	})

	t.Run("valid pattern stroke", func(t *testing.T) {
		th := validFillRefTheme()
		th.Mark = &MarkStyle{Stroke: "url(#hatch)"}
		if err := th.Validate(); err != nil {
			t.Fatalf("Validate: unexpected error: %v", err)
		}
	})

	t.Run("unresolved fill fails loud", func(t *testing.T) {
		th := validFillRefTheme()
		th.Mark = &MarkStyle{Fill: "url(#does_not_exist)"}
		err := th.Validate()
		if err == nil {
			t.Fatalf("Validate: expected error, got nil")
		}
		requireCode(t, err, "PRISM_THEME_FILL_REF_UNKNOWN")
	})

	t.Run("unresolved stroke on marks.<type> fails loud", func(t *testing.T) {
		th := validFillRefTheme()
		th.Marks = map[string]*MarkStyle{"bar": {Stroke: "url(#nope)"}}
		err := th.Validate()
		if err == nil {
			t.Fatalf("Validate: expected error, got nil")
		}
		requireCode(t, err, "PRISM_THEME_FILL_REF_UNKNOWN")
	})

	t.Run("unresolved fill on style.<name> fails loud", func(t *testing.T) {
		th := validFillRefTheme()
		th.Style = map[string]*MarkStyle{"emphasis": {Fill: "url(#nope)"}}
		err := th.Validate()
		if err == nil {
			t.Fatalf("Validate: expected error, got nil")
		}
		requireCode(t, err, "PRISM_THEME_FILL_REF_UNKNOWN")
	})

	t.Run("plain literal colors unaffected — zero behavior change", func(t *testing.T) {
		th := validFillRefTheme()
		th.Mark = &MarkStyle{Fill: "#3b82f6", Stroke: "steelblue"}
		th.Marks = map[string]*MarkStyle{"bar": {Fill: "#111827"}}
		th.Style = map[string]*MarkStyle{"emphasis": {Stroke: "#f00"}}
		if err := th.Validate(); err != nil {
			t.Fatalf("Validate: unexpected error for plain literal colors: %v", err)
		}
	})
}

// TestValidate_FillRef_ViewBackground covers Validate's
// cross-reference check on theme.View.background.
func TestValidate_FillRef_ViewBackground(t *testing.T) {
	t.Run("valid gradient background", func(t *testing.T) {
		th := validFillRefTheme()
		th.View = &ViewStyle{Background: "url(#brand_fade)"}
		if err := th.Validate(); err != nil {
			t.Fatalf("Validate: unexpected error: %v", err)
		}
	})

	t.Run("unresolved background fails loud", func(t *testing.T) {
		th := validFillRefTheme()
		th.View = &ViewStyle{Background: "url(#nope)"}
		err := th.Validate()
		if err == nil {
			t.Fatalf("Validate: expected error, got nil")
		}
		requireCode(t, err, "PRISM_THEME_FILL_REF_UNKNOWN")
	})

	t.Run("plain literal background unaffected", func(t *testing.T) {
		th := validFillRefTheme()
		th.View = &ViewStyle{Background: "transparent"}
		if err := th.Validate(); err != nil {
			t.Fatalf("Validate: unexpected error: %v", err)
		}
	})
}

// TestFillRef_DefID covers the E3-S3 def-id naming helper: gradient
// and pattern kinds get their matching "prism-gradient-"/
// "prism-pattern-" prefix, and the no-reference kinds (None, the
// pre-Validate-rejection Unknown case) degrade to "".
func TestFillRef_DefID(t *testing.T) {
	cases := []struct {
		name string
		ref  FillRef
		want string
	}{
		{"gradient", FillRef{Kind: FillRefGradient, Name: "brand_fade"}, "prism-gradient-brand_fade"},
		{"pattern", FillRef{Kind: FillRefPattern, Name: "hatch"}, "prism-pattern-hatch"},
		{"none", FillRef{Kind: FillRefNone}, ""},
		{"unknown", FillRef{Kind: FillRefUnknown, Name: "nope"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ref.DefID(); got != tc.want {
				t.Fatalf("DefID() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRegister_RejectsUnresolvedFillRef mirrors the filter/gradient/
// pattern registration coverage: Register must reject (and not
// mutate the registry for) a theme with a dangling url(#name) fill.
func TestRegister_RejectsUnresolvedFillRef(t *testing.T) {
	bad := &Theme{Mark: &MarkStyle{Fill: "url(#missing)"}}
	err := Register("fillref-test-invalid", bad)
	if err == nil {
		t.Fatalf("Register: expected error, got nil")
	}
	requireCode(t, err, "PRISM_THEME_FILL_REF_UNKNOWN")
	if _, ok := Get("fillref-test-invalid"); ok {
		t.Fatalf("Register: registry mutated despite validation failure")
	}
}
