package nodes

import (
	"context"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// PivotNode reshapes long → wide. P03 stub.
//
// Schema computation cannot determine the output column names without
// scanning the input data (the cells in the `pivot` column become the
// new column headers). In P03 we conservatively return the input
// schema unchanged so DAG validation does not block — the real Schema
// derivation lands in P04 alongside the real Execute path.
//
// TODO P04: implement schema-on-execute by reading the input table
// once at Schema() time (or annotating the plan with the discovered
// categories at build time when the data is available statically).
type PivotNode struct {
	id      plan.NodeID
	input   plan.NodeID
	pivot   string
	value   string
	groupby []string
	op      string
}

// NewPivot constructs a PivotNode.
func NewPivot(id, input plan.NodeID, pivot, value string, groupby []string, op string) *PivotNode {
	cp := make([]string, len(groupby))
	copy(cp, groupby)
	return &PivotNode{id: id, input: input, pivot: pivot, value: value, groupby: cp, op: op}
}

// ID implements plan.Node.
func (n *PivotNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *PivotNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Conservative default; see the package
// note above. TODO P04.
func (n *PivotNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	return requireSingleInput("PivotNode", in)
}

// Execute implements plan.Node. P03 stub.
func (n *PivotNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("PivotNode")
}

// Fingerprint implements plan.Node.
func (n *PivotNode) Fingerprint() string {
	return fingerprintFor("PivotNode",
		string(n.input), n.pivot, n.value,
		strings.Join(n.groupby, ","), n.op,
	)
}

// Pivot exposes the column whose distinct values become headers.
func (n *PivotNode) Pivot() string { return n.pivot }

// Value exposes the source value column.
func (n *PivotNode) Value() string { return n.value }
