package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismDatumAttachPerRow(t *testing.T) {
	marks := []scene.Mark{
		{Type: scene.MarkRect, Rect: &scene.RectGeom{X: 0, W: 10}},
		{Type: scene.MarkRect, Rect: &scene.RectGeom{X: 10, W: 10}},
		{Type: scene.MarkRect, Rect: &scene.RectGeom{X: 20, W: 10}},
	}
	AttachDatum(marks, "layer-0", 3)

	for i, m := range marks {
		if m.Datum == nil {
			t.Fatalf("marks[%d].Datum == nil; want populated", i)
		}
		if m.Datum.LayerID != "layer-0" {
			t.Errorf("marks[%d].Datum.LayerID = %q; want layer-0", i, m.Datum.LayerID)
		}
		if int(m.Datum.RowID) != i {
			t.Errorf("marks[%d].Datum.RowID = %d; want %d", i, m.Datum.RowID, i)
		}
		if m.Datum.Fields != nil {
			t.Errorf("marks[%d].Datum.Fields = %v; want nil (D077 keeps fields out by default)", i, m.Datum.Fields)
		}
	}
}

func TestPrismDatumAttachDefaultLayerID(t *testing.T) {
	marks := []scene.Mark{{Type: scene.MarkRect, Rect: &scene.RectGeom{}}}
	AttachDatum(marks, "", 1)
	if marks[0].Datum == nil || marks[0].Datum.LayerID != "layer-0" {
		t.Fatalf("default LayerID = %q; want layer-0", marks[0].Datum.LayerID)
	}
}

func TestPrismDatumAttachLayerIDOverride(t *testing.T) {
	marks := []scene.Mark{{Type: scene.MarkRect, Rect: &scene.RectGeom{}}}
	AttachDatum(marks, "custom-layer", 1)
	if marks[0].Datum == nil || marks[0].Datum.LayerID != "custom-layer" {
		t.Fatalf("custom LayerID = %q; want custom-layer", marks[0].Datum.LayerID)
	}
}

func TestPrismDatumAttachRowCountBeyondMarks(t *testing.T) {
	// Composite encoders may report a row count > len(marks) (e.g.
	// after filtering). AttachDatum must cap at len(marks) — no panic.
	marks := []scene.Mark{{Type: scene.MarkRect, Rect: &scene.RectGeom{}}}
	AttachDatum(marks, "layer-0", 99)
	if marks[0].Datum == nil || marks[0].Datum.RowID != 0 {
		t.Fatalf("rowCount > len(marks) did not bound to len(marks)")
	}
}

func TestPrismDatumAttachRowCountBelowMarks(t *testing.T) {
	// Composite encoders that emit one mark per row + trailing
	// helper marks (boxplot whiskers etc.) — only the leading
	// rowCount marks gain Datum.
	marks := []scene.Mark{
		{Type: scene.MarkRect, Rect: &scene.RectGeom{}},
		{Type: scene.MarkRect, Rect: &scene.RectGeom{}},
		{Type: scene.MarkRect, Rect: &scene.RectGeom{}},
	}
	AttachDatum(marks, "layer-0", 2)
	if marks[0].Datum == nil || marks[1].Datum == nil {
		t.Fatalf("first two marks should have Datum")
	}
	if marks[2].Datum != nil {
		t.Fatalf("trailing helper mark should have Datum=nil; got %+v", marks[2].Datum)
	}
}

func TestPrismDatumAttachEmptyMarks(t *testing.T) {
	// No panic on empty input.
	AttachDatum(nil, "layer-0", 5)
	AttachDatum([]scene.Mark{}, "layer-0", 5)
}

func TestPrismDatumIntegrationWithEncode(t *testing.T) {
	// Drive the full Encode dispatch + assert Datum lands on the
	// produced per-row marks. Bar mark is the canonical per-row
	// encoder.
	tbl := buildTable(t, map[string]any{
		"brand": []string{"alpha", "beta", "gamma"},
		"score": []float64{0.4, 0.7, 0.5},
	})
	in := Inputs{
		Table: tbl,
		X: Channel{
			Field: "brand",
			Scale: &bandScaleT{cats: []string{"alpha", "beta", "gamma"}, rmin: 40, rmax: 780, padding: 0.1},
		},
		Y: Channel{
			Field: "score",
			Scale: &linScale{dmin: 0, dmax: 1, rmin: 560, rmax: 20},
		},
		Layout: plotRect(),
	}
	marks, _, err := Encode("bar", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 3 {
		t.Fatalf("expected 3 marks, got %d", len(marks))
	}
	for i, m := range marks {
		if m.Datum == nil {
			t.Fatalf("marks[%d] missing Datum after Encode dispatch", i)
		}
		if m.Datum.LayerID != "layer-0" {
			t.Errorf("marks[%d].Datum.LayerID = %q; want layer-0 (default)", i, m.Datum.LayerID)
		}
		if int(m.Datum.RowID) != i {
			t.Errorf("marks[%d].Datum.RowID = %d; want %d", i, m.Datum.RowID, i)
		}
	}
}

func TestPrismDatumIntegrationWithLayerIDOverride(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"brand": []string{"a", "b"},
		"score": []float64{0.5, 0.6},
	})
	in := Inputs{
		Table:   tbl,
		LayerID: "facet-cell-2-1",
		X: Channel{
			Field: "brand",
			Scale: &bandScaleT{cats: []string{"a", "b"}, rmin: 0, rmax: 100},
		},
		Y: Channel{
			Field: "score",
			Scale: &linScale{dmin: 0, dmax: 1, rmin: 100, rmax: 0},
		},
		Layout: plotRect(),
	}
	marks, _, err := Encode("bar", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i, m := range marks {
		if m.Datum == nil || m.Datum.LayerID != "facet-cell-2-1" {
			t.Errorf("marks[%d].Datum.LayerID = %v; want facet-cell-2-1", i, m.Datum)
		}
	}
}
