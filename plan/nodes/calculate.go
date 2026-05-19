package nodes

import (
	"context"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// CalculateNode appends one computed column derived from a Pulse
// expression.
type CalculateNode struct {
	id      plan.NodeID
	input   plan.NodeID
	expr    string
	as      string
	backend plan.Backend
}

// NewCalculate constructs a CalculateNode.
func NewCalculate(id, input plan.NodeID, expr, as string) *CalculateNode {
	return &CalculateNode{id: id, input: input, expr: expr, as: as}
}

// ID implements plan.Node.
func (n *CalculateNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *CalculateNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is input + one F64 field
// named n.as. F64 is the conservative bucket for expression results
// (every Pulse arithmetic expression promotes to float).
func (n *CalculateNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	s, err := requireSingleInput("CalculateNode", in)
	if err != nil {
		return nil, err
	}
	return appendField(s, n.as, encoding.FieldTypeF64), nil
}

// Execute implements plan.Node via the injected backend.
func (n *CalculateNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("CalculateNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *CalculateNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *CalculateNode) Fingerprint() string {
	return fingerprintFor("CalculateNode", string(n.input), n.expr, n.as)
}

// Expr exposes the expression for renderers + tests.
func (n *CalculateNode) Expr() string { return n.expr }

// As exposes the output column name for renderers + tests.
func (n *CalculateNode) As() string { return n.as }

// Kind implements plan.Labeled.
func (n *CalculateNode) Kind() string { return "CalculateNode" }

// Summary implements plan.Labeled.
func (n *CalculateNode) Summary() string { return n.as + " = " + n.expr }
