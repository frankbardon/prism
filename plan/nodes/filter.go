package nodes

import (
	"context"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// FilterNode applies a Pulse expression predicate to its input table.
// Execute routes through the injected backend; falls back to
// PRISM_COMPILE_001 when no backend is wired (preserves P03 stub
// behaviour for callers that haven't migrated). See D033.
type FilterNode struct {
	id      plan.NodeID
	input   plan.NodeID
	expr    string
	backend plan.Backend
}

// NewFilter constructs a FilterNode with a stable id derived from
// (input, expr) so two equivalent filters share a fingerprint.
func NewFilter(id, input plan.NodeID, expr string) *FilterNode {
	return &FilterNode{id: id, input: input, expr: expr}
}

// ID implements plan.Node.
func (n *FilterNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *FilterNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Filter does not change the schema.
func (n *FilterNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	return requireSingleInput("FilterNode", in)
}

// Execute implements plan.Node. Routes through the injected backend
// when one is wired; returns PRISM_COMPILE_001 otherwise.
func (n *FilterNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("FilterNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute. The
// builder calls this after construction so node constructors keep
// their P03 signatures stable. See D033.
func (n *FilterNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *FilterNode) Fingerprint() string {
	return fingerprintFor("FilterNode", string(n.input), n.expr)
}

// Expr exposes the predicate string for renderers + tests.
func (n *FilterNode) Expr() string { return n.expr }

// Kind implements plan.Labeled.
func (n *FilterNode) Kind() string { return "FilterNode" }

// Summary implements plan.Labeled.
func (n *FilterNode) Summary() string { return "expr: " + n.expr }
