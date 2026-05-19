// Package inmem is the default P04 implementation of plan.Backend.
// Every supported node kind is dispatched to a per-op helper that
// operates over the columnar *table.Table directly — no Pulse facade
// calls. The aggregate alias map (compile/aggregates.go) is the
// single source of truth for op naming.
//
// Pulse v0.8.4 exposes a request-based facade (pulse.Process); it has
// no public in-memory cohort constructor, so feeding intermediate
// tables back into Pulse is not possible in v1. In-memory execution
// here produces values byte-equal to what pulse.Process would
// compute against the source cohort — proven by
// TestPrismAggregateValueParity. See D035.
package inmem

import (
	"context"
	"fmt"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// Backend is the in-memory implementation of plan.Backend. It is
// safe for concurrent use across goroutines (no per-Compile state
// lives on the struct).
type Backend struct{}

// New returns the singleton in-memory backend. Callers can pass it
// to plan/build.Options.Backend or hold a pointer; equality semantics
// do not matter.
func New() *Backend { return &Backend{} }

// Compile dispatches one node to its per-op helper. Unsupported node
// kinds (Join, Union, Pivot, Unpivot — deferred to P07/P09/P10)
// return PRISM_COMPILE_001 so behaviour matches the P03 stubs.
func (b *Backend) Compile(ctx context.Context, node plan.Node, ins []*table.Table) (*table.Table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch n := node.(type) {
	case *nodes.FilterNode:
		return executeFilter(ctx, n, ins)
	case *nodes.ProjectNode:
		return executeProject(ctx, n, ins)
	case *nodes.SortNode:
		return executeSort(ctx, n, ins)
	case *nodes.LimitNode:
		return executeLimit(ctx, n, ins)
	case *nodes.SampleNode:
		return executeSample(ctx, n, ins)
	case *nodes.CalculateNode:
		return executeCalculate(ctx, n, ins)
	case *nodes.GroupAggregateNode:
		return executeGroupAggregate(ctx, n, ins)
	case *nodes.BinNode:
		return executeBin(ctx, n, ins)
	case *nodes.WindowNode:
		return executeWindow(ctx, n, ins)
	}
	return nil, notImplemented(node)
}

// notImplemented mirrors the PRISM_COMPILE_001 shape the P03 stubs
// emit so backend-routed calls fail the same way until a real impl
// lands. Carries the concrete node kind for the diagnostic.
func notImplemented(node plan.Node) error {
	kind := fmt.Sprintf("%T", node)
	return prismerrors.New(
		"PRISM_COMPILE_001",
		fmt.Sprintf("Node type %s is not implemented yet (lands in a later phase).", kind),
		map[string]any{"NodeType": kind, "Phase": "P07+"},
	)
}

// requireOneInput is the dispatch-time guard every single-input op
// uses. Returns the sole input table or an error if the shape is
// wrong.
func requireOneInput(node plan.Node, ins []*table.Table) (*table.Table, error) {
	if len(ins) != 1 || ins[0] == nil {
		return nil, fmt.Errorf("inmem: node %s expected exactly one input table, got %d", node.ID(), len(ins))
	}
	return ins[0], nil
}
