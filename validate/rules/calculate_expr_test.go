package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func calcSpec(expr spec.CalcExpr, as string) *spec.Spec {
	return &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Data:      &spec.Data{Name: "cohort"},
		Mark:      &spec.Mark{Shorthand: "bar"},
		Transform: []spec.Transform{{Calculate: &spec.CalculateTransform{Calculate: expr, As: as}}},
	}
}

func TestCalculateExprHappyPath(t *testing.T) {
	expr := spec.CalcExpr{Op: spec.CalcDiv, Operands: []spec.CalcExpr{
		{Field: "score"}, {Field: "target"},
	}}
	if errs := (CalculateExpr{}).Check(calcSpec(expr, "ratio"), cohortLookup()); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestCalculateExprUnknownField(t *testing.T) {
	expr := spec.CalcExpr{Op: spec.CalcAdd, Operands: []spec.CalcExpr{
		{Field: "score"}, {Field: "nope"},
	}}
	errs := CalculateExpr{}.Check(calcSpec(expr, "ratio"), cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_038" {
		t.Fatalf("want one PRISM_SPEC_038, got: %+v", errs)
	}
}

func TestCalculateExprCaseWhenUnknownField(t *testing.T) {
	expr := spec.CalcExpr{
		Case: []spec.CalcBranch{
			{When: spec.Predicate{Op: spec.PredGt, Field: "nope", Value: float64(1)}, Then: spec.CalcExpr{Literal: float64(1)}},
		},
		Else: &spec.CalcExpr{Literal: float64(0)},
	}
	// case.when reuses the shared filter-predicate checker, so a bad
	// predicate field surfaces as PRISM_SPEC_037 (not duplicated as 038).
	errs := CalculateExpr{}.Check(calcSpec(expr, "bucket"), cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_037" {
		t.Fatalf("want one PRISM_SPEC_037 for case.when unknown field, got: %+v", errs)
	}
}

func TestCalculateExprDivByLiteralZero(t *testing.T) {
	expr := spec.CalcExpr{Op: spec.CalcDiv, Operands: []spec.CalcExpr{
		{Field: "score"}, {Literal: float64(0)},
	}}
	errs := CalculateExpr{}.Check(calcSpec(expr, "ratio"), cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_038" {
		t.Fatalf("want one PRISM_SPEC_038 for div-by-literal-zero, got: %+v", errs)
	}
}

func TestCalculateExprAsShadowsColumn(t *testing.T) {
	expr := spec.CalcExpr{Field: "score"}
	errs := CalculateExpr{}.Check(calcSpec(expr, "score"), cohortLookup())
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_038" {
		t.Fatalf("want one PRISM_SPEC_038 for shadowing `as`, got: %+v", errs)
	}
}

func TestCalculateExprEmptyAs(t *testing.T) {
	// EmptyLookup means schema is unknown, so only the schema-independent
	// `as` check should fire.
	expr := spec.CalcExpr{Field: "score"}
	errs := CalculateExpr{}.Check(calcSpec(expr, ""), validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_038" {
		t.Fatalf("want one PRISM_SPEC_038 for empty `as`, got: %+v", errs)
	}
}
