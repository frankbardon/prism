package nodes

import (
	"context"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// SortNode orders rows by one or more (field, order) keys. P03 stub.
type SortNode struct {
	id    plan.NodeID
	input plan.NodeID
	sort  []SortKey
}

// NewSort constructs a SortNode. The sort slice is copied.
func NewSort(id, input plan.NodeID, sort []SortKey) *SortNode {
	sk := make([]SortKey, len(sort))
	copy(sk, sort)
	return &SortNode{id: id, input: input, sort: sk}
}

// ID implements plan.Node.
func (n *SortNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *SortNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Sort never changes the schema.
func (n *SortNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	return requireSingleInput("SortNode", in)
}

// Execute implements plan.Node. P03 stub.
func (n *SortNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("SortNode")
}

// Fingerprint implements plan.Node.
func (n *SortNode) Fingerprint() string {
	parts := []string{string(n.input)}
	for _, sk := range n.sort {
		parts = append(parts, sk.String())
	}
	return fingerprintFor("SortNode", parts...)
}

// Sort exposes the keys for renderers + tests.
func (n *SortNode) Sort() []SortKey { return n.sort }

// SortLabel returns a renderer-friendly summary like "score:desc,name:asc".
func (n *SortNode) SortLabel() string {
	parts := make([]string, len(n.sort))
	for i, sk := range n.sort {
		parts[i] = sk.String()
	}
	return strings.Join(parts, ",")
}

// Kind implements plan.Labeled.
func (n *SortNode) Kind() string { return "SortNode" }

// Summary implements plan.Labeled.
func (n *SortNode) Summary() string { return "by: " + n.SortLabel() }
