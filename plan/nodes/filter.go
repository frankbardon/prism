package nodes

import (
	"context"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// FilterNode applies a Pulse expression predicate to its input table.
// P03 stub — Execute returns PRISM_COMPILE_001. P04 wires real impl.
type FilterNode struct {
	id    plan.NodeID
	input plan.NodeID
	expr  string
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

// Execute implements plan.Node. P03 stub.
func (n *FilterNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("FilterNode")
}

// Fingerprint implements plan.Node.
func (n *FilterNode) Fingerprint() string {
	return fingerprintFor("FilterNode", string(n.input), n.expr)
}

// Expr exposes the predicate string for renderers + tests.
func (n *FilterNode) Expr() string { return n.expr }
