package svg

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// unsupportedMarks lists mark types the SVG backend cannot represent
// as geometry. `table` (E1-S2 landed the spec-level mark type; the
// Scene IR node itself lands in E1-S3) is the sole entry: a top-level
// table renders as DOM/CSS-driven markup (sort, paginate, row
// selection) with no meaningful SVG geometry equivalent, so asking
// the SVG backend to render one directly must fail loudly with
// PRISM_RENDER_MARK_UNSUPPORTED rather than silently emitting an
// empty <svg>. Embedding a table's per-cell sub-marks (e.g. a
// sparkline column) is unaffected — those stay ordinary
// geometry-bearing marks and are absent from this set.
//
// scene.MarkCustom (E2) is intentionally absent here: a custom mark
// IS supported directly by the SVG backend — see render/svg/custom.go
// — either by splicing a registered SVGCustomRenderer's fragment
// verbatim, or by <foreignObject>-wrapping an HTMLCustomRenderer's
// fragment as a fallback.
var unsupportedMarks = map[scene.MarkType]bool{
	scene.MarkTable: true,
}

// checkMarkSupport walks every top-level layer in doc.Grid and
// returns PRISM_RENDER_MARK_UNSUPPORTED for the first mark type this
// backend cannot render. Called once at the top of Render so a
// mismatch fails before any bytes are written.
func checkMarkSupport(doc *scene.SceneDoc) error {
	if doc == nil {
		return nil
	}
	for _, cell := range doc.Grid.Cells {
		for _, layer := range cell.Scene.Layers {
			if unsupportedMarks[layer.Mark] {
				return prismerrors.New(
					"PRISM_RENDER_MARK_UNSUPPORTED",
					fmt.Sprintf("Mark type %q has no svg geometry — render this scene via a different backend.", layer.Mark),
					map[string]any{"Mark": string(layer.Mark), "Format": "svg"},
				)
			}
		}
	}
	return nil
}
