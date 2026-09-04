package svg

import (
	"errors"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/render"
)

// TestPrismSVGTableMarkUnsupported guards the E1-S1 decision: a
// top-level `table` mark has no SVG geometry equivalent, so the SVG
// backend must fail loudly with PRISM_RENDER_MARK_UNSUPPORTED rather
// than silently emitting an empty <svg>. The `table` mark type
// itself doesn't exist in spec/encode yet (lands in E1-S2/E1-S3), so
// this test builds the SceneDoc by hand to exercise the general
// backend-capability guard ahead of that mark type landing.
func TestPrismSVGTableMarkUnsupported(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "table-scene",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
				Layers: []scene.SceneLayer{
					{ID: "layer-0", Mark: scene.MarkType("table")},
				},
			}},
		},
	}
	_, err := New().Render(doc, render.RenderOpts{Format: "svg"})
	if err == nil {
		t.Fatal("Render: want error for a top-level table mark, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("Render error is not an *errors.AppError: %v (%T)", err, err)
	}
	if ae.Code != "PRISM_RENDER_MARK_UNSUPPORTED" {
		t.Errorf("Code = %q, want PRISM_RENDER_MARK_UNSUPPORTED", ae.Code)
	}
}

// TestPrismSVGNonTableMarksUnaffected asserts the capability guard
// only rejects marks on the unsupported list — every mark type SVG
// already renders keeps working (regression guard against the map
// growing to reject something it shouldn't).
func TestPrismSVGNonTableMarksUnaffected(t *testing.T) {
	for _, mt := range []scene.MarkType{scene.MarkRect, scene.MarkLine, scene.MarkPoint} {
		doc := scene.NewDoc()
		doc.Grid = scene.SceneGrid{
			Layout: scene.GridLayout{Rows: 1, Cols: 1},
			Cells: []scene.SceneCell{
				{Row: 0, Col: 0, Scene: scene.Scene{
					ID:    "scene",
					Frame: scene.Rect{W: 800, H: 600},
					Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
					Layers: []scene.SceneLayer{
						{ID: "layer-0", Mark: mt},
					},
				}},
			},
		}
		if _, err := New().Render(doc, render.RenderOpts{Format: "svg"}); err != nil {
			t.Errorf("mark %q: unexpected error: %v", mt, err)
		}
	}
}
