package nodes

import (
	"context"
	"strconv"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// JoinKind is the join semantics token: inner|left|outer|anti. The
// schema computation does not branch on kind in P03 (the column set is
// the union either way; null behaviour comes in at Execute time, P07).
type JoinKind string

const (
	JoinInner JoinKind = "inner"
	JoinLeft  JoinKind = "left"
	JoinOuter JoinKind = "outer"
	JoinAnti  JoinKind = "anti"
)

// JoinNode hash-joins two inputs on equality. P03 stub.
type JoinNode struct {
	id      plan.NodeID
	left    plan.NodeID
	right   plan.NodeID
	on      []string
	kind    JoinKind
	maxRows int
}

// NewJoin constructs a JoinNode.
func NewJoin(id, left, right plan.NodeID, on []string, kind JoinKind, maxRows int) *JoinNode {
	cp := make([]string, len(on))
	copy(cp, on)
	return &JoinNode{id: id, left: left, right: right, on: cp, kind: kind, maxRows: maxRows}
}

// ID implements plan.Node.
func (n *JoinNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node. Order is (left, right) so the executor
// can pass the two upstream Tables in the right slot.
func (n *JoinNode) Inputs() []plan.NodeID { return []plan.NodeID{n.left, n.right} }

// Schema implements plan.Node. Output schema is the union of left and
// right schemas, dropping right-side duplicates of the join keys.
func (n *JoinNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	if len(in) != 2 || in[0] == nil || in[1] == nil {
		return nil, requireTwoInputsErr("JoinNode", in)
	}
	return joinedSchema(in[0], in[1], n.on), nil
}

// Execute implements plan.Node. P03 stub.
func (n *JoinNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("JoinNode")
}

// Fingerprint implements plan.Node.
func (n *JoinNode) Fingerprint() string {
	return fingerprintFor("JoinNode",
		string(n.left), string(n.right),
		string(n.kind), strings.Join(n.on, ","),
		strconv.Itoa(n.maxRows),
	)
}

// On exposes the join keys for renderers + tests.
func (n *JoinNode) On() []string { return n.on }

// JoinKind exposes the join semantics for renderers + tests.
// (Named JoinKind not Kind to avoid colliding with plan.Labeled.Kind.)
func (n *JoinNode) JoinKind() JoinKind { return n.kind }

// Kind implements plan.Labeled.
func (n *JoinNode) Kind() string { return "JoinNode" }

// Summary implements plan.Labeled.
func (n *JoinNode) Summary() string {
	return string(n.kind) + " on: " + strings.Join(n.on, ",")
}
