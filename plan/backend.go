package plan

import (
	"context"

	"github.com/frankbardon/prism/table"
)

// Backend is the contract the compiler uses to execute one DAG node
// against its materialised input tables. plan/nodes never call into
// Pulse (or any specific compute engine) directly: they route Execute
// through whichever Backend the builder injected. Concrete impls live
// in compile/ (the in-memory backend ships in P04; future Pulse /
// DuckDB / Arrow backends drop in behind the same interface).
//
// The interface lives in plan/ — not compile/ — because every plan
// node consumes it. Inverting the layering would force plan/nodes to
// import compile/ and risk an import cycle (compile/ already imports
// plan/ for the Node interface). See D032.
//
// Nodes with no injected backend fall back to PRISM_COMPILE_001 to
// preserve P03's stub semantics (see D033 for the injection
// mechanism).
type Backend interface {
	// Compile executes one node against its materialised input tables
	// and returns the resulting output table. ctx propagation is
	// best-effort — impls may honour cancellation between rows or only
	// at op boundaries.
	Compile(ctx context.Context, node Node, ins []*table.Table) (*table.Table, error)
}
