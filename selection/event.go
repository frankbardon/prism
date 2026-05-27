// Package selection defines the canonical structured selection event
// emitted by every Prism rendering context (Go-native, WASM/browser,
// Twirp /prism/scene). The same Event shape travels the wire in every
// environment so a single consumer handler works against any binding.
//
// Mark-type uniformity: every selection — point, interval, lasso — and
// every mark type (bar, line, point, rule + text combos, …) emits the
// same Event struct. The selection kind, populated fields, and extent
// pointers tell consumers what kind of gesture produced the event;
// the structure itself is invariant.
//
// Data-space coordinates are the canonical form. Pixel-space extent is
// optional and exists for UI overlays (brushes, tooltips) — for any
// downstream protocol the data-space extent is what matters because
// it survives re-renders at different sizes.
//
// Stable instance keys: InstanceKey is derived from the (layer_id,
// row_id) pair, not from render-time ordering. Re-rendering the same
// spec with the same data produces identical instance keys.
package selection

// Kind discriminates between the three selection gesture families.
type Kind string

const (
	KindPoint    Kind = "point"
	KindInterval Kind = "interval"
	KindLasso    Kind = "lasso"
)

// Event is the structured selection emission. It is the contract
// between Prism and downstream consumers (UI handlers, Twirp clients,
// MCP agents). JSON shape uses snake_case keys per Prism's wire
// conventions; field ordering in the struct mirrors the upgrade spec.
type Event struct {
	SceneID     string         `json:"scene_id"`
	SelectionID string         `json:"selection_id"`
	Kind        Kind           `json:"kind"`
	Timestamp   int64          `json:"timestamp"`
	Marks       []SelectedMark `json:"marks"`
	DataRows    []DataRowRef   `json:"data_rows"`
	DataExtent  *DataExtent    `json:"data_extent,omitempty"`
	PixelExtent *PixelExtent   `json:"pixel_extent,omitempty"`
	SpecPath    string         `json:"spec_path"`
}

// SelectedMark identifies one selected mark instance. MarkIndex is the
// layer index in the spec (Prism has one mark per layer; for an
// unlayered single-mark spec, MarkIndex is 0). InstanceKey is stable
// across re-renders.
type SelectedMark struct {
	MarkIndex   int    `json:"mark_index"`
	InstanceKey string `json:"instance_key"`
}

// DataRowRef points back into a named dataset. DatasetName is the
// scene-layer Source (the spec's data.source / dataset name).
// RowIndex is the table position; RowID is the dataset's id-column
// value when present.
type DataRowRef struct {
	DatasetName string `json:"dataset_name"`
	RowIndex    int    `json:"row_index"`
	RowID       string `json:"row_id,omitempty"`
}

// DataExtent carries the value-space bounds of an interval / lasso
// selection. For non-numeric (categorical) brushes, X/Y carry the
// stringified domain values via the optional Categories list.
type DataExtent struct {
	X *AxisExtent `json:"x,omitempty"`
	Y *AxisExtent `json:"y,omitempty"`
}

// AxisExtent is one axis's worth of selection bounds. Numeric ranges
// use Min/Max; categorical brushes populate Categories.
type AxisExtent struct {
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

// PixelExtent carries pixel-space bounds of an interval / lasso
// brush. Optional — populated when the renderer has pixel info to
// share; consumers should not assume presence.
type PixelExtent struct {
	X *PixelRange `json:"x,omitempty"`
	Y *PixelRange `json:"y,omitempty"`
}

// PixelRange is one axis's pixel bounds.
type PixelRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// InstanceKey returns the canonical instance-key string for a
// (layer, row) pair. Exposed for callers building events manually.
func InstanceKey(layerID string, rowID int64) string {
	return layerID + ":" + formatInt(rowID)
}
