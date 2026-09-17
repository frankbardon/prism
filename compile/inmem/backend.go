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

// Compile dispatches one node to the in-memory helper that executes
// it. The switch covers the node kinds that migrated to this backend
// seam — it is NOT a capability list, and a transform's executability
// must never be inferred from it.
//
// A node absent from the switch may execute perfectly well through its
// own Execute body: JoinNode and UnionNode do exactly that
// (plan/nodes/join_execute.go, plan/nodes/union_execute.go), and
// plan.Execute calls node.Execute directly, so they never needed to
// move here. Reading this switch as the set of supported transforms is
// how `join` and `union` came to be described as unimplemented when
// both run fine.
//
// PivotNode is the one node kind that genuinely has no executor, and
// the `pivot` transform behind it is refused at validate with
// PRISM_SPEC_067 before a plan is ever built. The authority on what
// actually runs is internal/gates/transform_executable_sync_test.go,
// which drives every transform end to end through the real planner and
// this backend rather than trusting any stated list — this comment
// included.
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
	case *nodes.TimeUnitNode:
		return executeTimeUnit(ctx, n, ins)
	case *nodes.WindowNode:
		return executeWindow(ctx, n, ins)
	case *nodes.CrosstabNode:
		return executeCrosstab(ctx, n, ins)
	case *nodes.RegressionNode:
		return executeRegression(ctx, n, ins)
	case *nodes.StackNode:
		return executeStack(ctx, n, ins)
	case *nodes.UnpivotNode:
		return executeUnpivot(ctx, n, ins)
	}
	return nil, notImplemented(node)
}

// notImplemented is the PRISM_COMPILE_001 a node raises when it
// reaches this backend and no helper above claims it. It carries the
// concrete node kind for the diagnostic.
//
// The NodeType + Phase context pair is the typed signature
// internal/gates/transform_executable_sync_test.go matches on: it tells
// a genuine missing implementation apart from plan.Execute's codeFor,
// which stamps PRISM_COMPILE_001 on any node error carrying no PRISM_*
// code of its own. Both keys must stay.
func notImplemented(node plan.Node) error {
	kind := fmt.Sprintf("%T", node)
	return prismerrors.New(
		"PRISM_COMPILE_001",
		fmt.Sprintf("Node type %s has no execution implementation.", kind),
		map[string]any{"NodeType": kind, "Phase": "unimplemented"},
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
