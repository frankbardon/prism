package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestExpressionParsesHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: "score > 0 and brand_id != 'x'"}},
			{Calculate: &spec.CalculateTransform{Calculate: "score * 2", As: "doubled"}},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := ExpressionParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestExpressionParsesFiresOnBadExpr(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: "score > "}},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := ExpressionParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_006" {
		t.Fatalf("expected one PRISM_SPEC_006, got: %+v", errs)
	}
}

func TestExpressionParsesIgnoresSelectionShorthand(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: "selection:brush and score > 0"}},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := ExpressionParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}
