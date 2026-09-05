package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// TestRender_ThemeGradientsPatterns_StructuralElements covers E3-S3:
// a resolved theme carrying Gradients/Patterns/ViewBackgroundRef must
// emit one <linearGradient>/<radialGradient>/<pattern> def per
// registry entry (sorted independently within each registry) and
// apply fill="url(#...)" to the view background rect.
func TestRender_ThemeGradientsPatterns_StructuralElements(t *testing.T) {
	white, _ := scene.ColorFromHex("#ffffff")
	black, _ := scene.ColorFromHex("#000000")
	th := &scene.Theme{
		Gradients: map[string]scene.Gradient{
			"fade": {Type: "linear", X1: 0, Y1: 0.5, X2: 1, Y2: 0.5, Stops: []scene.GradientStop{
				{Offset: 0, Color: *black}, {Offset: 1, Color: *white},
			}},
			"glow": {Type: "radial", X1: 0.5, Y1: 0.5, X2: 0.6, Stops: []scene.GradientStop{
				{Offset: 0, Color: *white}, {Offset: 1, Color: *black},
			}},
		},
		Patterns: map[string]scene.Pattern{
			"hatch": {Type: "cross-hatch", Color: "#6b7280", Spacing: 6, Size: 1},
		},
		ViewBackgroundRef: "prism-gradient-fade",
	}
	doc := scene.NewDoc()
	doc.Theme = th
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s1",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
			}},
		},
	}

	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)

	// Gradient/pattern names sorted independently within each
	// registry: "fade" before "glow", then the single pattern.
	wantDefs := `<defs><linearGradient id="prism-gradient-fade" x1="0" y1="0.5" x2="1" y2="0.5"><stop offset="0" stop-color="#000000"/><stop offset="1" stop-color="#ffffff"/></linearGradient><radialGradient id="prism-gradient-glow" cx="0.5" cy="0.5" r="0.6"><stop offset="0" stop-color="#ffffff"/><stop offset="1" stop-color="#000000"/></radialGradient><pattern id="prism-pattern-hatch" patternUnits="userSpaceOnUse" width="6" height="6"><path d="M0,0L6,6M6,0L0,6" fill="none" stroke="#6b7280" stroke-width="1"/></pattern></defs>`
	if !strings.Contains(s, wantDefs) {
		t.Errorf("output missing sorted gradient/pattern defs.\nwant substring: %s\ngot:\n%s", wantDefs, s)
	}

	// View background rect is emitted (only) because ViewBackgroundRef
	// is set, filled with the gradient url ref instead of "none".
	if !strings.Contains(s, `class="prism-view" x="0" y="0" width="800" height="600" fill="url(#prism-gradient-fade)"`) {
		t.Errorf("view background rect missing or malformed:\n%s", s)
	}
}

// TestRender_NoThemeGradientsPatterns_NoDefsNoViewRect proves the
// feature is fully opt-in: a theme with neither registry and no
// ViewBackgroundRef emits no gradient/pattern <defs> block and no
// view background rect, so existing themes/goldens are unaffected.
func TestRender_NoThemeGradientsPatterns_NoDefsNoViewRect(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s1",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
			}},
		},
	}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "<defs>") {
		t.Errorf("output unexpectedly contains <defs> with no theme gradients/patterns:\n%s", s)
	}
	if strings.Contains(s, `class="prism-view"`) {
		t.Errorf("output unexpectedly contains a view rect with no ViewBackgroundRef:\n%s", s)
	}
}

// TestWriteStyleAttrs_FillStrokeRef covers the per-mark
// Style.FillRef/StrokeRef -> fill/stroke="url(#...)" attr emission
// directly, including precedence over a literal Fill/Stroke *Color
// when both happen to be set.
func TestWriteStyleAttrs_FillStrokeRef(t *testing.T) {
	blue, _ := scene.ColorFromHex("#4c78a8")

	w := NewWriter()
	writeStyleAttrs(w, scene.Style{FillRef: "prism-gradient-brand_fade", StrokeRef: "prism-pattern-hatch"})
	want := ` fill="url(#prism-gradient-brand_fade)" stroke="url(#prism-pattern-hatch)"`
	if got := w.String(); got != want {
		t.Errorf("writeStyleAttrs = %q, want %q", got, want)
	}

	// FillRef wins over a literal Fill *Color when both are set (the
	// two are meant to be mutually exclusive per encode.applyThemeMarkStyle,
	// but the renderer's precedence must still be deterministic).
	w2 := NewWriter()
	writeStyleAttrs(w2, scene.Style{Fill: blue, FillRef: "prism-gradient-brand_fade"})
	if got, want := w2.String(), ` fill="url(#prism-gradient-brand_fade)"`; got != want {
		t.Errorf("writeStyleAttrs FillRef precedence = %q, want %q", got, want)
	}

	// No ref set falls back to the literal Fill/Stroke *Color exactly
	// as before this story.
	w3 := NewWriter()
	writeStyleAttrs(w3, scene.Style{Fill: blue})
	if got, want := w3.String(), ` fill="#4c78a8"`; got != want {
		t.Errorf("writeStyleAttrs literal fallback = %q, want %q", got, want)
	}
}
