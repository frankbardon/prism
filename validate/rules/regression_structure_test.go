package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func wellFormedRegression() *spec.RegressionTransform {
	return &spec.RegressionTransform{
		Regression: spec.RegressionBody{
			Target:     "sales",
			Predictors: []string{"spend"},
			As:         "fitted",
		},
	}
}

// A regression that follows a filter (derived input) must pass — the
// former "must be first" position error was retired.
func TestRegressionStructureAllowsDerivedInput(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: "gt", Field: "spend", Value: 0}}},
			{Regression: wellFormedRegression()},
		},
	}
	if errs := (RegressionStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("filter→regression must pass structural validation, got: %+v", errs)
	}
}

func TestRegressionStructureAllowsFirstPosition(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{{Regression: wellFormedRegression()}},
	}
	if errs := (RegressionStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("first-position regression must pass, got: %+v", errs)
	}
}

// A regression missing predictors still fires PRISM_SPEC_035, even in
// the derived-input position (and with no accompanying position error).
func TestRegressionStructureFiresOnMissingPredictors(t *testing.T) {
	rt := wellFormedRegression()
	rt.Regression.Predictors = nil
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: "gt", Field: "spend", Value: 0}}},
			{Regression: rt},
		},
	}
	errs := RegressionStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_035" {
		t.Fatalf("expected exactly one PRISM_SPEC_035 for missing predictors, got: %+v", errs)
	}
}

func TestRegressionStructureFiresOnMissingTarget(t *testing.T) {
	rt := wellFormedRegression()
	rt.Regression.Target = ""
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{{Regression: rt}},
	}
	errs := RegressionStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_035" {
		t.Fatalf("expected exactly one PRISM_SPEC_035 for missing target, got: %+v", errs)
	}
}
