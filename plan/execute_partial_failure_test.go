package plan_test

import (
	"context"
	"runtime"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
)

// TestPrismExecutorPartialFailure pins the design/04-multi-source.md
// partial-failure contract end-to-end with parallel execution:
//
//   - Three Source nodes; source-1 fails with PRISM_RESOLVE_002.
//   - Two sinks: sink-A consumes source-1 only, sink-B consumes
//     source-2 + source-3.
//
// Expected: sink-A skipped, sink-B + source-2 + source-3 succeed;
// exactly one error (source-1, PRISM_RESOLVE_002). Verifies behaviour
// at both workers=1 (sequential) and workers=NumCPU (parallel).
func TestPrismExecutorPartialFailure(t *testing.T) {
	t.Parallel()

	build := func() *plan.DAG {
		b := plan.NewBuilder()
		src1 := &execNode{id: "source-1",
			err: prismerrors.New("PRISM_RESOLVE_002",
				"Local .pulse file /missing.pulse not found on the configured filesystem.",
				map[string]any{"Path": "/missing.pulse"}),
		}
		src2 := &execNode{id: "source-2", tbl: mkTbl(t, "xxh64:src2aaaaaaaaaaa")}
		src3 := &execNode{id: "source-3", tbl: mkTbl(t, "xxh64:src3aaaaaaaaaaa")}
		sinkA := &execNode{id: "sink-A", inputs: []plan.NodeID{"source-1"},
			tbl: mkTbl(t, "xxh64:sinkA")}
		sinkB := &execNode{id: "sink-B", inputs: []plan.NodeID{"source-2", "source-3"},
			tbl: mkTbl(t, "xxh64:sinkB")}
		for _, n := range []plan.Node{src1, src2, src3, sinkA, sinkB} {
			_ = b.AddNode(n)
		}
		_ = b.MarkRoot("source-1")
		_ = b.MarkRoot("source-2")
		_ = b.MarkRoot("source-3")
		_ = b.MarkSink("sink-A")
		_ = b.MarkSink("sink-B")
		d, err := b.Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		return d
	}

	assertPartial := func(t *testing.T, r *plan.ExecResult) {
		t.Helper()
		if _, ok := r.Tables["source-2"]; !ok {
			t.Errorf("source-2 missing from Tables")
		}
		if _, ok := r.Tables["source-3"]; !ok {
			t.Errorf("source-3 missing from Tables")
		}
		if _, ok := r.Tables["sink-B"]; !ok {
			t.Errorf("sink-B missing from Tables")
		}
		if _, ok := r.Tables["source-1"]; ok {
			t.Errorf("source-1 should not be in Tables (it failed)")
		}
		if _, ok := r.Tables["sink-A"]; ok {
			t.Errorf("sink-A should not be in Tables (input failed)")
		}
		if len(r.Errors) != 1 {
			t.Fatalf("Errors=%d; want 1 (source-1 only)", len(r.Errors))
		}
		ne := r.Errors[0]
		if ne.Node != "source-1" {
			t.Errorf("error node=%q; want source-1", ne.Node)
		}
		if ne.Code != "PRISM_RESOLVE_002" {
			t.Errorf("error code=%q; want PRISM_RESOLVE_002", ne.Code)
		}
	}

	t.Run("workers_1", func(t *testing.T) {
		r, err := plan.Execute(context.Background(), build(), plan.ExecOpts{Workers: 1})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		assertPartial(t, r)
	})

	t.Run("workers_NumCPU", func(t *testing.T) {
		r, err := plan.Execute(context.Background(), build(), plan.ExecOpts{Workers: runtime.NumCPU()})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		assertPartial(t, r)
	})
}
