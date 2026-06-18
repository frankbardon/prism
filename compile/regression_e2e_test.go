package compile_test

import (
	"context"
	"math"
	"testing"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/types"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// TestRegressionTrendEndpoints runs a regression transform (score ~ age)
// against tiny.pulse end-to-end and asserts it emits exactly two
// trend-line endpoints whose fitted values match a direct Pulse REG_OLS
// fit (intercept + slope*x at the predictor min/max). This pins the
// leaf node's coefficient + x-domain wiring to Pulse's own OLS output.
func TestRegressionTrendEndpoints(t *testing.T) {
	cohortPath := fixturePath(t)
	fs := afero.NewOsFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: cohortPath},
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
		Resolver: resolve.New(nil),
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

	// Direct Pulse REG_OLS fit + predictor range for the expected line.
	pulseInst, err := pulse.New(pulse.Options{FS: fs})
	if err != nil {
		t.Fatalf("pulse.New: %v", err)
	}
	resp, err := pulseInst.Process(context.Background(), &pulse.Request{
		Cohort: &types.Cohort{Filename: cohortPath},
		Aggregations: []*types.Aggregation{
			{Type: types.AGG_MIN, Field: "age", Label: "lo"},
			{Type: types.AGG_MAX, Field: "age", Label: "hi"},
		},
		Regressions: []*types.RegressionSpec{{
			Type:       types.REG_OLS,
			Target:     "score",
			Predictors: []string{"age"},
		}},
	})
	if err != nil {
		t.Fatalf("pulse.Process: %v", err)
	}
	coef := resp.Regressions[0].Coefficients
	intercept := coef["(intercept)"]
	slope := coef["age"]
	want := func(x float64) float64 { return intercept + slope*x }

	for i := 0; i < final.NumRows(); i++ {
		x, _ := ageCol.ValueAt(i).(float64)
		got, _ := fitCol.ValueAt(i).(float64)
		if math.Abs(got-want(x)) > 1e-9 {
			t.Errorf("endpoint %d: fitted(%v)=%v, want %v", i, x, got, want(x))
		}
	}
}
