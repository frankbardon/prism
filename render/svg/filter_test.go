package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// TestRender_ThemeFilters_StructuralElements covers E1-S2: a resolved
// theme carrying AxisFilter/LegendFilter/TitleFilter/ViewFilter must
// apply filter="url(#prism-filter-<name>)" to the corresponding
// structural element (axis group, legend group, title text, view
// background rect), and the theme's Filters registry must emit one
// <filter id="prism-filter-<name>"> def per entry.
func TestRender_ThemeFilters_StructuralElements(t *testing.T) {
	th := &scene.Theme{
		Filters: map[string]string{
			"glow":  `<feGaussianBlur stdDeviation="1"/>`,
			"shade": `<feDropShadow dx="1" dy="1"/>`,
		},
		AxisFilter:   "glow",
		LegendFilter: "shade",
		TitleFilter:  "glow",
		ViewFilter:   "shade",
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
				Title: &scene.TextElement{X: 400, Y: 10, Content: "Title"},
				Axes: []scene.Axis{
					{ID: "x-axis", Channel: scene.ChannelX, Position: scene.AxisPositionBottom},
				},
				Legends: []scene.Legend{
					{ID: "color-legend", Channel: scene.ChannelColor, Entries: []scene.LegendEntry{
						{Label: "a", Swatch: scene.SwatchSpec{Type: scene.SwatchSolid}},
					}},
				},
			}},
		},
	}

	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)

	// One <filter> def per registered Filters entry, sorted by name.
	wantDefs := `<defs><filter id="prism-filter-glow"><feGaussianBlur stdDeviation="1"/></filter><filter id="prism-filter-shade"><feDropShadow dx="1" dy="1"/></filter></defs>`
	if !strings.Contains(s, wantDefs) {
		t.Errorf("output missing sorted filter defs.\nwant substring: %s\ngot:\n%s", wantDefs, s)
	}

	// Title text carries the resolved TitleFilter.
	if !strings.Contains(s, `class="prism-title" x="400" y="10" text-anchor="middle" filter="url(#prism-filter-glow)"`) {
		t.Errorf("title text missing filter attr:\n%s", s)
	}

	// Axes wrapper group carries the resolved AxisFilter.
	if !strings.Contains(s, `class="prism-axes" filter="url(#prism-filter-glow)"`) {
		t.Errorf("axes group missing filter attr:\n%s", s)
	}

	// Legends wrapper group carries the resolved LegendFilter.
	if !strings.Contains(s, `class="prism-legends" filter="url(#prism-filter-shade)"`) {
		t.Errorf("legends group missing filter attr:\n%s", s)
	}

	// View background rect is emitted (only) because ViewFilter is
	// set, sized to the scene Frame, carrying the filter attr.
	if !strings.Contains(s, `class="prism-view" x="0" y="0" width="800" height="600" fill="none" filter="url(#prism-filter-shade)"`) {
		t.Errorf("view background rect missing or malformed:\n%s", s)
	}
}

// TestRender_NoThemeFilters_NoDefsNoViewRect proves the escape hatch
// is fully opt-in: a theme with no Filters registry and no per-block
// Filter references emits no <defs> filter block and no view
// background rect, so existing themes/goldens are unaffected.
func TestRender_NoThemeFilters_NoDefsNoViewRect(t *testing.T) {
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
		t.Errorf("output unexpectedly contains <defs> with no theme filters:\n%s", s)
	}
	if strings.Contains(s, `class="prism-view"`) {
		t.Errorf("output unexpectedly contains a view rect with no ViewFilter:\n%s", s)
	}
}

// TestWriteStyleAttrs_Filter covers the per-mark Style.Filter →
// filter="url(#...)" attr emission directly.
func TestWriteStyleAttrs_Filter(t *testing.T) {
	w := NewWriter()
	writeStyleAttrs(w, scene.Style{Filter: "soft"})
	if got, want := w.String(), ` filter="url(#prism-filter-soft)"`; got != want {
		t.Errorf("writeStyleAttrs = %q, want %q", got, want)
	}

	w2 := NewWriter()
	writeStyleAttrs(w2, scene.Style{})
	if got := w2.String(); got != "" {
		t.Errorf("writeStyleAttrs with no Filter emitted %q, want empty", got)
	}
}
