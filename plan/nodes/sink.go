package nodes

import (
	"context"
	"fmt"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// SinkNode is the synthetic terminal node every P03 DAG ends with. It
// receives one input table and emits it verbatim. P05 replaces this
// with Scene encoding nodes; until then the Sink gives the executor a
// deterministic terminal and the Builder a clear "this is the answer"
// marker.
//
// See D030 for the rationale.
type SinkNode struct {
	id    plan.NodeID
	input plan.NodeID
}

// NewSink constructs a SinkNode.
func NewSink(id, input plan.NodeID) *SinkNode {
	return &SinkNode{id: id, input: input}
}

// ID implements plan.Node.
func (n *SinkNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *SinkNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Sink is a no-op pass-through.
func (n *SinkNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	return requireSingleInput("SinkNode", in)
}

// Execute implements plan.Node. Passes its single input through.
func (n *SinkNode) Execute(_ context.Context, in []*table.Table) (*table.Table, error) {
	if len(in) != 1 || in[0] == nil {
		return nil, fmt.Errorf("SinkNode: expected exactly one input table, got %d", len(in))
	}
	return in[0], nil
}

// Fingerprint implements plan.Node.
func (n *SinkNode) Fingerprint() string {
	return fingerprintFor("SinkNode", string(n.input))
}
