package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestSelectionRefHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"brush": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"x"}}},
		},
		Transform: []spec.Transform{{
			Filter: &spec.FilterTransform{Filter: "selection:brush and score > 0"},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := SelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestSelectionRefFiresOnUnknown(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"brush": {Interval: &spec.IntervalSelection{Type: "interval"}},
		},
		Transform: []spec.Transform{{
			Filter: &spec.FilterTransform{Filter: "selection:typo"},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := SelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_004" {
		t.Fatalf("expected one PRISM_SPEC_004, got: %+v", errs)
	}
}
