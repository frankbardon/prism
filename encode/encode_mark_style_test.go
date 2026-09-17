package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
)

func fptr(v float64) *float64 { return &v }

// TestPrismApplyMarkDefStyleVocabulary covers the E4-S1 style fields
// applyMarkDef gained: fill_opacity / stroke_opacity (independent of
// opacity), font, font_weight, font_style.
func TestPrismApplyMarkDefStyleVocabulary(t *testing.T) {
	style := scene.Style{}
	applyMarkDef(&spec.MarkDef{
		Type:          "text",
		Opacity:       fptr(0.5),
		FillOpacity:   fptr(0.25),
		StrokeOpacity: fptr(0),
		Font:          "Inter",
		FontWeight:    "bold",
		FontStyle:     "italic",
	}, &style)

	if style.Opacity != 0.5 {
		t.Errorf("Opacity = %v, want 0.5", style.Opacity)
	}
	// fill_opacity and stroke_opacity are independent of opacity —
	// neither overrides it, and an explicit 0 survives.
	if style.FillOpacity == nil || *style.FillOpacity != 0.25 {
		t.Errorf("FillOpacity = %v, want 0.25", style.FillOpacity)
	}
	if style.StrokeOpacity == nil || *style.StrokeOpacity != 0 {
		t.Errorf("StrokeOpacity = %v, want an explicit 0", style.StrokeOpacity)
	}
	if style.FontFamily != "Inter" {
		t.Errorf("FontFamily = %q, want %q", style.FontFamily, "Inter")
	}
	if style.FontWeight != 700 {
		t.Errorf("FontWeight = %d, want 700", style.FontWeight)
	}
	if style.FontStyle != "italic" {
		t.Errorf("FontStyle = %q, want %q", style.FontStyle, "italic")
	}
}

// TestPrismApplyMarkDefLeavesUnsetFieldsAlone asserts a mark def that
// declares none of the new fields does not clobber what the theme
// cascade already wrote — the precondition for spec-over-theme
// precedence being a *per-field* rule rather than a wholesale replace.
func TestPrismApplyMarkDefLeavesUnsetFieldsAlone(t *testing.T) {
	style := scene.Style{
		FillOpacity: fptr(0.4),
		FontFamily:  "Georgia",
		FontWeight:  300,
		FontStyle:   "italic",
	}
	applyMarkDef(&spec.MarkDef{Type: "text"}, &style)

	if style.FillOpacity == nil || *style.FillOpacity != 0.4 {
		t.Errorf("FillOpacity = %v, want the pre-existing 0.4", style.FillOpacity)
	}
	if style.FontFamily != "Georgia" || style.FontWeight != 300 || style.FontStyle != "italic" {
		t.Errorf("typography clobbered: %q/%d/%q", style.FontFamily, style.FontWeight, style.FontStyle)
	}
}

// TestPrismMarkDefBeatsThemeMarkStyle drives the two halves of the
// cascade in the order Encode runs them (theme first, mark def
// second) and asserts the spec value wins on every field both
// structs carry. theme.MarkStyle and spec.MarkDef have identically
// named fields, so this is the test that would catch them being
// crossed.
func TestPrismMarkDefBeatsThemeMarkStyle(t *testing.T) {
	style := scene.Style{}
	ms := &theme.MarkStyle{
		FillOpacity: fptr(0.1),
		FontWeight:  "300",
		FontStyle:   "normal",
	}
	applyThemeMarkStyle(&style, ms, &theme.Theme{}, nil, nil)

	if style.FillOpacity == nil || *style.FillOpacity != 0.1 {
		t.Fatalf("theme FillOpacity not applied: %v", style.FillOpacity)
	}
	if style.FontWeight != 300 {
		t.Fatalf("theme FontWeight = %d, want 300", style.FontWeight)
	}
	if style.FontStyle != "normal" {
		t.Fatalf("theme FontStyle = %q, want %q", style.FontStyle, "normal")
	}

	applyMarkDef(&spec.MarkDef{
		Type:        "text",
		FillOpacity: fptr(0.9),
		FontWeight:  float64(700),
		FontStyle:   "italic",
	}, &style)

	if style.FillOpacity == nil || *style.FillOpacity != 0.9 {
		t.Errorf("FillOpacity = %v, want the spec's 0.9", style.FillOpacity)
	}
	if style.FontWeight != 700 {
		t.Errorf("FontWeight = %d, want the spec's 700", style.FontWeight)
	}
	if style.FontStyle != "italic" {
		t.Errorf("FontStyle = %q, want the spec's %q", style.FontStyle, "italic")
	}
}

func TestPrismNormalizeFontWeight(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int
		ok   bool
	}{
		{"nil", nil, 0, false},
		{"empty string", "", 0, false},
		{"normal", "normal", 400, true},
		{"bold", "bold", 700, true},
		{"lighter", "lighter", 100, true},
		{"bolder", "bolder", 700, true},
		{"numeric string", "500", 500, true},
		{"json number", float64(600), 600, true},
		{"int", 250, 250, true},
		{"zero", float64(0), 0, false},
		{"negative", float64(-100), -100, false},
		{"unknown keyword", "extra-heavy", 0, false},
		{"wrong type", []string{"bold"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeFontWeight(tc.in)
			if ok != tc.ok {
				t.Fatalf("normalizeFontWeight(%v) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Errorf("normalizeFontWeight(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
