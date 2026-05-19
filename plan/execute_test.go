package plan_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// execNode is the in-test plan.Node with a custom Execute hook so the
// executor tests can stage real return values / errors / blocks without
// depending on plan/nodes (the cycle problem).
type execNode struct {
	id     plan.NodeID
	inputs []plan.NodeID
	tbl    *table.Table
	err    error
	delay  time.Duration
	calls  *int
	mu     *sync.Mutex
}

func (n *execNode) ID() plan.NodeID       { return n.id }
func (n *execNode) Inputs() []plan.NodeID { return n.inputs }
func (n *execNode) Fingerprint() string   { return "exec:" + string(n.id) }
func (n *execNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	return &encoding.Schema{Fields: []encoding.Field{{Name: "v", Type: encoding.FieldTypeF64}}}, nil
}
func (n *execNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if n.delay > 0 {
		select {
		case <-time.After(n.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if n.calls != nil {
		n.mu.Lock()
		*n.calls++
		n.mu.Unlock()
	}
	return n.tbl, n.err
}

func mkTbl(t *testing.T, hash string) *table.Table {
	t.Helper()
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "v", Type: encoding.FieldTypeF64},
	}}
	cols := map[string]table.Column{"v": table.FloatColumn{1, 2, 3}}
	tbl, err := table.NewTable(schema, cols, 3, hash)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// TestPrismExecutorSequential runs a Source (real table) → Filter
// (returns PRISM_COMPILE_001) → Sink mini-DAG and asserts the
// partial-failure semantics described in design/05-dag-executor.md.
func TestPrismExecutorSequential(t *testing.T) {
	srcTbl := mkTbl(t, "xxh64:0000000000000001")
	src := &execNode{id: "src", tbl: srcTbl}
	filt := &execNode{id: "filter", inputs: []plan.NodeID{"src"},
		err: prismerrors.New("PRISM_COMPILE_001", "not implemented: FilterNode",
			map[string]any{"NodeType": "FilterNode", "Phase": "P04"})}
	sink := &execNode{id: "sink", inputs: []plan.NodeID{"filter"}, tbl: srcTbl}

	b := plan.NewBuilder()
	for _, n := range []plan.Node{src, filt, sink} {
		if err := b.AddNode(n); err != nil {
			t.Fatalf("AddNode: %v", err)
		}
	}
	_ = b.MarkRoot("src")
	_ = b.MarkSink("sink")
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	result, err := plan.Execute(context.Background(), d, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Tables) != 1 {
		t.Errorf("Tables len=%d, want 1 (only src succeeds)", len(result.Tables))
	}
	if _, ok := result.Tables["src"]; !ok {
		t.Error("src table missing from result")
	}
	if _, ok := result.Tables["filter"]; ok {
		t.Error("filter should have failed")
	}
	if _, ok := result.Tables["sink"]; ok {
		t.Error("sink should have been skipped (input failed)")
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors len=%d, want 1", len(result.Errors))
	}
	if result.Errors[0].Code != "PRISM_COMPILE_001" {
		t.Errorf("Errors[0].Code=%q", result.Errors[0].Code)
	}
}

func TestPrismExecutorRespectsAbortOnError(t *testing.T) {
	srcTbl := mkTbl(t, "xxh64:0000000000000001")
	src := &execNode{id: "src", tbl: srcTbl}
	filt := &execNode{id: "filter", inputs: []plan.NodeID{"src"},
		err: prismerrors.New("PRISM_COMPILE_001", "stub", nil)}
	other := &execNode{id: "limit", inputs: []plan.NodeID{"src"}, tbl: srcTbl}

	b := plan.NewBuilder()
	for _, n := range []plan.Node{src, filt, other} {
		if err := b.AddNode(n); err != nil {
			t.Fatalf("AddNode: %v", err)
		}
	}
	_ = b.MarkRoot("src")
	_ = b.MarkSink("filter")
	_ = b.MarkSink("limit")
	d, _ := b.Build()

	_, err := plan.Execute(context.Background(), d, plan.ExecOpts{Workers: 1, AbortOnError: true})
	if err == nil {
		t.Fatal("expected NodeError, got nil")
	}
	var ne *plan.NodeError
	if !errors.As(err, &ne) {
		t.Fatalf("expected *NodeError, got %T", err)
	}
}

func TestPrismExecutorPerNodeTimeout(t *testing.T) {
	src := &execNode{id: "src", tbl: mkTbl(t, "x"), delay: 200 * time.Millisecond}
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot("src")
	_ = b.MarkSink("src")
	d, _ := b.Build()

	result, err := plan.Execute(context.Background(), d, plan.ExecOpts{
		Workers: 1, PerNodeTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors=%v", result.Errors)
	}
	if !errors.Is(result.Errors[0].Err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", result.Errors[0].Err)
	}
}

// TestPrismExecutorWorkersEquivalence runs the same multi-source
// fan-in DAG with workers=1 and workers=NumCPU and asserts identical
// table hashes. Race-detector friendly: every goroutine touches result
// only under the mutex the executor manages.
func TestPrismExecutorWorkersEquivalence(t *testing.T) {
	build := func() *plan.DAG {
		b := plan.NewBuilder()
		for i, hash := range []string{"a", "b", "c", "d"} {
			id := plan.NodeID("src" + string(rune('0'+i)))
			_ = b.AddNode(&execNode{id: id, tbl: mkTbl(t, hash)})
			_ = b.MarkRoot(id)
		}
		_ = b.AddNode(&execNode{
			id: "sink", inputs: []plan.NodeID{"src0", "src1", "src2", "src3"},
			tbl: mkTbl(t, "joined"),
		})
		_ = b.MarkSink("sink")
		d, err := b.Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		return d
	}

	d1 := build()
	r1, err := plan.Execute(context.Background(), d1, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("Execute(1): %v", err)
	}
	d2 := build()
	r2, err := plan.Execute(context.Background(), d2, plan.ExecOpts{Workers: runtime.NumCPU()})
	if err != nil {
		t.Fatalf("Execute(NumCPU): %v", err)
	}

	if len(r1.Tables) != len(r2.Tables) {
		t.Fatalf("table counts differ: %d vs %d", len(r1.Tables), len(r2.Tables))
	}
	for id, tbl1 := range r1.Tables {
		tbl2, ok := r2.Tables[id]
		if !ok {
			t.Fatalf("table %s missing under NumCPU run", id)
		}
		if tbl1.Hash() != tbl2.Hash() {
			t.Errorf("table %s hash differs: %s vs %s", id, tbl1.Hash(), tbl2.Hash())
		}
	}
}

func TestPrismExecutorCallbacks(t *testing.T) {
	src := &execNode{id: "src", tbl: mkTbl(t, "x")}
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot("src")
	_ = b.MarkSink("src")
	d, _ := b.Build()

	var starts, dones int
	var mu sync.Mutex
	_, err := plan.Execute(context.Background(), d, plan.ExecOpts{
		Workers: 1,
		OnNodeStart: func(id plan.NodeID) {
			mu.Lock()
			starts++
			mu.Unlock()
		},
		OnNodeDone: func(id plan.NodeID, _ time.Duration, _ error) {
			mu.Lock()
			dones++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if starts != 1 || dones != 1 {
		t.Fatalf("starts=%d dones=%d, want 1/1", starts, dones)
	}
}
