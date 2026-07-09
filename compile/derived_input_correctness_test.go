package compile_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/validate"

	// Blank import registers the default semantic rule set (including
	// CrosstabStructure / RegressionStructure) so the full-path
	// validation assertions below run the real registry, not a rule in
	// isolation.
	_ "github.com/frankbardon/prism/validate/rules"
)

// These tests lock in the E3 capability lift: crosstab and regression now
// accept a DERIVED (non-leaf) input. E3-S1 added "it runs" coverage at the
// Build+Execute layer; this file upgrades that to CORRECTNESS-CHECKED
// coverage — every derived-input result is compared against the same
// computation performed directly on the pre-filtered rows, so a broken
// filter (one that failed to narrow the input) would make the equality
// assertions fail. The expected values are recomputed in-test from the
// committed inline rows, so no hand-copied magic numbers can drift.

// filterThreshold keeps score strictly greater than this. Chosen so the
// filter drops a meaningful chunk of the tiny cohort (roughly half),
// guaranteeing the pre/post-filter counts differ.
const filterThreshold = 0.5

// numAt coerces an int/float column cell to float64 for comparison.
func numAt(t *testing.T, col table.Column, i int) float64 {
	t.Helper()
	switch x := col.ValueAt(i).(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	default:
		t.Fatalf("cell %d is not numeric: %T (%v)", i, col.ValueAt(i), col.ValueAt(i))
		return 0
	}
}

// crosstabCellKey composes the (brand_id, age) address of a crosstab cell.
func crosstabCellKey(brand string, age int64) string {
	return fmt.Sprintf("%s|%d", brand, age)
}

// expectedCrosstabCounts computes the brand_id × age count(score) crosstab
// directly over the rows whose score exceeds minScore (minScore < 0 keeps
// all rows). Returns the per-cell counts plus the kept/total record split
// so callers can assert the filter actually narrowed the input.
func expectedCrosstabCounts(t *testing.T, rows []map[string]any, minScore float64) (counts map[string]float64, kept, total int) {
	t.Helper()
	counts = map[string]float64{}
	total = len(rows)
	for _, r := range rows {
		score, ok := r["score"].(float64)
		if !ok {
			t.Fatalf("row missing float score: %v", r)
		}
		if !(score > minScore) {
			continue
		}
		kept++
		brand, _ := r["brand_id"].(string)
		age, _ := r["age"].(float64)
		// count(score) skips nulls; every kept row has a present score,
		// so the cell count is the partition size.
		counts[crosstabCellKey(brand, int64(age))]++
	}
	return counts, kept, total
}

// readCrosstabCounts drains a brand_id × age × n crosstab table into a
// per-cell count map keyed by (brand_id, age).
func readCrosstabCounts(t *testing.T, tbl *table.Table) map[string]float64 {
	t.Helper()
	brandCol, ok := tbl.Column("brand_id")
	if !ok {
		t.Fatal("crosstab output missing brand_id column")
	}
	ageCol, ok := tbl.Column("age")
	if !ok {
		t.Fatal("crosstab output missing age column")
	}
	nCol, ok := tbl.Column("n")
	if !ok {
		t.Fatal("crosstab output missing n cell column")
	}
	out := map[string]float64{}
	for i := 0; i < tbl.NumRows(); i++ {
		brand, _ := brandCol.ValueAt(i).(string)
		age := int64(numAt(t, ageCol, i))
		out[crosstabCellKey(brand, age)] = numAt(t, nCol, i)
	}
	return out
}

// assertCountsEqual fails if got and want disagree on any cell (including
// cells present in only one of them).
func assertCountsEqual(t *testing.T, got, want map[string]float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("crosstab cell count = %d, want %d\n got=%v\nwant=%v", len(got), len(want), got, want)
	}
	for key, w := range want {
		g, ok := got[key]
		if !ok {
			t.Errorf("cell %q missing from pipeline output (want count %v)", key, w)
			continue
		}
		if g != w {
			t.Errorf("cell %q count = %v, want %v", key, g, w)
		}
	}
}

// crosstabSpec builds a brand_id × age count(score) crosstab body.
func crosstabBody() *spec.CrosstabTransform {
	return &spec.CrosstabTransform{Crosstab: spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "brand_id"}},
		Columns: []spec.CrosstabGroup{{Field: "age", Type: "category"}},
		Cell:    spec.CrosstabCell{Aggregate: "count", Field: "score", As: "n"},
	}}
}

