package compile_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/spec"
)

// TestCrosstabAcceptsDerivedInput builds a Source → Filter → Crosstab DAG
// over the tiny cohort and asserts the crosstab now consumes a DERIVED
// (non-leaf) input — the filter output — rather than being pinned to a
// materialised leaf. Pre-E3 this raised
// PRISM_PLAN_CROSSTAB_REQUIRES_SOURCE at build time. The assertion runs
// at the plan Build+Execute layer (validate is bypassed), which is the
// surface this story generalises.
func TestCrosstabAcceptsDerivedInput(t *testing.T) {
	fs := afero.NewMemMapFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: 0.5}}},
			{Crosstab: &spec.CrosstabTransform{Crosstab: spec.CrosstabBody{
				Rows:    []spec.CrosstabGroup{{Field: "brand_id"}},
				Columns: []spec.CrosstabGroup{{Field: "age", Type: "category"}},
				Cell:    spec.CrosstabCell{Aggregate: "count", Field: "score", As: "n"},
			}}},
		},
	}

	dag, tipID, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: tinyResolver(t),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build filter→crosstab must succeed (no REQUIRES_SOURCE): %v", err)
	}
	// Source + Filter + Crosstab = 3; the crosstab consumes the filter.
	if got := len(dag.Nodes()); got != 3 {
		t.Errorf("DAG node count = %d, want 3 (Source+Filter+Crosstab)", got)
	}

	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute had %d node errors; first = %v", len(res.Errors), res.Errors[0])
	}
	final := res.Tables[tipID]
	if final == nil {
		t.Fatal("no crosstab tip table")
	}
	if _, ok := final.Column("brand_id"); !ok {
		t.Error("crosstab output missing row axis column brand_id")
	}
	nCol, ok := final.Column("n")
	if !ok {
		t.Fatal("crosstab output missing cell column n")
	}
	// The filter keeps score > 0.5, so at least one cell survives.
	total := 0.0
	for i := 0; i < final.NumRows(); i++ {
		v, _ := nCol.ValueAt(i).(float64)
		total += v
	}
	if final.NumRows() == 0 || total == 0 {
		t.Fatal("crosstab over the filtered input produced no cells")
	}
}

// TestRegressionAcceptsDerivedInput builds a Source → Filter → Regression
// DAG over the tiny cohort and asserts the regression fits the DERIVED
// filter output (previously PRISM_PLAN_REGRESSION_REQUIRES_SOURCE). The
// OLS fit still emits the two trend-line endpoints.
func TestRegressionAcceptsDerivedInput(t *testing.T) {
	fs := afero.NewMemMapFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: 0.5}}},
			{Regression: &spec.RegressionTransform{Regression: spec.RegressionBody{
				Target:     "score",
				Predictors: []string{"age"},
				As:         "fitted",
			}}},
		},
	}

	dag, tipID, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: tinyResolver(t),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build filter→regression must succeed (no REQUIRES_SOURCE): %v", err)
	}
	if got := len(dag.Nodes()); got != 3 {
		t.Errorf("DAG node count = %d, want 3 (Source+Filter+Regression)", got)
	}

	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute had %d node errors; first = %v", len(res.Errors), res.Errors[0])
	}
	final := res.Tables[tipID]
	if final == nil {
		t.Fatal("no regression tip table")
	}
	if final.NumRows() != 2 {
		t.Fatalf("regression output rows = %d, want 2 endpoints", final.NumRows())
	}
	if _, ok := final.Column("age"); !ok {
		t.Error("regression output missing predictor column age")
	}
	if _, ok := final.Column("fitted"); !ok {
		t.Error("regression output missing fitted column")
	}
}
