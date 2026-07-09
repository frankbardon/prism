package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// wellFormedCrosstab returns a shape-valid crosstab transform.
func wellFormedCrosstab() *spec.CrosstabTransform {
	return &spec.CrosstabTransform{
		Crosstab: spec.CrosstabBody{
			Rows:    []spec.CrosstabGroup{{Field: "region"}},
			Columns: []spec.CrosstabGroup{{Field: "quarter"}},
			Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "revenue"},
		},
	}
}

// A crosstab that follows a filter (derived input) must pass — the
// former "must be the first transform" position error was retired.
func TestCrosstabStructureAllowsDerivedInput(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: "gt", Field: "revenue", Value: 0}}},
			{Crosstab: wellFormedCrosstab()},
		},
	}
	if errs := (CrosstabStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("filter→crosstab must pass structural validation, got: %+v", errs)
	}
}

// A crosstab as the sole/first transform is still fine.
func TestCrosstabStructureAllowsFirstPosition(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{{Crosstab: wellFormedCrosstab()}},
	}
	if errs := (CrosstabStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("first-position crosstab must pass, got: %+v", errs)
	}
}

// A crosstab missing rows still fires PRISM_SPEC_032, even in the
// derived-input position.
func TestCrosstabStructureFiresOnMissingRows(t *testing.T) {
	ct := wellFormedCrosstab()
	ct.Crosstab.Rows = nil
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: "gt", Field: "revenue", Value: 0}}},
			{Crosstab: ct},
		},
	}
	errs := CrosstabStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_032" {
		t.Fatalf("expected exactly one PRISM_SPEC_032 for missing rows, got: %+v", errs)
	}
}

// A crosstab missing columns still fires PRISM_SPEC_032 (no position
// error alongside it).
func TestCrosstabStructureFiresOnMissingColumns(t *testing.T) {
	ct := wellFormedCrosstab()
	ct.Crosstab.Columns = nil
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{{Crosstab: ct}},
	}
	errs := CrosstabStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_032" {
		t.Fatalf("expected exactly one PRISM_SPEC_032 for missing columns, got: %+v", errs)
	}
}

// A bad normalize value still fires PRISM_SPEC_034.
func TestCrosstabStructureFiresOnBadNormalize(t *testing.T) {
	ct := wellFormedCrosstab()
	ct.Crosstab.Normalize = "diagonal"
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Transform: []spec.Transform{{Crosstab: ct}},
	}
	errs := CrosstabStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_034" {
		t.Fatalf("expected exactly one PRISM_SPEC_034, got: %+v", errs)
	}
}
