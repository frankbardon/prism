package encode

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// buildCustomSceneDoc builds the full SceneDoc for a custom-mark spec
// (E2). Like a table mark, a custom mark has no cartesian/polar
// geometry — Encode dispatches here before any x/y scale resolution
// or mark-family encoding runs. Unlike a table mark, this node IS
// consumed by the SVG backend directly (render/svg/custom.go) — see
// encode/scene/custom.go's doc comment.
//
// The renderer name travels through untouched: encode has no opinion
// on whether it is registered (that's a render-time concern), and an
// empty/unregistered name still produces a valid SceneDoc — it just
// fails loudly the first time a render backend tries to resolve it.
func buildCustomSceneDoc(
	s *spec.Spec, tbl *table.Table, markDef *spec.MarkDef,
	sceneTheme *scene.Theme, layout Layout, hasTitle bool,
) (*scene.SceneDoc, error) {
	rendererName := ""
	if markDef != nil {
		rendererName = markDef.Renderer
	}

	// Layers still carries one entry so render/svg's per-layer walk
	// (and any future non-svg backend) recognises this cell as a
	// custom mark via layer.Mark even though Marks stays empty — a
	// custom mark has no positioned geometry of its own.
	layer := scene.SceneLayer{ID: "layer-0", Mark: scene.MarkCustom}

	custom := &scene.Custom{
		Renderer: rendererName,
		Box:      scene.Box{W: layout.Plot.W, H: layout.Plot.H},
		Rows:     tbl.Rows(),
	}

	sceneObj := scene.Scene{
		ID:         "scene-0",
		Frame:      layout.Frame,
		Plot:       layout.Plot,
		Layers:     []scene.SceneLayer{layer},
		Custom:     custom,
		Selections: BuildSelections(s.Selection),
		Animation:  animationFromSpec(s),
	}
	if hasTitle {
		sceneObj.Title = &scene.TextElement{
			Content: titleText(s),
			X:       layout.Plot.CenterX(),
			Y:       20,
		}
	}
	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: sceneObj},
		},
	}
	return doc, nil
}
