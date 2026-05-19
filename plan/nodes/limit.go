package nodes

import (
	"context"
	"strconv"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// LimitNode keeps the first N rows (with optional offset). P03 stub.
type LimitNode struct {
	id     plan.NodeID
	input  plan.NodeID
	limit  int
	offset int
}

// NewLimit constructs a LimitNode.
func NewLimit(id, input plan.NodeID, limit, offset int) *LimitNode {
	return &LimitNode{id: id, input: input, limit: limit, offset: offset}
}

// ID implements plan.Node.
func (n *LimitNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *LimitNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Limit never changes the schema.
func (n *LimitNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	return requireSingleInput("LimitNode", in)
}

// Execute implements plan.Node. P03 stub.
func (n *LimitNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("LimitNode")
}

// Fingerprint implements plan.Node.
func (n *LimitNode) Fingerprint() string {
	return fingerprintFor("LimitNode", string(n.input), strconv.Itoa(n.limit), strconv.Itoa(n.offset))
}

// Limit exposes the row cap for renderers + tests.
func (n *LimitNode) Limit() int { return n.limit }

// Offset exposes the offset for renderers + tests.
func (n *LimitNode) Offset() int { return n.offset }
