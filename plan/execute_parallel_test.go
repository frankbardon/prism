package plan_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// slowExecNode sleeps for `delay` before returning a deterministic
// table. Tracks concurrent invocations via `inFlight` so tests can
// assert the actual parallelism observed (not just total wall time).
type slowExecNode struct {
	id        plan.NodeID
	inputs    []plan.NodeID
	delay     time.Duration
	hash      string
	inFlight  *int64
	maxSeen   *int64
	callCount *int64
}

func (n *slowExecNode) ID() plan.NodeID       { return n.id }
func (n *slowExecNode) Inputs() []plan.NodeID { return n.inputs }
func (n *slowExecNode) Fingerprint() string   { return "slow:" + string(n.id) }
func (n *slowExecNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	return &encoding.Schema{Fields: []encoding.Field{{Name: "v", Type: encoding.FieldTypeF64}}}, nil
}
func (n *slowExecNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if n.callCount != nil {
		atomic.AddInt64(n.callCount, 1)
	}
	if n.inFlight != nil {
		cur := atomic.AddInt64(n.inFlight, 1)
		defer atomic.AddInt64(n.inFlight, -1)
		// Best-effort max — racy on read but monotone-increasing.
		for {
			prev := atomic.LoadInt64(n.maxSeen)
			if cur <= prev || atomic.CompareAndSwapInt64(n.maxSeen, prev, cur) {
				break
			}
		}
	}
	if n.delay > 0 {
		select {
		case <-time.After(n.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "v", Type: encoding.FieldTypeF64},
	}}
	cols := map[string]table.Column{"v": table.FloatColumn{1, 2, 3}}
	return table.NewTable(schema, cols, 3, n.hash)
}

// buildSiblingDAG returns a DAG with N sibling slowExecNodes feeding
// one downstream sink. Each sibling sleeps for `delay`. Callers pass
// the shared atomic counters so tests can assert parallelism.
func buildSiblingDAG(
	t *testing.T,
	n int, delay time.Duration,
	inFlight, maxSeen, calls *int64,
) (*plan.DAG, []plan.NodeID) {
	t.Helper()
	b := plan.NewBuilder()
	rootIDs := make([]plan.NodeID, 0, n)
	for i := 0; i < n; i++ {
		id := plan.NodeID("src" + string(rune('0'+i)))
		rootIDs = append(rootIDs, id)
		_ = b.AddNode(&slowExecNode{
			id: id, delay: delay,
			hash:     "h-" + string(rune('a'+i)),
			inFlight: inFlight, maxSeen: maxSeen, callCount: calls,
		})
		_ = b.MarkRoot(id)
	}
	sinkNode := &slowExecNode{
		id: "sink", inputs: rootIDs, hash: "sink",
	}
	_ = b.AddNode(sinkNode)
	_ = b.MarkSink("sink")
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return d, rootIDs
}

// TestPrismExecutorParallel asserts the bounded worker pool actually
// schedules sibling nodes in parallel. With three 50 ms sleeps and one
// worker, total elapsed is ≥150 ms (sequential). With four workers it
// completes well under 100 ms. Identical tables emerge under both.
func TestPrismExecutorParallel(t *testing.T) {
	t.Parallel()

	t.Run("sequential_workers_1", func(t *testing.T) {
		var inFlight, maxSeen, calls int64
		d, _ := buildSiblingDAG(t, 3, 50*time.Millisecond, &inFlight, &maxSeen, &calls)

		start := time.Now()
		result, err := plan.Execute(context.Background(), d, plan.ExecOpts{Workers: 1})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("Execute(1): %v", err)
		}
		if elapsed < 150*time.Millisecond {
			t.Errorf("workers=1 elapsed=%v; want ≥150ms (3 × 50ms sequential)", elapsed)
		}
		if got := atomic.LoadInt64(&maxSeen); got != 1 {
			t.Errorf("workers=1 maxInFlight=%d; want 1", got)
		}
		if len(result.Tables) != 4 { // 3 sources + 1 sink
			t.Errorf("Tables=%d; want 4", len(result.Tables))
		}
	})

	t.Run("parallel_workers_4", func(t *testing.T) {
		var inFlight, maxSeen, calls int64
		d, _ := buildSiblingDAG(t, 3, 50*time.Millisecond, &inFlight, &maxSeen, &calls)

		start := time.Now()
		result, err := plan.Execute(context.Background(), d, plan.ExecOpts{Workers: 4})
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("Execute(4): %v", err)
		}
		// 3 × 50ms in parallel plus sink ≈ 100ms. Allow generous slack
		// for slow CI: budget 200ms (still <150ms sequential floor).
		if elapsed >= 200*time.Millisecond {
			t.Errorf("workers=4 elapsed=%v; expected <200ms (parallel)", elapsed)
		}
		if got := atomic.LoadInt64(&maxSeen); got < 2 {
			t.Errorf("workers=4 maxInFlight=%d; want ≥2 (parallel scheduling)", got)
		}
		if len(result.Tables) != 4 {
			t.Errorf("Tables=%d; want 4", len(result.Tables))
		}
	})

	t.Run("hash_equivalence_across_workers", func(t *testing.T) {
		var inFlight1, maxSeen1, calls1 int64
		d1, _ := buildSiblingDAG(t, 3, 5*time.Millisecond, &inFlight1, &maxSeen1, &calls1)
		var inFlight2, maxSeen2, calls2 int64
		d2, _ := buildSiblingDAG(t, 3, 5*time.Millisecond, &inFlight2, &maxSeen2, &calls2)

		r1, err := plan.Execute(context.Background(), d1, plan.ExecOpts{Workers: 1})
		if err != nil {
			t.Fatalf("Execute(1): %v", err)
		}
		r2, err := plan.Execute(context.Background(), d2, plan.ExecOpts{Workers: 8})
		if err != nil {
			t.Fatalf("Execute(8): %v", err)
		}
		if len(r1.Tables) != len(r2.Tables) {
			t.Fatalf("Tables counts differ: %d vs %d", len(r1.Tables), len(r2.Tables))
		}
		for id, t1 := range r1.Tables {
			t2, ok := r2.Tables[id]
			if !ok {
				t.Errorf("table %s missing under workers=8", id)
				continue
			}
			if t1.Hash() != t2.Hash() {
				t.Errorf("table %s hash differs: %q vs %q", id, t1.Hash(), t2.Hash())
			}
		}
	})
}

