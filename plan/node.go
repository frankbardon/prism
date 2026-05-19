// Package plan holds Prism's logical DAG: the Node interface every node
// type satisfies, the immutable DAG type with structural-sharing
// optimizer passes, the sequential / bounded-pool executor, and the
// DOT / text / JSON renderers used by `prism plan`.
//
// P02 shipped only the minimal Node shape needed by SourceNode (no
// Schema method) and an optional SchemaProbe capability. P03 widens
// Node to the canonical interface from design/05-dag-executor.md so
// every node — including the twelve P03 stubs — can declare its
// output schema without executing. SourceNode satisfies the new
// Schema(in) hook by delegating to its existing OutputSchema().
//
// Decision: see D028 — Node.Schema(in) is required, not optional.
package plan

import (
	"context"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/table"
)

// NodeID is the stable identifier for one DAG node. Implementations
// choose the format (path basename, sha hash, monotonic counter); the
// executor only needs equality semantics. IDs are case-sensitive and
// must be unique within a single DAG.
type NodeID string

// Node is the contract every DAG node satisfies. It is the only
// interface the executor knows about: schedulers, optimizer passes,
// renderers, and the cache key builder all consume Nodes through this
// surface.
//
// Schema(in) lets callers reason about a node's output shape without
// executing — required for DAG visualisation, optimizer-pass
// eligibility, and the stubbed P03 nodes whose Execute bodies return
// PRISM_COMPILE_001 until P04. The `in` slice carries upstream
// schemas in declaration order; nodes that ignore inputs (Source)
// pass nil.
//
// Fingerprint is a deterministic string capturing this node's identity
// (op + parameters). The cache key builder combines it with each
// input Table's Hash() to form a content-addressable cache key.
type Node interface {
	// ID returns the stable identifier for this node.
	ID() NodeID
	// Inputs returns the upstream NodeIDs this node depends on. A
	// SourceNode returns nil (it has no upstream).
	Inputs() []NodeID
	// Schema returns the node's output schema given its inputs'
	// schemas. Nodes that cannot compute their schema without
	// execution data (rare; Pivot is the only P03 example) return
	// the first input schema and document the gap.
	Schema(in []*encoding.Schema) (*encoding.Schema, error)
	// Execute runs the node and returns the materialised Table. The
	// `in` slice carries upstream Tables in declaration order; for a
	// SourceNode it is always nil. Stubbed P03 nodes return
	// PRISM_COMPILE_001 here.
	Execute(ctx context.Context, in []*table.Table) (*table.Table, error)
	// Fingerprint returns a deterministic string capturing this node's
	// identity (op + parameters) used as a cache key component.
	Fingerprint() string
}
