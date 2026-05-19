// Package plan holds the logical DAG types used between Validate and
// Compile. P02 ships only the minimal Node shape needed for SourceNode;
// the full DAG, topo sort, optimizer passes, and executor land in P03.
//
// See design/05-dag-executor.md for the target surface.
package plan

import (
	"context"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/table"
)

// NodeID is the stable identifier for one DAG node. Implementations
// choose the format (path basename, sha hash, monotonic counter); the
// executor only needs equality semantics.
type NodeID string

// Node is the contract every DAG node satisfies. P02 widens this in
// P03 to add Schema(in []*encoding.Schema) (*encoding.Schema, error)
// for ahead-of-execute schema reasoning; for now SourceNode does not
// need that hook because it discovers schema during Resolve.
type Node interface {
	// ID returns the stable identifier for this node.
	ID() NodeID
	// Inputs returns the upstream NodeIDs this node depends on. A
	// SourceNode returns nil (it has no upstream).
	Inputs() []NodeID
	// Execute runs the node and returns the materialised Table. The
	// `in` slice carries upstream Tables in declaration order; for a
	// SourceNode it is always nil.
	Execute(ctx context.Context, in []*table.Table) (*table.Table, error)
	// Fingerprint returns a deterministic string capturing this node's
	// identity (op + parameters) used as a cache key component.
	Fingerprint() string
}

// SchemaProbe is an optional capability — nodes that can report their
// output schema without executing implement it so callers (validate,
// predict, plan visualisation) can reason ahead of materialisation.
// SourceNode satisfies SchemaProbe by resolving the ref and returning
// the Pulse schema unchanged.
type SchemaProbe interface {
	OutputSchema() (*encoding.Schema, error)
}