// execTip builds + executes a spec over the tiny cohort and returns the
// tip table, failing on any build/exec/node error.
func execTip(t *testing.T, s *spec.Spec) *table.Table {
	t.Helper()
	fs := afero.NewMemMapFs()
	dag, tipID, err := build.Build(s, build.Options{
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
	final := res.Tables[tipID]
	if final == nil {
		t.Fatal("no tip table")
	}
	return final
}

// TestFilterThenCrosstabMatchesPrefilteredCounts is the correctness-checked
// upgrade of TestCrosstabAcceptsDerivedInput: it asserts the filter→crosstab
// pivot equals the crosstab computed directly on the pre-filtered rows,
// cell-by-cell. If the filter node failed to narrow the input, the pipeline
// would produce counts over the full cohort and the equality check would
// fail — so this both proves the derived-input path AND that the filter is
// engaged.
func TestFilterThenCrosstabMatchesPrefilteredCounts(t *testing.T) {
	rows := loadInlineRows(t, "tiny.json")
	want, kept, total := expectedCrosstabCounts(t, rows, filterThreshold)

	// Sanity: the filter must actually narrow the input (kept < total),
	// otherwise the equality assertion below would not be meaningful.
	if kept == total {
		t.Fatalf("filter did not narrow input (kept=total=%d); pick a threshold that drops rows", total)
	}
	if kept == 0 {
		t.Fatal("filter dropped every row; threshold too aggressive")
	}

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: filterThreshold}}},
			{Crosstab: crosstabBody()},
		},
	}
	got := readCrosstabCounts(t, execTip(t, s))
	assertCountsEqual(t, got, want)

	// Cross-check the discriminating property directly: the crosstab over
	// the FULL cohort has strictly more total records than the filtered
	// one, so the two pivots are genuinely different tables.
	full, _, _ := expectedCrosstabCounts(t, rows, -1)
	if sumCounts(full) <= sumCounts(want) {
		t.Fatalf("filtered crosstab total (%v) not smaller than full (%v); filter not discriminating",
			sumCounts(want), sumCounts(full))
	}
}

// sumCounts totals a cell-count map.
func sumCounts(m map[string]float64) float64 {
	var s float64
	for _, v := range m {
		s += v
	}
	return s
}

// TestFilterThenSortThenCrosstabExecutes guards the GENERAL multi-transform
// prefix case (not just a single filter): filter → sort → crosstab must
// Build+Execute and still produce the correct pivot. Sort is order-only, so
// the counts must match the filtered expectation regardless of ordering.
func TestFilterThenSortThenCrosstabExecutes(t *testing.T) {
	rows := loadInlineRows(t, "tiny.json")
	want, kept, total := expectedCrosstabCounts(t, rows, filterThreshold)
	if kept == total || kept == 0 {
		t.Fatalf("filter must narrow input (kept=%d, total=%d)", kept, total)
	}

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: filterThreshold}}},
			{Sort: &spec.SortTransform{Sort: []spec.SortFieldDef{{Field: "age", Order: "ascending"}}}},
			{Crosstab: crosstabBody()},
		},
	}

	fs := afero.NewMemMapFs()
	dag, tipID, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: tinyResolver(t),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build filter→sort→crosstab: %v", err)
	}
	// Source + Filter + Sort + Crosstab = 4.
	if got := len(dag.Nodes()); got != 4 {
		t.Errorf("DAG node count = %d, want 4 (Source+Filter+Sort+Crosstab)", got)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute had %d node errors; first = %v", len(res.Errors), res.Errors[0])
	}
	got := readCrosstabCounts(t, res.Tables[tipID])
	assertCountsEqual(t, got, want)
}

// TestCrosstabLeafInputStillCorrect guards the pre-existing LEAF-input case
// (crosstab as the first transform over the source) using the same
// correctness harness — it must keep producing the full-cohort pivot.
func TestCrosstabLeafInputStillCorrect(t *testing.T) {
	rows := loadInlineRows(t, "tiny.json")
	want, kept, total := expectedCrosstabCounts(t, rows, -1) // -1 keeps all
	if kept != total {
		t.Fatalf("leaf case must keep all rows (kept=%d, total=%d)", kept, total)
	}

	s := &spec.Spec{
		Data:      &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{{Crosstab: crosstabBody()}},
	}
	got := readCrosstabCounts(t, execTip(t, s))
	assertCountsEqual(t, got, want)
}

// expectedOLS fits score ~ age over the rows whose score exceeds minScore,
// mirroring executeRegression's centred-sufficient-statistics estimator so
// the two agree to floating-point tolerance. Returns slope, intercept, and
// the predictor extent (xmin/xmax) that anchor the two emitted endpoints.
func expectedOLS(t *testing.T, rows []map[string]any, minScore float64) (slope, intercept, xmin, xmax float64, kept int) {
	t.Helper()
	var n int
	var sumX, sumY, sumXX, sumXY float64
	haveExtent := false
	for _, r := range rows {
		score, _ := r["score"].(float64)
		if !(score > minScore) {
			continue
		}
		kept++
		age, _ := r["age"].(float64)
		x, y := age, score
		if !haveExtent {
			xmin, xmax, haveExtent = x, x, true
		} else {
			if x < xmin {
				xmin = x
			}
			if x > xmax {
				xmax = x
			}
		}
		n++
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
	}
	if n < 2 {
		t.Fatalf("need >=2 complete records for OLS, got %d", n)
	}
	nf := float64(n)
	meanX := sumX / nf
	meanY := sumY / nf
	sxx := sumXX - nf*meanX*meanX
	sxy := sumXY - nf*meanX*meanY
	slope = sxy / sxx
	intercept = meanY - slope*meanX
	return slope, intercept, xmin, xmax, kept
}

