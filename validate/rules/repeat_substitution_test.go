package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// TestPrismRepeatSubstitutionPositive — declared axis substitution
// validates clean.
func TestPrismRepeatSubstitutionPositive(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"day": "2026-01-01", "score": 0.4, "latency_ms": 110}]},
		"repeat": {"row": ["score", "latency_ms"]},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "line",
			"encoding": {
				"x": {"field": "day", "type": "temporal"},
				"y": {"field": {"repeat": "row"}, "type": "quantitative"}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := RepeatSubstitution{}.Check(s, validate.EmptyLookup{})
	if len(out) != 0 {
		t.Errorf("unexpected errors: %+v", out)
	}
}

// TestPrismRepeatSubstitutionNegative — substitution references an
// axis the parent did not declare; rule fires.
func TestPrismRepeatSubstitutionNegative(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"day": "2026-01-01", "score": 0.4}]},
		"repeat": {"row": ["score"]},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "bar",
			"encoding": {
				"x": {"field": "day", "type": "nominal"},
				"y": {"field": {"repeat": "column"}, "type": "quantitative"}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := RepeatSubstitution{}.Check(s, validate.EmptyLookup{})
	if len(out) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(out), out)
	}
	if out[0].Code != "PRISM_SPEC_012" {
		t.Errorf("code = %q, want PRISM_SPEC_012", out[0].Code)
	}
	if out[0].Context["Axis"] != "column" {
		t.Errorf("Context[Axis]=%v, want column", out[0].Context["Axis"])
	}
}

// TestPrismRepeatSubstitutionBothAxes — both axes declared, both
// substitutions resolve clean.
func TestPrismRepeatSubstitutionBothAxes(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"day": "2026-01-01", "a": 1.0, "b": 2.0, "x": 3.0, "y": 4.0}]},
		"repeat": {"row": ["a", "b"], "column": ["x", "y"]},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "point",
			"encoding": {
				"x": {"field": {"repeat": "column"}, "type": "quantitative"},
				"y": {"field": {"repeat": "row"},    "type": "quantitative"}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := RepeatSubstitution{}.Check(s, validate.EmptyLookup{})
	if len(out) != 0 {
		t.Errorf("unexpected errors: %+v", out)
	}
}

// TestPrismRepeatSubstitutionNoRepeat — a non-repeat spec with no
// substitution should never fire the rule.
func TestPrismRepeatSubstitutionNoRepeat(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"a": 1.0, "b": 2.0}]},
		"mark": "bar",
		"encoding": {
			"x": {"field": "a", "type": "quantitative"},
			"y": {"field": "b", "type": "quantitative"}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := RepeatSubstitution{}.Check(s, validate.EmptyLookup{})
	if len(out) != 0 {
		t.Errorf("unexpected errors: %+v", out)
	}
}
