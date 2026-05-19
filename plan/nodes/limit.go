package nodes

import (
	"context"
	"strconv"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// LimitNode keeps the first N rows (with optional offset).
type LimitNode struct {
	id      plan.NodeID
	input   plan.NodeID
	limit   int
	offset  int
	backend plan.Backend
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

// Execute implements plan.Node via the injected backend.
func (n *LimitNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("LimitNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *LimitNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *LimitNode) Fingerprint() string {
	return fingerprintFor("LimitNode", string(n.input), strconv.Itoa(n.limit), strconv.Itoa(n.offset))
}

// Limit exposes the row cap for renderers + tests.
func (n *LimitNode) Limit() int { return n.limit }

// Offset exposes the offset for renderers + tests.
func (n *LimitNode) Offset() int { return n.offset }

// Kind implements plan.Labeled.
func (n *LimitNode) Kind() string { return "LimitNode" }

// Summary implements plan.Labeled.
func (n *LimitNode) Summary() string {
	if n.offset > 0 {
		return "limit: " + strconv.Itoa(n.limit) + " offset: " + strconv.Itoa(n.offset)
	}
	return "limit: " + strconv.Itoa(n.limit)
}
