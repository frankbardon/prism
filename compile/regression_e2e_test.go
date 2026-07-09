package compile_test

import (
	"context"
	"math"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/spec"
)

// Frozen REG_OLS coefficients for score ~ age over the tiny cohort,
// captured via pulse.Process (REG_OLS) at the point Pulse left the
// ingestion path. See inline_fixture_test.go.
const (
	frozenRegIntercept = 0.5069977372795017
	frozenRegSlope     = -0.00017968737184751842
)

// TestRegressionTrendEndpoints runs a regression transform (score ~ age)
// over the tiny cohort's inline rows and asserts it emits exactly two
// trend-line endpoints whose fitted values match the frozen Pulse OLS
// fit (intercept + slope*x at each emitted predictor value). This pins
// the leaf node's coefficient + x-domain wiring to the value Pulse's OLS
// produced, without a live Pulse dependency.
func TestRegressionTrendEndpoints(t *testing.T) {
	fs := afero.NewMemMapFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Regression: &spec.RegressionTransform{Regression: spec.RegressionBody{
				Target:     "score",
				Predictors: []string{"age"},
				As:         "fitted",
			}}},
		},
	}

	dag, _, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: tinyResolver(t),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute had %d node errors; first = %v", len(res.Errors), res.Errors[0])
	}
	final := finalTable(dag, res)
	if final == nil {
		t.Fatal("no tip table")
	}
	if final.NumRows() != 2 {
		t.Fatalf("regression output rows = %d, want 2 endpoints", final.NumRows())
	}
	ageCol, ok := final.Column("age")
	if !ok {
		t.Fatal("missing age (predictor) column")
	}
	fitCol, ok := final.Column("fitted")
	if !ok {
		t.Fatal("missing fitted column")
	}

	want := func(x float64) float64 { return frozenRegIntercept + frozenRegSlope*x }
	for i := 0; i < final.NumRows(); i++ {
		x, _ := ageCol.ValueAt(i).(float64)
		got, _ := fitCol.ValueAt(i).(float64)
		if math.Abs(got-want(x)) > 1e-9 {
			t.Errorf("endpoint %d: fitted(%v)=%v, want %v", i, x, got, want(x))
		}
	}
}
