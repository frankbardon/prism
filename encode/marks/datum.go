package marks

import "github.com/frankbardon/prism/encode/scene"

// AttachDatum stamps a *scene.Datum back-reference on the first
// rowCount marks in the slice. layerID identifies the SceneLayer the
// marks belong to (defaults to "layer-0" for flat specs; composite
// encoders override via Inputs.LayerID).
//
// Per D077, only the (layer_id, row_id) pair is populated by default.
// The Fields bag stays nil to keep the JSON payload small; tooltip
// channels already carry pre-formatted field values via D063.
// Composite encoders that emit aggregation rows (boxplot whiskers,
// histogram bin edges) can call AttachDatum on the appropriate prefix
// and leave the trailing helper marks without a Datum — the JS
// hit-test silently ignores marks without the data-prism-datum-row
// attribute.
//
// rowCount typically equals the number of rows in the upstream table
// (the per-row encoders produce one mark per row). When rowCount is
// larger than len(marks), the helper stops at the end of the marks
// slice — both bounds are respected.
func AttachDatum(marks []scene.Mark, layerID string, rowCount int) {
	if len(marks) == 0 {
		return
	}
	if layerID == "" {
		layerID = "layer-0"
	}
	n := rowCount
	if n > len(marks) {
		n = len(marks)
	}
	for i := 0; i < n; i++ {
		marks[i].Datum = &scene.Datum{
			LayerID: layerID,
			RowID:   int64(i),
		}
	}
}