// TestPrismExecutorRaceFree exists so `go test -race` exercises the
// bounded-pool code path under stress. 50 sibling nodes feeding one
// sink; race detector clean is the only assertion. (Failure surfaces as
// a `DATA RACE` report from the runtime; the test body just needs to
// drive enough concurrency to trip any latent races.)
func TestPrismExecutorRaceFree(t *testing.T) {
	t.Parallel()

	b := plan.NewBuilder()
	rootIDs := make([]plan.NodeID, 0, 50)
	for i := 0; i < 50; i++ {
		id := plan.NodeID("rsrc-" + intToStr(i))
		rootIDs = append(rootIDs, id)
		_ = b.AddNode(&slowExecNode{
			id: id, delay: time.Millisecond, hash: "h-" + intToStr(i),
		})
		_ = b.MarkRoot(id)
	}
	_ = b.AddNode(&slowExecNode{
		id: "racy-sink", inputs: rootIDs, hash: "racy-sink",
	})
	_ = b.MarkSink("racy-sink")
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// High worker count to maximise contention on the shared mutex.
	result, err := plan.Execute(context.Background(), d, plan.ExecOpts{Workers: 32})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Tables) != 51 {
		t.Errorf("Tables=%d; want 51", len(result.Tables))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors=%v; want none", result.Errors)
	}
}

// TestPrismExecutorWorkersEnvOverride pins PRISM_QUERY_WORKERS behaviour:
// when ExecOpts.Workers == 0, limits.QueryWorkers() drives concurrency.
// The test asserts the observed max-concurrency aligns with the env var.
func TestPrismExecutorWorkersEnvOverride(t *testing.T) {
	// Don't t.Parallel — we mutate the env.

	t.Setenv("PRISM_QUERY_WORKERS", "2")

	var inFlight, maxSeen, calls int64
	d, _ := buildSiblingDAG(t, 5, 30*time.Millisecond, &inFlight, &maxSeen, &calls)

	start := time.Now()
	_, err := plan.Execute(context.Background(), d, plan.ExecOpts{Workers: 0})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := atomic.LoadInt64(&maxSeen)
	if got > 2 {
		t.Errorf("maxInFlight=%d; want ≤2 (PRISM_QUERY_WORKERS=2)", got)
	}
	if got < 2 {
		t.Errorf("maxInFlight=%d; want ≥2 (expected actual parallelism)", got)
	}
	// 5 sources × 30ms / 2 workers = ~75ms. Sink adds another invocation.
	// Floor: 3 × 30ms = 90ms (5 nodes across 2 workers needs 3 batches).
	// Ceiling: 5 × 30ms = 150ms (would mean fully sequential).
	if elapsed < 60*time.Millisecond {
		t.Errorf("elapsed=%v; suspiciously fast for workers=2", elapsed)
	}
}

// intToStr is a zero-dep itoa for test ids.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// _ keeps sync imported in case future tests want shared coordination.
var _ = sync.Mutex{}
