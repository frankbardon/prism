package nodes

import (
	"context"
	"strings"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// PivotNode reshapes long → wide. It plans but does not execute.
//
// `pivot` is the one transform Prism's grammar accepts that nothing
// implements: validate refuses it with PRISM_SPEC_067 before a plan is
// ever built, and a node constructed by hand fails at Execute with
// PRISM_COMPILE_001. `crosstab` produces the same long → wide shape and
// does execute (plan/nodes/crosstab.go + compile/inmem/crosstab.go), so
// it is the answer for callers who land here.
//
// Schema cannot name the output columns without scanning the input data
// (the cells in the `pivot` column become the new column headers), so
// it returns the input schema unchanged — conservative, and enough to
// keep DAG validation from blocking. An executor would derive the
// output schema at execute time, the way GroupAggregateNode and
// CrosstabNode do.
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

// Schema implements plan.Node. Conservative default — the input shape
// verbatim; see the type note above.
func (n *PivotNode) Schema(in []*table.Schema) (*table.Schema, error) {
	return requireSingleInput("PivotNode", in)
}

// Execute implements plan.Node. There is no executor; see the type
// note above.
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

// Kind implements plan.Labeled.
func (n *PivotNode) Kind() string { return "PivotNode" }

// Summary implements plan.Labeled.
func (n *PivotNode) Summary() string {
	return "pivot: " + n.pivot + " value: " + n.value
}
