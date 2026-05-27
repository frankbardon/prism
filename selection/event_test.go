package selection_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/selection"
)

func TestInstanceKeyStable(t *testing.T) {
	a := selection.InstanceKey("layer-0", 42)
	b := selection.InstanceKey("layer-0", 42)
	if a != b {
		t.Fatalf("InstanceKey not stable: %q vs %q", a, b)
	}
	if a != "layer-0:42" {
		t.Fatalf("InstanceKey format = %q want %q", a, "layer-0:42")
	}
}

func TestBuildPointEventResolvesMarks(t *testing.T) {
	doc := &scene.SceneDoc{
		Version: scene.CurrentVersion,
		Grid: scene.SceneGrid{
			Cells: []scene.SceneCell{{
				Scene: scene.Scene{
					ID: "scene-0",
					Layers: []scene.SceneLayer{
						{ID: "layer-a", Source: "cohort.pulse", Mark: scene.MarkRect},
						{ID: "layer-b", Source: "cohort.pulse", Mark: scene.MarkLine},
					},
				},
			}},
		},
	}

	ev, err := selection.Build(selection.BuildInput{
		SelectionID: "brush",
		Kind:        selection.KindPoint,
		Timestamp:   1700000000000,
		Points: []selection.PointHit{
			{LayerID: "layer-a", RowID: 3},
			{LayerID: "layer-b", RowID: 7},
		},
	}, doc, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if ev.SceneID != "scene-0" {
		t.Errorf("SceneID = %q want scene-0", ev.SceneID)
	}
	if ev.SelectionID != "brush" {
		t.Errorf("SelectionID = %q", ev.SelectionID)
	}
	if ev.Kind != selection.KindPoint {
		t.Errorf("Kind = %q", ev.Kind)
	}
	if ev.SpecPath != "/selection/brush" {
		t.Errorf("SpecPath = %q", ev.SpecPath)
	}
	if len(ev.Marks) != 2 {
		t.Fatalf("Marks len = %d want 2", len(ev.Marks))
	}
	if ev.Marks[0].MarkIndex != 0 || ev.Marks[0].InstanceKey != "layer-a:3" {
		t.Errorf("Marks[0] = %+v", ev.Marks[0])
	}
	if ev.Marks[1].MarkIndex != 1 || ev.Marks[1].InstanceKey != "layer-b:7" {
		t.Errorf("Marks[1] = %+v", ev.Marks[1])
	}
	if len(ev.DataRows) != 2 {
		t.Fatalf("DataRows len = %d want 2", len(ev.DataRows))
	}
	if ev.DataRows[0].DatasetName != "cohort.pulse" || ev.DataRows[0].RowIndex != 3 {
		t.Errorf("DataRows[0] = %+v", ev.DataRows[0])
	}
}

func TestBuildIntervalEventCarriesExtent(t *testing.T) {
	doc := &scene.SceneDoc{
		Version: scene.CurrentVersion,
		Grid: scene.SceneGrid{
			Cells: []scene.SceneCell{{
				Scene: scene.Scene{
					ID:     "scene-1",
					Layers: []scene.SceneLayer{{ID: "layer-x", Source: "main", Mark: scene.MarkLine}},
				},
			}},
		},
	}
	lo := 10.0
	hi := 50.0
	ev, err := selection.Build(selection.BuildInput{
		SelectionID: "range",
		Kind:        selection.KindInterval,
		Range: &selection.DataExtent{
			X: &selection.AxisExtent{Min: &lo, Max: &hi},
		},
		PixelExtent: &selection.PixelExtent{
			X: &selection.PixelRange{Min: 100, Max: 400},
		},
	}, doc, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if ev.DataExtent == nil || ev.DataExtent.X == nil {
		t.Fatalf("DataExtent missing")
	}
	if *ev.DataExtent.X.Min != 10 || *ev.DataExtent.X.Max != 50 {
		t.Errorf("DataExtent.X = %+v", *ev.DataExtent.X)
	}
	if ev.PixelExtent == nil || ev.PixelExtent.X.Min != 100 {
		t.Errorf("PixelExtent = %+v", ev.PixelExtent)
	}
	if len(ev.Marks) != 0 {
		t.Errorf("Marks should be empty for interval-only, got %d", len(ev.Marks))
	}
	if ev.Timestamp == 0 {
		t.Errorf("Timestamp defaulted to 0 — should be now()")
	}
}

func TestBuildRequiresSelectionID(t *testing.T) {
	doc := &scene.SceneDoc{Grid: scene.SceneGrid{Cells: []scene.SceneCell{{Scene: scene.Scene{ID: "s"}}}}}
	_, err := selection.Build(selection.BuildInput{Kind: selection.KindPoint}, doc, nil)
	if err == nil {
		t.Fatal("expected error for missing SelectionID")
	}
}

func TestBuildJSONShape(t *testing.T) {
	doc := &scene.SceneDoc{
		Grid: scene.SceneGrid{Cells: []scene.SceneCell{{
			Scene: scene.Scene{ID: "scene-0", Layers: []scene.SceneLayer{{ID: "L", Source: "ds"}}},
		}}},
	}
	ev, err := selection.Build(selection.BuildInput{
		SceneID:     "scene-0",
		SelectionID: "sel",
		Kind:        selection.KindPoint,
		Timestamp:   1,
		Points:      []selection.PointHit{{LayerID: "L", RowID: 0}},
	}, doc, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(raw)
	for _, key := range []string{
		`"scene_id":"scene-0"`,
		`"selection_id":"sel"`,
		`"kind":"point"`,
		`"timestamp":1`,
		`"marks":`,
		`"data_rows":`,
		`"spec_path":"/selection/sel"`,
	} {
		if !strings.Contains(s, key) {
			t.Errorf("JSON missing %s: %s", key, s)
		}
	}
}

func TestBuildUnknownLayerEmitsBestEffort(t *testing.T) {
	doc := &scene.SceneDoc{Grid: scene.SceneGrid{Cells: []scene.SceneCell{{
		Scene: scene.Scene{ID: "s"},
	}}}}
	ev, err := selection.Build(selection.BuildInput{
		SelectionID: "sel",
		Kind:        selection.KindPoint,
		Points:      []selection.PointHit{{LayerID: "stale", RowID: 99}},
	}, doc, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(ev.Marks) != 1 || ev.Marks[0].MarkIndex != -1 {
		t.Errorf("expected MarkIndex=-1 for stale layer, got %+v", ev.Marks)
	}
}