// TestFilterThenRegressionMatchesPrefilteredFit is the correctness-checked
// upgrade of TestRegressionAcceptsDerivedInput: it asserts the OLS endpoints
// emitted for filter→regression equal the fit computed directly over the
// filtered subset. A no-op filter would fit the full cohort instead, giving
// different endpoints, so this proves both derived input and filter
// engagement.
func TestFilterThenRegressionMatchesPrefilteredFit(t *testing.T) {
	rows := loadInlineRows(t, "tiny.json")
	slope, intercept, xmin, xmax, kept := expectedOLS(t, rows, filterThreshold)
	if kept == len(rows) {
		t.Fatalf("filter did not narrow input (kept=total=%d)", len(rows))
	}

	// The filtered fit must genuinely differ from the full-cohort fit,
	// else the equality assertion could pass with a broken filter.
	fullSlope, fullIntercept, _, _, _ := expectedOLS(t, rows, -1)
	if math.Abs(slope-fullSlope) < 1e-9 && math.Abs(intercept-fullIntercept) < 1e-9 {
		t.Fatalf("filtered fit indistinguishable from full fit; filter not discriminating")
	}

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: filterThreshold}}},
			{Regression: &spec.RegressionTransform{Regression: spec.RegressionBody{
				Target:     "score",
				Predictors: []string{"age"},
				As:         "fitted",
			}}},
		},
	}
	final := execTip(t, s)
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

	// Endpoints may arrive in either order; match each emitted predictor
	// value to the expected extent and assert the fitted value.
	want := func(x float64) float64 { return intercept + slope*x }
	sawMin, sawMax := false, false
	for i := 0; i < final.NumRows(); i++ {
		x := numAt(t, ageCol, i)
		got := numAt(t, fitCol, i)
		if math.Abs(got-want(x)) > 1e-9 {
			t.Errorf("endpoint %d: fitted(%v)=%v, want %v", i, x, got, want(x))
		}
		switch {
		case math.Abs(x-xmin) < 1e-9:
			sawMin = true
		case math.Abs(x-xmax) < 1e-9:
			sawMax = true
		}
	}
	if !sawMin || !sawMax {
		t.Errorf("endpoints did not span the filtered predictor extent [%v,%v] (sawMin=%v sawMax=%v)",
			xmin, xmax, sawMin, sawMax)
	}
}

// TestDerivedInputPassesSemanticValidation is the full-path validation
// assertion (complement to the E3-S2 rule-in-isolation tests): a
// filter→crosstab / filter→regression spec run through the whole default
// semantic rule registry must emit no retired position error
// (PRISM_SPEC_033) and no crosstab/regression structural error
// (PRISM_SPEC_032/034/035). This proves the lifted constraint holds end to
// end through the registered validator, not just the extracted rule.
func TestDerivedInputPassesSemanticValidation(t *testing.T) {
	sem := validate.NewDefaultSemanticValidator()

	cases := []struct {
		name string
		s    *spec.Spec
	}{
		{
			name: "filter→crosstab",
			s: &spec.Spec{
				Schema: "urn:prism:schema:v1:spec",
				Transform: []spec.Transform{
					{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: filterThreshold}}},
					{Crosstab: crosstabBody()},
				},
			},
		},
		{
			name: "filter→regression",
			s: &spec.Spec{
				Schema: "urn:prism:schema:v1:spec",
				Transform: []spec.Transform{
					{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: filterThreshold}}},
					{Regression: &spec.RegressionTransform{Regression: spec.RegressionBody{
						Target:     "score",
						Predictors: []string{"age"},
						As:         "fitted",
					}}},
				},
			},
		},
	}

	// Codes that would signal the (now-lifted) position constraint or a
	// crosstab/regression shape violation.
	forbidden := map[string]bool{
		"PRISM_SPEC_032": true, // crosstab shape
		"PRISM_SPEC_033": true, // retired position error
		"PRISM_SPEC_034": true, // crosstab normalize
		"PRISM_SPEC_035": true, // regression shape/position
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, e := range sem.Validate(tc.s, validate.EmptyLookup{}) {
				if forbidden[e.Code] {
					t.Errorf("%s emitted forbidden code %s: %s", tc.name, e.Code, e.Message)
				}
			}
		})
	}
}
