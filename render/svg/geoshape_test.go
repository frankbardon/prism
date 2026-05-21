package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

func TestRenderGeoshape_EmitsPath(t *testing.T) {
	doc := newGeoshapeDoc(&scene.PolygonGeom{
		Outer: [][2]float64{{10, 10}, {100, 10}, {100, 100}, {10, 100}},
	})
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `class="prism-mark-geoshape"`) {
		t.Errorf("missing geoshape class:\n%s", got)
	}
	if !strings.Contains(got, `d="M10,10L100,10L100,100L10,100Z"`) {
		t.Errorf("missing or malformed d attribute:\n%s", got)
	}
	if !strings.Contains(got, `fill-rule="evenodd"`) {
		t.Errorf("missing fill-rule:\n%s", got)
	}
}

func TestRenderGeoshape_Holes(t *testing.T) {
	doc := newGeoshapeDoc(&scene.PolygonGeom{
		Outer: [][2]float64{{0, 0}, {100, 0}, {100, 100}, {0, 100}},
		Holes: [][][2]float64{{{40, 40}, {60, 40}, {60, 60}, {40, 60}}},
	})
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := string(out)
	// One sub-path per ring (outer + hole), each ending in Z.
	if strings.Count(got, "Z") < 2 {
		t.Errorf("expected outer + hole sub-paths, got:\n%s", got)
	}
}

func newGeoshapeDoc(g *scene.PolygonGeom) *scene.SceneDoc {
	return &scene.SceneDoc{
		Version: scene.CurrentVersion,
		Grid: scene.SceneGrid{
			Layout: scene.GridLayout{Rows: 1, Cols: 1},
			Cells: []scene.SceneCell{{
				Scene: scene.Scene{
					Frame: scene.Rect{W: 400, H: 200},
					Plot:  scene.Rect{W: 400, H: 200},
					Layers: []scene.SceneLayer{{
						ID:   "layer-0",
						Mark: scene.MarkGeoshape,
						Marks: []scene.Mark{{
							Type:     scene.MarkGeoshape,
							ID:       "geoshape-0-0",
							Geoshape: g,
						}},
					}},
				},
			}},
		},
	}
}
