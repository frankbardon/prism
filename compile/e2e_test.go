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

// frozenFilteredBrandMean is the per-brand mean(score) Pulse produced
// for the tiny cohort after filtering score > 0.5, captured via
// pulse.Process. See inline_fixture_test.go.
var frozenFilteredBrandMean = map[string]float64{
	"alpha": 0.6202836947132125,
	"beta":  0.6102438646357161,
	"delta": 0.6154473106030667,
	"gamma": 0.6094092974140735,
}

// TestPrismSingleSourceLinearPipeline builds a Source → Filter →
// GroupAggregate DAG over the tiny cohort's inline rows and asserts:
//   - The DAG has 3 nodes including one root and one sink.
//   - The tip output carries one row per surviving brand.
//   - Every avg is in (0.5, 1.0] (filter > 0.5; synth bound 1.0).
//   - The avg values equal what Pulse reported for the same
//     filter+groupby+mean request (frozen, parity preserved).
func TestPrismSingleSourceLinearPipeline(t *testing.T) {
	fs := afero.NewMemMapFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: tinyRef},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: spec.Predicate{Op: spec.PredGt, Field: "score", Value: 0.5}}},
			{Aggregate: &spec.AggregateTransform{
				Groupby:   []string{"brand_id"},
				Aggregate: []spec.AggregateOp{{Op: "mean", Field: "score", As: "avg"}},
			}},
		},
	}

	dag, tipID, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: tinyResolver(t),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Node-count sanity: Source + Filter + GroupAggregate = 3.
	if got := len(dag.Nodes()); got != 3 {
		t.Errorf("DAG node count = %d, want 3 (Source+Filter+GroupAggregate)", got)
	}
	if got := len(dag.Roots()); got != 1 {
		t.Errorf("DAG roots = %d, want 1", got)
	}
	if got := len(dag.Sinks()); got != 1 {
		t.Errorf("DAG sinks = %d, want 1", got)
	}
	if dag.Sinks()[0] != tipID {
		t.Errorf("Sinks[0]=%q want tip=%q", dag.Sinks()[0], tipID)
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

	avgCol, ok := final.Column("avg")
	if !ok {
		t.Fatal("missing avg column")
	}
	for i := 0; i < final.NumRows(); i++ {
		v := avgCol.ValueAt(i).(float64)
		if v <= 0.5 {
			t.Errorf("avg[%d] = %v, expected > 0.5 (filter)", i, v)
		}
		if v > 1.0 {
			t.Errorf("avg[%d] = %v, expected <= 1.0 (synth bound)", i, v)
		}
	}

	// Cross-check against the frozen Pulse output.
	got := tableToBrandValueMap(t, final, "avg")
	if len(got) != len(frozenFilteredBrandMean) {
		t.Errorf("brand count = %d, want %d", len(got), len(frozenFilteredBrandMean))
	}
	for brand, want := range frozenFilteredBrandMean {
		gotVal, ok := got[brand]
		if !ok {
			t.Errorf("brand %q missing from Prism output", brand)
			continue
		}
		if math.Abs(gotVal-want) > 1e-6 {
			t.Errorf("brand %q: Prism=%v frozen(Pulse)=%v (delta=%g)",
				brand, gotVal, want, math.Abs(gotVal-want))
		}
	}
}
