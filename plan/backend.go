package plan

import (
	"context"

	"github.com/frankbardon/prism/table"
)

// Backend is the contract the compiler uses to execute one DAG node
// against its materialised input tables. plan/nodes never call into
// Pulse (or any specific compute engine) directly: they route Execute
// through whichever Backend the builder injected. Concrete impls live
// in compile/ (compile/inmem is the in-memory backend every build
// wires by default; other backends drop in behind the same
// interface).
//
// The interface lives in plan/ — not compile/ — because every plan
// node consumes it. Inverting the layering would force plan/nodes to
// import compile/ and risk an import cycle (compile/ already imports
// plan/ for the Node interface). See D032.
//
// A backend-routed node with no injected backend falls back to
// PRISM_COMPILE_001 (see D033 for the injection mechanism). Nodes that
// carry their own Execute body — JoinNode, UnionNode — never consult a
// backend at all.
type Backend interface {
	// Compile executes one node against its materialised input tables
	// and returns the resulting output table. ctx propagation is
	// best-effort — impls may honour cancellation between rows or only
	// at op boundaries.
	Compile(ctx context.Context, node Node, ins []*table.Table) (*table.Table, error)
}
