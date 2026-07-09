package inmem

import (
	"context"
	"math"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// regFixture builds a small (age, score) table with an exact linear
// relationship score = 2*age + 3 so the OLS fit is analytically known:
// slope=2, intercept=3, x-domain [10, 40].
func regFixture(t *testing.T) *table.Table {
	t.Helper()
	age := table.FloatColumn{10, 20, 30, 40}
	score := table.FloatColumn{23, 43, 63, 83} // 2*age + 3
	schema := &table.Schema{Fields: []table.Field{
		{Name: "age", Type: table.FieldTypeF64},
		{Name: "score", Type: table.FieldTypeF64},
	}}
	tbl, err := table.NewTable(schema, map[string]table.Column{
		"age": age, "score": score,
	}, 4, "regsrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// runReg builds a RegressionNode over the fixture schema and executes the
// in-memory OLS fit, returning the two-endpoint output table.
func runReg(t *testing.T, in *table.Table, body spec.RegressionBody) *table.Table {
	t.Helper()
	n, err := nodes.NewRegression("reg", "src", "ref", afero.NewMemMapFs(),
		&spec.RegressionTransform{Regression: body})
	if err != nil {
		t.Fatalf("NewRegression: %v", err)
	}
	out, err := executeRegression(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeRegression: %v", err)
	}
	return out
}

func TestRegressionExactLine(t *testing.T) {
	out := runReg(t, regFixture(t), spec.RegressionBody{
		Target: "score", Predictors: []string{"age"}, As: "fitted",
	})
	if out.NumRows() != 2 {
		t.Fatalf("rows = %d, want 2 endpoints", out.NumRows())
	}
	ageCol, ok := out.Column("age")
	if !ok {
		t.Fatal("missing age column")
	}
	fitCol, ok := out.Column("fitted")
	if !ok {
		t.Fatal("missing fitted column")
	}

	// slope=2, intercept=3 → endpoints at x=10 (23) and x=40 (83).
	wantX := []float64{10, 40}
	wantY := []float64{23, 83}
	for i := 0; i < out.NumRows(); i++ {
		x, _ := ageCol.ValueAt(i).(float64)
		y, _ := fitCol.ValueAt(i).(float64)
		if math.Abs(x-wantX[i]) > 1e-9 {
			t.Errorf("endpoint %d age = %v, want %v", i, x, wantX[i])
		}
		if math.Abs(y-wantY[i]) > 1e-9 {
			t.Errorf("endpoint %d fitted = %v, want %v", i, y, wantY[i])
		}
	}
}

// TestRegressionNoisyFit checks the closed-form OLS estimator against a
// hand-computed slope/intercept for non-collinear data.
func TestRegressionNoisyFit(t *testing.T) {
	// x = {1,2,3,4,5}, y = {2,4,5,4,5}. Sxx=10, Sxy=6 → slope=0.6,
	// intercept = ȳ - slope*x̄ = 4 - 0.6*3 = 2.2.
	age := table.FloatColumn{1, 2, 3, 4, 5}
	score := table.FloatColumn{2, 4, 5, 4, 5}
	schema := &table.Schema{Fields: []table.Field{
		{Name: "age", Type: table.FieldTypeF64},
		{Name: "score", Type: table.FieldTypeF64},
	}}
	in, err := table.NewTable(schema, map[string]table.Column{"age": age, "score": score}, 5, "noisy")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	out := runReg(t, in, spec.RegressionBody{Target: "score", Predictors: []string{"age"}})

	fitCol, _ := out.Column("fitted")
	ageCol, _ := out.Column("age")
	// fitted = 2.2 + 0.6*x; endpoints at x=1 → 2.8, x=5 → 5.2.
	want := func(x float64) float64 { return 2.2 + 0.6*x }
	for i := 0; i < out.NumRows(); i++ {
		x, _ := ageCol.ValueAt(i).(float64)
		y, _ := fitCol.ValueAt(i).(float64)
		if math.Abs(y-want(x)) > 1e-9 {
			t.Errorf("endpoint %d fitted(%v) = %v, want %v", i, x, y, want(x))
		}
	}
}

func TestRegressionZeroVarianceFails(t *testing.T) {
	age := table.FloatColumn{5, 5, 5}
	score := table.FloatColumn{1, 2, 3}
	schema := &table.Schema{Fields: []table.Field{
		{Name: "age", Type: table.FieldTypeF64},
		{Name: "score", Type: table.FieldTypeF64},
	}}
	in, err := table.NewTable(schema, map[string]table.Column{"age": age, "score": score}, 3, "flat")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	n, err := nodes.NewRegression("reg", "src", "ref", afero.NewMemMapFs(),
		&spec.RegressionTransform{Regression: spec.RegressionBody{Target: "score", Predictors: []string{"age"}}})
	if err != nil {
		t.Fatalf("NewRegression: %v", err)
	}
	if _, err := executeRegression(context.Background(), n, []*table.Table{in}); err == nil {
		t.Fatal("expected zero-variance predictor to fail the fit")
	}
}
