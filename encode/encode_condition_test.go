package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// TestEncodeNoConditionLocksGoldens — encoding a spec without any
// condition block leaves Mark.Conditions nil so existing SVG / PDF
// goldens stay byte-identical.
func TestEncodeNoConditionLocksGoldens(t *testing.T) {
	s, tables, tipID := runPipeline(t, "bar_basic.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for li, layer := range doc.Grid.Cells[0].Scene.Layers {
		for mi, m := range layer.Marks {
			if m.Conditions != nil {
				t.Errorf("layer[%d].mark[%d].Conditions = %+v; want nil", li, mi, m.Conditions)
			}
		}
	}
}

// TestEncodeConditionStaticTestBakesStyle — a `test`-driven condition
// that matches a row's data is evaluated at encode time and baked into
// the row's Style.Fill, so the SVG renderer paints it without any
// browser involvement.
func TestEncodeConditionStaticTestBakesStyle(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {
			"name": "scores",
			"values": [
				{"region": "west", "score": 0.42},
				{"region": "east", "score": 0.91}
			]
		},
		"mark": "bar",
		"encoding": {
			"x": {"field": "region", "type": "nominal"},
			"y": {"field": "score",  "type": "quantitative"},
			"color": {
				"condition": [
					{"test": "score >= 0.7", "value": "#22c55e"}
				],
				"value": "#cbd5e1"
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	tables, tipID := buildAndExecute(t, s)
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	marks := doc.Grid.Cells[0].Scene.Layers[0].Marks
	if len(marks) != 2 {
		t.Fatalf("want 2 marks, got %d", len(marks))
	}
	for _, m := range marks {
		if len(m.Conditions) != 0 {
			t.Errorf("static condition should not leak to Conditions: %+v", m.Conditions)
		}
	}
	// Row 0 is "west" (score 0.42 → fails the test); row 1 is "east"
	// (score 0.91 → passes). Per-row marks ship in table order.
	west := findMarkByRow(t, marks, 0)
	east := findMarkByRow(t, marks, 1)
	if east.Style.Fill == nil || east.Style.Fill.Hex() != "#22c55e" {
		t.Errorf("east mark fill = %v; want #22c55e", east.Style.Fill)
	}
	if west.Style.Fill != nil && west.Style.Fill.Hex() == "#22c55e" {
		t.Errorf("west mark unexpectedly inherited #22c55e: %v", west.Style.Fill)
	}
}

// TestEncodeConditionSelectionEmitsEntry — a selection-driven
// condition does not bake into Style; instead it appends an entry to
// Mark.Conditions that the browser-side selection layer will react to.
func TestEncodeConditionSelectionEmitsEntry(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {
			"name": "scores",
			"values": [
				{"region": "west", "score": 0.42},
				{"region": "east", "score": 0.91}
			]
		},
		"mark": "bar",
		"encoding": {
			"x": {"field": "region", "type": "nominal"},
			"y": {"field": "score",  "type": "quantitative"},
			"color": {
				"condition": [
					{"selection": "brush", "value": "#22c55e"}
				],
				"value": "#cbd5e1"
			}
		},
		"selection": {"brush": {"type": "interval", "encodings": ["x"]}}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	tables, tipID := buildAndExecute(t, s)
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	marks := doc.Grid.Cells[0].Scene.Layers[0].Marks
	for mi, m := range marks {
		if len(m.Conditions) != 1 {
			t.Fatalf("mark[%d] Conditions = %d, want 1", mi, len(m.Conditions))
		}
		c := m.Conditions[0]
		if c.Attr != "fill" || c.Selection != "brush" || c.WhenValue != "#22c55e" {
			t.Errorf("mark[%d].Conditions[0] = %+v", mi, c)
		}
	}
}

func findMarkByRow(t *testing.T, marks []scene.Mark, row int64) scene.Mark {
	t.Helper()
	for _, m := range marks {
		if m.Datum != nil && m.Datum.RowID == row {
			return m
		}
	}
	t.Fatalf("no mark for row=%d", row)
	return scene.Mark{}
}
