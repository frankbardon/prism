package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SelectionRef is dormant as of E2-S1: structured filter predicates
// carry no selection references (the "selection:<name>" shorthand lived
// in the old free-form filter expression string, which is gone), and
// condition-encoding references land in v2. The rule reports nothing.
func TestSelectionRefNoOp(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"brush": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"x"}}},
		},
		Transform: []spec.Transform{{
			Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: 0}},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := SelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}
