package plan

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/table"
)

// ExecOpts controls one Execute call. Zero values are sensible:
// Workers=0 consults PRISM_QUERY_WORKERS env, then falls back to
// runtime.NumCPU(); Cache=nil disables memoization; PerNodeTimeout=0
// disables per-node timeouts; AbortOnError=false is partial-failure
// mode.
type ExecOpts struct {
	// Workers is the upper bound on goroutines per level. 0 = consult
	// PRISM_QUERY_WORKERS (set positive) else runtime.NumCPU(); 1 =
	// strictly sequential.
	Workers        int
	Cache          TableCache
	JoinMaxRows    int
	PerNodeTimeout time.Duration
	AbortOnError   bool
	OnNodeStart    func(NodeID)
	OnNodeDone     func(NodeID, time.Duration, error)
}

// ExecResult carries the per-node tables that successfully
// materialised plus per-failed-node errors.
type ExecResult struct {
	Tables  map[NodeID]*table.Table
	Errors  []NodeError
	Elapsed time.Duration
}

// NodeError is the per-failed-node detail entry.
type NodeError struct {
	Node NodeID
	Code string
	Err  error
}

// Error implements the error interface.
func (e *NodeError) Error() string {
	return fmt.Sprintf("node %s: %s: %v", e.Node, e.Code, e.Err)
}

// Unwrap returns the inner error so errors.As / errors.Is work.
func (e *NodeError) Unwrap() error { return e.Err }

// Execute runs d through the executor with the given options.
//
// Workers resolution order (P07):
//  1. ExecOpts.Workers > 0 → use as-is.
//  2. ExecOpts.Workers == 0 AND PRISM_QUERY_WORKERS > 0 → env wins.
//  3. Otherwise → runtime.NumCPU().
//
// Workers == 1 is the sequential path (P03 contract).
//
// Partial-failure policy (D006): a node whose Execute returns an
// error leaves its dependents un-runnable (the inputsReady check
// detects the missing table); sibling paths continue. AbortOnError
// flips to fail-fast: returns the first NodeError immediately.
//
// ctx cancellation is honoured at level boundaries.
func Execute(ctx context.Context, d *DAG, opts ExecOpts) (*ExecResult, error) {
	if d == nil {
		return nil, fmt.Errorf("plan.Execute: nil DAG")
	}
	levels, err := d.TopoLevels()
	if err != nil {
		return nil, err
	}

	workers := resolveWorkers(opts.Workers)

	result := &ExecResult{Tables: map[NodeID]*table.Table{}}
	var mu sync.Mutex
	start := time.Now()

	for _, level := range levels {
		if err := ctx.Err(); err != nil {
			result.Elapsed = time.Since(start)
			return result, err
		}
		if err := runLevel(ctx, d, level, workers, opts, result, &mu); err != nil {
			result.Elapsed = time.Since(start)
			return result, err
		}
		if opts.AbortOnError {
			mu.Lock()
			abort := len(result.Errors) > 0
			var first NodeError
			if abort {
				first = result.Errors[0]
			}
			mu.Unlock()
			if abort {
				result.Elapsed = time.Since(start)
				return result, &first
			}
		}
	}
	result.Elapsed = time.Since(start)
	return result, nil
}

// runLevel fans out the nodes in one level across at most `workers`
// goroutines and waits for completion. The sequential path
// (workers==1) still uses a goroutine + semaphore so the code path
// is identical at every concurrency level — easier to reason about
// than two implementations.
func runLevel(
	ctx context.Context, d *DAG, level []NodeID,
	workers int, opts ExecOpts, result *ExecResult, mu *sync.Mutex,
) error {
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for _, id := range level {
		// Skip nodes whose inputs failed in earlier levels.
		if !inputsReady(d, id, result, mu) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(id NodeID) {
			defer wg.Done()
			defer func() { <-sem }()
			runOne(ctx, d, id, opts, result, mu)
		}(id)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// runOne executes a single node. Caller has confirmed inputs are
// present. Results / errors are appended to result under mu.
func runOne(
	ctx context.Context, d *DAG, id NodeID,
	opts ExecOpts, result *ExecResult, mu *sync.Mutex,
) {
	node, ok := d.Node(id)
	if !ok {
		mu.Lock()
		result.Errors = append(result.Errors, NodeError{
			Node: id, Code: "PRISM_PLAN_003",
			Err: fmt.Errorf("node %s not in DAG", id),
		})
		mu.Unlock()
		return
	}

	mu.Lock()
	ins := make([]*table.Table, len(node.Inputs()))
	ok = true
	for i, in := range node.Inputs() {
		t, present := result.Tables[in]
		if !present {
			ok = false
			break
		}
		ins[i] = t
	}
	mu.Unlock()
	if !ok {
		// Inputs not actually ready (e.g. failed concurrently). Skip.
		return
	}

	// Cache check.
	var key string
	if opts.Cache != nil {
		key = CacheKey(node, ins)
		if cached, hit := opts.Cache.Get(key); hit {
			mu.Lock()
			result.Tables[id] = cached
			mu.Unlock()
			return
		}
	}

	nodeCtx := ctx
	var cancel context.CancelFunc
	if opts.PerNodeTimeout > 0 {
		nodeCtx, cancel = context.WithTimeout(ctx, opts.PerNodeTimeout)
		defer cancel()
	}

	if opts.OnNodeStart != nil {
		opts.OnNodeStart(id)
	}
	startedAt := time.Now()
	out, err := node.Execute(nodeCtx, ins)
	elapsed := time.Since(startedAt)
	if opts.OnNodeDone != nil {
		opts.OnNodeDone(id, elapsed, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if err != nil {
		result.Errors = append(result.Errors, NodeError{
			Node: id, Code: codeFor(err), Err: err,
		})
		return
	}
	result.Tables[id] = out
	if opts.Cache != nil && key != "" {
		opts.Cache.Put(key, out)
	}
}

// inputsReady reports whether every input id for node id has a
// materialised table in result. Locked read so concurrent siblings
// see consistent state.
func inputsReady(d *DAG, id NodeID, result *ExecResult, mu *sync.Mutex) bool {
	n, ok := d.Node(id)
	if !ok {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	for _, in := range n.Inputs() {
		if _, present := result.Tables[in]; !present {
			return false
		}
	}
	return true
}

// resolveWorkers implements the three-step precedence documented on
// Execute: explicit opt value wins; sentinel 0 consults the env; final
// fallback is NumCPU. The function is kept package-private so tests
// observe the same precedence through the public Execute entry.
func resolveWorkers(opt int) int {
	if opt > 0 {
		return opt
	}
	if env, ok := limits.QueryWorkers(); ok && env > 0 {
		return env
	}
	return runtime.NumCPU()
}

// codeFor extracts the PRISM_* code from an error. AppError carries it
// directly; other errors get a generic catch-all so the NodeError still
// has a usable code field. context.DeadlineExceeded surfaces as the
// canonical timeout sentinel for callers that want to special-case it.
func codeFor(err error) string {
	if err == nil {
		return ""
	}
	var ae *prismerrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "PRISM_PLAN_TIMEOUT"
	}
	return "PRISM_COMPILE_001"
}
