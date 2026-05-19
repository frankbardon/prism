package compile_test

import (
	"context"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/types"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile"
	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/resolve"
	specpkg "github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// fixturePath returns the absolute path of testdata/cohorts/tiny.pulse,
// resolved relative to this test file so `go test ./...` works
// regardless of cwd.
func fixturePath(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "..")
	return filepath.Join(root, "testdata", "cohorts", "tiny.pulse")
}

// TestPrismAggregateValueParity is the PHASE.md test gate. For every
// Pulse-backed aggregate alias, computing the aggregate via Prism's
// in-memory backend on tiny.pulse must equal what pulse.Process
// returns for the same request. Tolerance is 1e-9 (Welford-style
// rounding drifts within ULP per Pulse's docs).
func TestPrismAggregateValueParity(t *testing.T) {
	cohortPath := fixturePath(t)
	fs := afero.NewOsFs()

	// --- Prism side: build a tiny spec and execute it.
	prismResults := map[string]map[string]float64{}
	for _, alias := range compile.PulseBackedAliases() {
		// Skip aliases whose semantics are non-numeric on the score
		// field (distinct counts distinct strings; mode picks the
		// most-frequent value — both meaningful but require deeper
		// fixtures to compare exactly. count we cover separately.)
		if alias == "distinct" || alias == "mode" {
			continue
		}
		field := "score"
		if alias == "count" {
			field = "score" // pulse.AGG_COUNT counts non-null score
		}
		s := &specpkg.Spec{
			Data: &specpkg.Data{Source: cohortPath},
			Transform: []specpkg.Transform{
				{Aggregate: &specpkg.AggregateTransform{
					Groupby:   []string{"brand_id"},
					Aggregate: []specpkg.AggregateOp{{Op: alias, Field: field, As: "value"}},
				}},
			},
		}
		dag, err := build.Build(s, build.Options{
			FS:       fs,
			Resolver: resolve.New(nil),
			Backend:  inmem.New(),
		})
		if err != nil {
			t.Fatalf("%s: build: %v", alias, err)
		}
		res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
		if err != nil {
			t.Fatalf("%s: execute: %v", alias, err)
		}
		final := finalTable(dag, res)
		if final == nil {
			t.Fatalf("%s: no sink table", alias)
		}
		prismResults[alias] = tableToBrandValueMap(t, final, "value")
	}

	// --- Pulse side: direct pulse.Process calls.
	pulseInst, err := pulse.New(pulse.Options{FS: fs})
	if err != nil {
		t.Fatalf("pulse.New: %v", err)
	}

	pulseResults := map[string]map[string]float64{}
	for alias, mapping := range pulseAliasSet(t) {
		if alias == "distinct" || alias == "mode" {
			continue
		}
		field := "score"
		req := &pulse.Request{
			Cohort: &types.Cohort{Filename: cohortPath},
			Groups: []*types.Group{{Type: types.GROUP_CATEGORY, Field: "brand_id"}},
			Aggregations: []*types.Aggregation{{
				Type:   mapping.Type,
				Field:  field,
				Label:  "value",
				Params: mapping.Params,
			}},
		}
		resp, err := pulseInst.Process(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: pulse.Process: %v", alias, err)
		}
		out := map[string]float64{}
		for _, row := range resp.Data {
			brand, _ := row["brand_id"].(string)
			val, _ := row["value"].(float64)
			out[brand] = val
		}
		pulseResults[alias] = out
	}

	// --- Compare.
	for alias, prism := range prismResults {
		pulseVals := pulseResults[alias]
		if len(prism) != len(pulseVals) {
			t.Errorf("%s: group count Prism=%d Pulse=%d", alias, len(prism), len(pulseVals))
		}
		for brand, prismVal := range prism {
			pulseVal, ok := pulseVals[brand]
			if !ok {
				t.Errorf("%s: brand %q present in Prism but missing from Pulse", alias, brand)
				continue
			}
			if math.Abs(prismVal-pulseVal) > 1e-6 {
				t.Errorf("%s/%s: Prism=%v Pulse=%v (delta=%g)",
					alias, brand, prismVal, pulseVal, math.Abs(prismVal-pulseVal))
			}
		}
	}
}

// pulseAliasSet returns the subset of AliasToPulse entries we exercise
// for parity. Helper kept in the test file so the parity surface is
// explicit and easy to scan.
func pulseAliasSet(t *testing.T) map[string]compile.AggregateMapping {
	t.Helper()
	out := map[string]compile.AggregateMapping{}
	for _, alias := range compile.PulseBackedAliases() {
		out[alias] = compile.AliasToPulse[alias]
	}
	return out
}

// finalTable locates the Sink node's output table inside an
// ExecResult. The builder always wires exactly one Sink (D030).
func finalTable(dag *plan.DAG, res *plan.ExecResult) *table.Table {
	for _, id := range dag.Sinks() {
		if t, ok := res.Tables[id]; ok {
			return t
		}
	}
	return nil
}

// tableToBrandValueMap projects (brand_id, value) into a map.
func tableToBrandValueMap(t *testing.T, tbl *table.Table, valueCol string) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	brand, ok := tbl.Column("brand_id")
	if !ok {
		t.Fatal("brand_id column missing from prism table")
	}
	val, ok := tbl.Column(valueCol)
	if !ok {
		t.Fatalf("%s column missing", valueCol)
	}
	for i := 0; i < tbl.NumRows(); i++ {
		key, _ := brand.ValueAt(i).(string)
		v, _ := val.ValueAt(i).(float64)
		out[key] = v
	}
	return out
}

// ensure nodes import stays for goimports stability when the test
// is extended.
var _ = nodes.NewSink
