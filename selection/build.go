package selection

import (
	"fmt"
	"strconv"
	"time"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// BuildInput is the raw selection state from a gesture (DOM hit-test,
// brush release, server selection apply). Build resolves it against a
// SceneDoc to produce a structured Event.
type BuildInput struct {
	SceneID     string
	SelectionID string
	Kind        Kind
	Timestamp   int64 // 0 → now (ms since epoch)

	// Point gestures populate Points; interval / lasso gestures
	// populate Range / Categories + optional Pixel*.
	Points []PointHit

	// Range is the value-space brush extent for interval gestures.
	Range *DataExtent

	// PixelExtent is optional UI-overlay info.
	PixelExtent *PixelExtent
}

// PointHit is the DOM-side hit-test result for one mark instance.
type PointHit struct {
	LayerID string
	RowID   int64
}

// Build resolves the raw input against the SceneDoc + Spec to produce
// the canonical Event. doc is the scene the gesture happened on; sp
// is the spec the scene was compiled from (used to map selection IDs
// to their JSON Pointer in the spec). doc must be non-nil; sp may be
// nil — when nil, SpecPath is computed structurally as "/selection/<id>".
func Build(in BuildInput, doc *scene.SceneDoc, sp *spec.Spec) (Event, error) {
	if in.SelectionID == "" {
		return Event{}, fmt.Errorf("selection: BuildInput.SelectionID required")
	}
	if in.Kind == "" {
		return Event{}, fmt.Errorf("selection: BuildInput.Kind required (point|interval|lasso)")
	}
	if doc == nil {
		return Event{}, fmt.Errorf("selection: SceneDoc required")
	}

	ts := in.Timestamp
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	sceneID := in.SceneID
	if sceneID == "" {
		sceneID = primarySceneID(doc)
	}

	ev := Event{
		SceneID:     sceneID,
		SelectionID: in.SelectionID,
		Kind:        in.Kind,
		Timestamp:   ts,
		Marks:       []SelectedMark{},
		DataRows:    []DataRowRef{},
		DataExtent:  in.Range,
		PixelExtent: in.PixelExtent,
		SpecPath:    "/selection/" + in.SelectionID,
	}

	// Resolve point hits → SelectedMark + DataRowRef pairs.
	if len(in.Points) > 0 {
		marks, rows := resolvePoints(in.Points, doc)
		ev.Marks = marks
		ev.DataRows = rows
	}

	// Validate the selection ID exists in the spec when sp != nil.
	// We don't reject when missing — selections can be ephemeral
	// (e.g. server-resolved derived state) — but Spec presence lets
	// us emit the canonical path verbatim.
	_ = sp // currently spec-presence is enough; reserved for future scoped paths.

	return ev, nil
}

// resolvePoints walks the scene doc once, building (layer_index,
// row_index) → SelectedMark entries for each PointHit. Layer index is
// the index of the layer within its enclosing scene; the scene
// containing the hit is the first cell to declare that layer ID.
func resolvePoints(points []PointHit, doc *scene.SceneDoc) ([]SelectedMark, []DataRowRef) {
	marks := make([]SelectedMark, 0, len(points))
	rows := make([]DataRowRef, 0, len(points))

	for _, p := range points {
		layerIdx, source, found := locateLayer(doc, p.LayerID)
		if !found {
			// Unknown layer (stale event after re-render) → still emit a
			// best-effort entry so downstream consumers can decide
			// whether to drop or react.
			marks = append(marks, SelectedMark{
				MarkIndex:   -1,
				InstanceKey: InstanceKey(p.LayerID, p.RowID),
			})
			rows = append(rows, DataRowRef{
				DatasetName: "",
				RowIndex:    int(p.RowID),
			})
			continue
		}
		marks = append(marks, SelectedMark{
			MarkIndex:   layerIdx,
			InstanceKey: InstanceKey(p.LayerID, p.RowID),
		})
		rows = append(rows, DataRowRef{
			DatasetName: source,
			RowIndex:    int(p.RowID),
		})
	}
	return marks, rows
}

// locateLayer returns the layer's index within its enclosing scene
// and its declared Source (dataset name). Returns (-1, "", false) when
// no layer with the given ID exists.
func locateLayer(doc *scene.SceneDoc, layerID string) (int, string, bool) {
	if doc == nil {
		return -1, "", false
	}
	for _, cell := range doc.Grid.Cells {
		for i, layer := range cell.Scene.Layers {
			if layer.ID == layerID {
				return i, layer.Source, true
			}
		}
	}
	return -1, "", false
}

func primarySceneID(doc *scene.SceneDoc) string {
	if doc == nil || len(doc.Grid.Cells) == 0 {
		return ""
	}
	return doc.Grid.Cells[0].Scene.ID
}

func formatInt(v int64) string { return strconv.FormatInt(v, 10) }
