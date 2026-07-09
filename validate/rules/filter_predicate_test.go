package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func cohortLookup() validate.SchemaLookup {
	l := validate.NewStaticLookup()
	l.Register("cohort", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
		{Name: "score", Type: "quantitative"},
		{Name: "target", Type: "quantitative"},
	}})
	return l
}

func filterSpec(pred spec.Predicate) *spec.Spec {
	return &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Data:      &spec.Data{Name: "cohort"},
		Mark:      &spec.Mark{Shorthand: "bar"},
		Transform: []spec.Transform{{Filter: &spec.FilterTransform{Filter: pred}}},
	}
}

func TestFilterPredicateHappyPath(t *testing.T) {
	s := filterSpec(spec.Predicate{And: []spec.Predicate{
		{Op: spec.PredGt, Field: "score", Value: 100},
		{Op: spec.PredEq, Field: "brand_id", Value: "acme"},
		{Op: spec.PredLt, Field: "score", ToField: "target"},
		{Op: spec.PredOneOf, Field: "brand_id", Values: []any{"a", "b"}},
		{Op: spec.PredBetween, Field: "score", Lo: 1, Hi: 10},
		{Op: spec.PredIsNull, Field: "target"},
	}})
	if errs := (FilterPredicate{}).Check(s, cohortLookup()); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestFilterPredicateUnknownField(t *testing.T) {
	s := filterSpec(spec.Predicate{Op: spec.PredGt, Field: "nope", Value: 1})
	errs := FilterPredicate{}.Check(s, cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037, got: %+v", errs)
	}
}

func TestFilterPredicateUnknownToField(t *testing.T) {
	s := filterSpec(spec.Predicate{Op: spec.PredGt, Field: "score", ToField: "nope"})
	errs := FilterPredicate{}.Check(s, cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037, got: %+v", errs)
	}
}

func TestFilterPredicateTypeMismatchLiteral(t *testing.T) {
	// score is quantitative; comparing to a string literal is invalid.
	s := filterSpec(spec.Predicate{Op: spec.PredGt, Field: "score", Value: "high"})
	errs := FilterPredicate{}.Check(s, cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037, got: %+v", errs)
	}
}

func TestFilterPredicateTypeMismatchFieldVsField(t *testing.T) {
	// brand_id (nominal) vs score (quantitative) mixes domains.
	s := filterSpec(spec.Predicate{Op: spec.PredEq, Field: "brand_id", ToField: "score"})
	errs := FilterPredicate{}.Check(s, cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037, got: %+v", errs)
	}
}

func TestFilterPredicateBetweenReversed(t *testing.T) {
	s := filterSpec(spec.Predicate{Op: spec.PredBetween, Field: "score", Lo: 10, Hi: 1})
	errs := FilterPredicate{}.Check(s, cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037, got: %+v", errs)
	}
}

// Without a resolvable schema (inline data) the rule still catches the
// schema-free defect: a reversed between range.
func TestFilterPredicateBetweenReversedNoSchema(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Data:      &spec.Data{Values: []map[string]any{{"v": 1}}},
		Mark:      &spec.Mark{Shorthand: "bar"},
		Transform: []spec.Transform{{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredBetween, Field: "v", Lo: 10, Hi: 1}}}},
	}
	errs := FilterPredicate{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037, got: %+v", errs)
	}
}
