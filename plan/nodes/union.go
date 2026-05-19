package nodes

import (
	"context"
	"fmt"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// UnionNode vertically concatenates N inputs. P03 stub.
//
// Schema computation in P03 returns the first input's schema verbatim;
// the real Execute path (P07) will validate cross-input schema
// compatibility (same field names, compatible types). Marked here so
// the eventual swap is mechanical.
//
// TODO P07: validate that every input schema matches in[0] and emit a
// PRISM_PLAN_* code on mismatch.
type UnionNode struct {
	id     plan.NodeID
	inputs []plan.NodeID
}

// NewUnion constructs a UnionNode. The inputs slice is copied.
func NewUnion(id plan.NodeID, inputs []plan.NodeID) *UnionNode {
	cp := make([]plan.NodeID, len(inputs))
	copy(cp, inputs)
	return &UnionNode{id: id, inputs: cp}
}

// ID implements plan.Node.
func (n *UnionNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *UnionNode) Inputs() []plan.NodeID { return n.inputs }

// Schema implements plan.Node. Returns the first input's schema; see
// the TODO P07 note on cross-input validation.
func (n *UnionNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("UnionNode: no input schemas")
	}
	if in[0] == nil {
		return nil, fmt.Errorf("UnionNode: input[0] schema is nil")
	}
	return in[0], nil
}

// Execute implements plan.Node. P03 stub.
func (n *UnionNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("UnionNode")
}

// Fingerprint implements plan.Node.
func (n *UnionNode) Fingerprint() string {
	parts := make([]string, len(n.inputs))
	for i, id := range n.inputs {
		parts[i] = string(id)
	}
	return fingerprintFor("UnionNode", parts...)
}
