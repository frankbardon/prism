package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestShapeValidatorAcceptsTableMark asserts the schema bundle accepts
// mark: {type: "table", page_size: N} with encoding.columns[] (E1-S2),
// including a column carrying a sub-mark for its cells.
func TestShapeValidatorAcceptsTableMark(t *testing.T) {
	v, err := NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	const table = `{
        "$schema": "urn:prism:schema:v1:spec",
        "data": {"values": [{"name": "acme", "trend": 1}]},
        "mark": {"type": "table", "page_size": 50},
        "encoding": {
            "columns": [
                {"field": "name", "type": "nominal", "title": "Name"},
                {"field": "trend", "type": "quantitative", "mark": "sparkline"}
            ]
        }
    }`
	var doc any
	if err := json.NewDecoder(strings.NewReader(table)).Decode(&doc); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if errs := v.Validate(doc); len(errs) > 0 {
		t.Fatalf("expected zero shape errors, got: %+v", errs)
	}
}
