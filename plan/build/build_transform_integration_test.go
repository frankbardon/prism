package build_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// structuredTransformSpec exercises BOTH a structured filter and two
// chained structured calculate transforms over inline data — the level
// above the per-operator unit coverage in compile/inmem. The filter is
// an `and` combinator that packs the edge cases the acceptance criteria
// call out into one predicate:
//
//   - field-vs-field comparison  (score > target, via to_field)
//   - inclusive range            (between score 0.40..0.70)
//   - set membership             (region one_of {NA, LATAM, MEA, ANZ})
//   - null handling              (not_null flag → the null-flag row drops)
//
// The calculate chain then exercises `coalesce` (null fallback on a
// surviving row) and a `case` with a fall-through branch plus `else`.
//
//	region | score | target | flag | bonus |
//	 NA    | 0.42  | 0.40   | 1    | 5     | PASS  (else→"low",  bonus 5)
//	 EU    | 0.71  | 0.65   | 1    | 8     | drop  (between: 0.71 > 0.70)
//	 APAC  | 0.58  | 0.60   | 1    | 4     | drop  (set + score<target)
//	 LATAM | 0.56  | 0.55   | null | 3     | drop  (flag is null)
//	 MEA   | 0.66  | 0.30   | 1    | null  | PASS  (high, bonus→0 coalesce)
//	 ANZ   | 0.45  | 0.40   | 1    | 2     | PASS  (mid  fall-through, bonus 2)
const structuredTransformSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "title": "Structured filter + calculate integration",
  "data": {
    "name": "regions",
    "values": [
      {"region": "NA",    "score": 0.42, "target": 0.40, "flag": 1.0,  "bonus": 5.0},
      {"region": "EU",    "score": 0.71, "target": 0.65, "flag": 1.0,  "bonus": 8.0},
      {"region": "APAC",  "score": 0.58, "target": 0.60, "flag": 1.0,  "bonus": 4.0},
      {"region": "LATAM", "score": 0.56, "target": 0.55, "flag": null, "bonus": 3.0},
      {"region": "MEA",   "score": 0.66, "target": 0.30, "flag": 1.0,  "bonus": null},
      {"region": "ANZ",   "score": 0.45, "target": 0.40, "flag": 1.0,  "bonus": 2.0}
    ]
  },
  "transform": [
    {"filter": {"and": [
      {"op": "gt", "field": "score", "to_field": "target"},
      {"op": "between", "field": "score", "lo": 0.40, "hi": 0.70},
      {"op": "one_of", "field": "region", "values": ["NA", "LATAM", "MEA", "ANZ"]},
      {"op": "not_null", "field": "flag"}
    ]}},
    {"calculate": {"fn": "coalesce", "args": [{"field": "bonus"}, {"literal": 0}]}, "as": "bonus_filled"},
    {"calculate": {"case": [
      {"when": {"op": "gte", "field": "score", "value": 0.60}, "then": {"literal": "high"}},
      {"when": {"op": "gte", "field": "score", "value": 0.44}, "then": {"literal": "mid"}}
    ], "else": {"literal": "low"}}, "as": "tier"}
  ],
  "mark": {"type": "bar"},
  "encoding": {
    "x": {"field": "region", "type": "nominal"},
    "y": {"field": "score",  "type": "quantitative"},
    "color": {"field": "tier", "type": "nominal"}
  }
}`

// runStructuredTransforms decodes the shared spec, builds a DAG over an
// in-memory FS (hermetic — no .pulse, no disk), executes it with the
// given worker count, and returns the tip table.
func runStructuredTransforms(t *testing.T, workers int) *table.Table {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(structuredTransformSpec))
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	dag, tip, err := build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: workers})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("Execute node errors: %v", res.Errors)
	}
	tbl, ok := res.Tables[tip]
	if !ok || tbl == nil {
		t.Fatal("tip table missing")
	}
	return tbl
}

// TestPrismStructuredFilterCalculateIntegration is the E2-S5 integration
// test: a spec carrying a structured filter + chained structured
// calculate transforms runs end-to-end through plan.Build → plan.Execute
// over inline data and yields the expected output table.
func TestPrismStructuredFilterCalculateIntegration(t *testing.T) {
	tbl := runStructuredTransforms(t, 1)

	if tbl.NumRows() != 3 {
		t.Fatalf("survivors = %d, want 3 (NA, MEA, ANZ)", tbl.NumRows())
	}

	// The derived columns are present alongside the originals.
	for _, name := range []string{"region", "score", "bonus", "bonus_filled", "tier"} {
		if _, ok := tbl.Column(name); !ok {
			t.Errorf("output missing column %q", name)
		}
	}

	regionCol, _ := tbl.Column("region")
	wantRegions := []string{"NA", "MEA", "ANZ"} // input order preserved
	for i, want := range wantRegions {
		if got := regionCol.ValueAt(i).(string); got != want {
			t.Errorf("region row %d = %q, want %q", i, got, want)
		}
	}

	// coalesce: MEA's bonus is null → falls back to 0; NA/ANZ keep theirs.
	bonusFilled, _ := tbl.Column("bonus_filled")
	wantBonus := []float64{5, 0, 2}
	for i, want := range wantBonus {
		if bonusFilled.IsNull(i) {
			t.Errorf("bonus_filled row %d unexpectedly null", i)
			continue
		}
		if got := bonusFilled.ValueAt(i).(float64); got != want {
			t.Errorf("bonus_filled row %d = %v, want %v", i, got, want)
		}
	}

	// case: first branch (high), fall-through branch (mid), else (low).
	tierCol, _ := tbl.Column("tier")
	if tierCol.Kind() != table.KindString {
		t.Fatalf("tier kind = %v, want string", tierCol.Kind())
	}
	wantTier := []string{"low", "high", "mid"} // NA 0.42, MEA 0.66, ANZ 0.45
	for i, want := range wantTier {
		if got := tierCol.ValueAt(i).(string); got != want {
			t.Errorf("tier row %d = %q, want %q", i, got, want)
		}
	}
}

// TestPrismStructuredTransformsWorkerEquivalence confirms the structured
// transform chain yields an identical table hash under serial and
// parallel execution — a race-detector-friendly cross-check of the
// plan-execute path the integration exercises.
func TestPrismStructuredTransformsWorkerEquivalence(t *testing.T) {
	serial := runStructuredTransforms(t, 1)
	parallel := runStructuredTransforms(t, 4)
	if serial.Hash() != parallel.Hash() {
		t.Errorf("table hash differs: serial %s vs parallel %s", serial.Hash(), parallel.Hash())
	}
	if serial.NumRows() != parallel.NumRows() {
		t.Errorf("row count differs: %d vs %d", serial.NumRows(), parallel.NumRows())
	}
}
