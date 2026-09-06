package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// TestEncodeCategoryStyles_AppliesMatchingEntry — a chart with no
// spec-level condition still picks up a theme-level category_styles
// entry automatically: the matched datum's fill comes from the theme,
// and a non-matching datum keeps the base default.
func TestEncodeCategoryStyles_AppliesMatchingEntry(t *testing.T) {
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
			"y": {"field": "score",  "type": "quantitative"}
		},
		"theme": {
			"name": "light",
			"category_styles": {
				"region": {
					"east": {"fill": "#22c55e"}
				}
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
	west := findMarkByRow(t, marks, 0)
	east := findMarkByRow(t, marks, 1)
	if east.Style.Fill == nil || east.Style.Fill.Hex() != "#22c55e" {
		t.Errorf("east mark fill = %v; want #22c55e", east.Style.Fill)
	}
	if west.Style.Fill != nil && west.Style.Fill.Hex() == "#22c55e" {
		t.Errorf("west mark unexpectedly inherited #22c55e: %v", west.Style.Fill)
	}
}

// TestEncodeCategoryStyles_NoMatchLeavesBaseStyle — a category_styles
// map that has entries but none matching the data present renders
// every mark with the base/default style, unchanged.
func TestEncodeCategoryStyles_NoMatchLeavesBaseStyle(t *testing.T) {
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
			"y": {"field": "score",  "type": "quantitative"}
		},
		"theme": {
			"name": "light",
			"category_styles": {
				"region": {
					"south": {"fill": "#22c55e"}
				}
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

	// Same spec minus the category_styles block should produce an
	// identical style for every mark — the presence of unmatched
	// category_styles entries must be a no-op.
	plainBody := []byte(`{
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
			"y": {"field": "score",  "type": "quantitative"}
		},
		"theme": {"name": "light"}
	}`)
	plainSpec, err := spec.DecodeBytes(plainBody)
	if err != nil {
		t.Fatalf("DecodeBytes (plain): %v", err)
	}
	plainTables, plainTipID := buildAndExecute(t, plainSpec)
	plainDoc, err := encode.Encode(plainSpec, plainTables, plainTipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode (plain): %v", err)
	}

	marks := doc.Grid.Cells[0].Scene.Layers[0].Marks
	plainMarks := plainDoc.Grid.Cells[0].Scene.Layers[0].Marks
	if len(marks) != len(plainMarks) {
		t.Fatalf("mark count mismatch: %d vs %d", len(marks), len(plainMarks))
	}
	for i := range marks {
		gotFill := fillHex(marks[i].Style.Fill)
		wantFill := fillHex(plainMarks[i].Style.Fill)
		if gotFill != wantFill {
			t.Errorf("mark[%d] fill = %q, want %q (unmatched category_styles must be a no-op)", i, gotFill, wantFill)
		}
	}
}

// TestEncodeCategoryStyles_ConditionWinsOverCategoryStyle — when a
// spec-level condition and a theme-level category_styles entry both
// target the same field/value, the condition's resolved style wins.
// This proves the precedence order the story requires: applyCategoryStyles
// must run before applyConditions so the later write (the condition)
// overwrites the earlier one (the category style).
func TestEncodeCategoryStyles_ConditionWinsOverCategoryStyle(t *testing.T) {
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
					{"test": {"op": "eq", "field": "region", "value": "east"}, "value": "#22c55e"}
				],
				"value": "#cbd5e1"
			}
		},
		"theme": {
			"name": "light",
			"category_styles": {
				"region": {
					"east": {"fill": "#111111"},
					"west": {"fill": "#222222"}
				}
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
	west := findMarkByRow(t, marks, 0)
	east := findMarkByRow(t, marks, 1)

	// east matches both the condition (region == "east") and the
	// category_styles entry (region.east) — the condition's
	// "#22c55e" must win over the category style's "#111111".
	if east.Style.Fill == nil || east.Style.Fill.Hex() != "#22c55e" {
		t.Errorf("east mark fill = %v; want #22c55e (condition must win over category_styles)", east.Style.Fill)
	}
	// west only matches the category_styles entry (no condition
	// targets region == "west"), so the category style applies.
	if west.Style.Fill == nil || west.Style.Fill.Hex() != "#222222" {
		t.Errorf("west mark fill = %v; want #222222 (category_styles applies with no competing condition)", west.Style.Fill)
	}
}

func fillHex(c *scene.Color) string {
	if c == nil {
		return ""
	}
	return c.Hex()
}
