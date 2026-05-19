package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestShapeValidatorAcceptsMinimalBar(t *testing.T) {
	v, err := NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	const minimal = `{
        "$schema": "urn:prism:schema:v1:spec",
        "data": {"values": [{"x": 1, "y": 2}]},
        "mark": "bar",
        "encoding": {
            "x": {"field": "x", "type": "nominal"},
            "y": {"field": "y", "type": "quantitative"}
        }
    }`
	var doc any
	if err := json.NewDecoder(strings.NewReader(minimal)).Decode(&doc); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if errs := v.Validate(doc); len(errs) > 0 {
		t.Fatalf("expected zero shape errors, got: %+v", errs)
	}
}

func TestShapeValidatorRejectsUnknownTopLevel(t *testing.T) {
	v, err := NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	const bad = `{
        "$schema": "urn:prism:schema:v1:spec",
        "mark": "bar",
        "totally_unknown": 1
    }`
	var doc any
	if err := json.NewDecoder(strings.NewReader(bad)).Decode(&doc); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	errs := v.Validate(doc)
	if len(errs) == 0 {
		t.Fatalf("expected shape errors for unknown top-level field, got none")
	}
}

func TestShapeValidatorRequiresOneOfCompositionKeys(t *testing.T) {
	v, err := NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	// Missing mark/layer/concat/...; the oneOf must fail.
	const bad = `{"$schema": "urn:prism:schema:v1:spec"}`
	var doc any
	if err := json.NewDecoder(strings.NewReader(bad)).Decode(&doc); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	errs := v.Validate(doc)
	if len(errs) == 0 {
		t.Fatalf("expected at least one shape error for missing composition key, got none")
	}
}
