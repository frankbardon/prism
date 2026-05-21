package svg_test

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
)

// TestPrismRenderEmitsMarkKeyAttr asserts that every per-row mark
// whose scene-IR Key is non-empty round-trips a
// `data-prism-mark-key="<key>"` attribute in the SVG. Drives the
// browser-side animator's mark-matching path.
func TestPrismRenderEmitsMarkKeyAttr(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{
				Row: 0, Col: 0,
				Scene: scene.Scene{
					ID:    "s1",
					Frame: scene.Rect{X: 0, Y: 0, W: 400, H: 300},
					Plot:  scene.Rect{X: 40, Y: 20, W: 340, H: 260},
					Layers: []scene.SceneLayer{{
						ID:   "layer-0",
						Mark: scene.MarkRect,
						Marks: []scene.Mark{
							{Type: scene.MarkRect, Key: "region=west", Rect: &scene.RectGeom{X: 60, Y: 200, W: 40, H: 60}},
							{Type: scene.MarkRect, Key: "region=east", Rect: &scene.RectGeom{X: 120, Y: 150, W: 40, H: 110}},
						},
					}},
				},
			},
		},
	}
	body, err := svg.New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := string(body)
	for _, want := range []string{
		`data-prism-mark-key="region=west"`,
		`data-prism-mark-key="region=east"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

// TestPrismRenderOmitsMarkKeyAttrByDefault asserts that marks with
// no Key string emit no data-prism-mark-key attribute. Regression
// lock for every existing SVG golden.
func TestPrismRenderOmitsMarkKeyAttrByDefault(t *testing.T) {
	got, err := renderFixture(t, "bar_basic.json")
	if err != nil {
		t.Fatalf("renderFixture: %v", err)
	}
	if strings.Contains(string(got), "data-prism-mark-key") {
		t.Errorf("unexpected data-prism-mark-key emitted for non-animated spec:\n%s", truncate(got, 800))
	}
}
