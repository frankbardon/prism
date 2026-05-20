package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeEmitsPointSelection(t *testing.T) {
	s, tables, tipID := runPipeline(t, "selection_point.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sels := doc.Grid.Cells[0].Scene.Selections
	if len(sels) != 1 {
		t.Fatalf("Selections len = %d, want 1", len(sels))
	}
	if sels[0].ID != "highlight" {
		t.Errorf("Selections[0].ID = %q, want highlight", sels[0].ID)
	}
	if sels[0].Kind != scene.SelectionPoint {
		t.Errorf("Selections[0].Kind = %q, want point", sels[0].Kind)
	}
	if sels[0].On != scene.EventClick {
		t.Errorf("Selections[0].On = %q, want click", sels[0].On)
	}
	if sels[0].Reactive != scene.ReactiveClient {
		t.Errorf("Selections[0].Reactive = %q, want client", sels[0].Reactive)
	}
}

func TestPrismEncodeEmitsIntervalSelection(t *testing.T) {
	s, tables, tipID := runPipeline(t, "selection_interval.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sels := doc.Grid.Cells[0].Scene.Selections
	if len(sels) != 1 {
		t.Fatalf("Selections len = %d, want 1", len(sels))
	}
	if sels[0].ID != "brush" {
		t.Errorf("Selections[0].ID = %q, want brush", sels[0].ID)
	}
	if sels[0].Kind != scene.SelectionInterval {
		t.Errorf("Selections[0].Kind = %q, want interval", sels[0].Kind)
	}
	if sels[0].On != scene.EventBrush {
		t.Errorf("Selections[0].On = %q, want brush", sels[0].On)
	}
	if len(sels[0].Channels) != 1 || sels[0].Channels[0] != scene.ChannelX {
		t.Errorf("Selections[0].Channels = %v, want [x]", sels[0].Channels)
	}
}

func TestPrismEncodeNoSelectionBlock(t *testing.T) {
	s, tables, tipID := runPipeline(t, "bar_basic.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := doc.Grid.Cells[0].Scene.Selections; got != nil {
		t.Errorf("Selections = %v; want nil (no selection block in spec)", got)
	}
}

func TestPrismEncodeMarkHasDatum(t *testing.T) {
	// T13.02 + T13.03 wire together: scene marks for per-row encoders
	// (bar) carry Datum back-references that hit-testing keys off of.
	s, tables, tipID := runPipeline(t, "selection_point.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	layer := doc.Grid.Cells[0].Scene.Layers[0]
	if len(layer.Marks) == 0 {
		t.Fatalf("layer has no marks")
	}
	for i, m := range layer.Marks {
		if m.Datum == nil {
			t.Fatalf("marks[%d].Datum nil; want populated for selection hit-test", i)
		}
		if int(m.Datum.RowID) != i {
			t.Errorf("marks[%d].Datum.RowID = %d; want %d", i, m.Datum.RowID, i)
		}
		if m.Datum.LayerID != "layer-0" {
			t.Errorf("marks[%d].Datum.LayerID = %q; want layer-0", i, m.Datum.LayerID)
		}
	}
}
