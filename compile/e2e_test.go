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

// TestPrismSingleSourceLinearPipeline is the PHASE.md demoable
// artifact, run as a test. Builds a Source → Filter → GroupAggregate
// → Sink DAG against testdata/cohorts/tiny.pulse and asserts:
//   - The DAG has 4 nodes including one Source root and one Sink.
//   - The Sink output table carries one row per surviving brand.
//   - Every avg is in [0.5, 1.0] (filter > 0.5; synth bound 1.0).
//   - The avg values equal what pulse.Process reports for the same
//     filter+groupby+mean request to within 1e-6.
func TestPrismSingleSourceLinearPipeline(t *testing.T) {
	cohortPath := fixturePath(t)
	fs := afero.NewOsFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: cohortPath},
		Transform: []spec.Transform{
			{Filter: &spec.FilterTransform{Filter: "score > 0.5"}},
			{Aggregate: &spec.AggregateTransform{
				Groupby:   []string{"brand_id"},
				Aggregate: []spec.AggregateOp{{Op: "mean", Field: "score", As: "avg"}},
			}},
		},
	}

	dag, tipID, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Node-count sanity: Source + Filter + GroupAggregate = 3 (SinkNode
	// retired in P05 per D040; the tip is now the GroupAggregate itself).
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
			t.Errorf("avg[%d] = %v, expected ≤ 1.0 (synth bound)", i, v)
		}
	}

	// Cross-check against pulse.Process direct.
	prism := tableToBrandValueMap(t, final, "avg")

	pulseInst, err := pulse.New(pulse.Options{FS: fs})
	if err != nil {
		t.Fatalf("pulse.New: %v", err)
	}
	resp, err := pulseInst.Process(context.Background(), &pulse.Request{
		Cohort: &types.Cohort{Filename: cohortPath},
		Filterers: []*types.Filterer{{
			Type:       types.FILTER_EXPRESSION,
			Expression: "score > 0.5",
		}},
		Groups: []*types.Group{{Type: types.GROUP_CATEGORY, Field: "brand_id"}},
		Aggregations: []*types.Aggregation{{
			Type:  types.AGG_AVERAGE,
			Field: "score",
			Label: "avg",
		}},
	})
	if err != nil {
		t.Fatalf("pulse.Process: %v", err)
	}

	for _, row := range resp.Data {
		brand, _ := row["brand_id"].(string)
		pulseVal, _ := row["avg"].(float64)
		prismVal, ok := prism[brand]
		if !ok {
			t.Errorf("brand %q present in Pulse output but missing from Prism", brand)
			continue
		}
		if math.Abs(prismVal-pulseVal) > 1e-6 {
			t.Errorf("brand %q: Prism=%v Pulse=%v (delta=%g)",
				brand, prismVal, pulseVal, math.Abs(prismVal-pulseVal))
		}
	}
}
