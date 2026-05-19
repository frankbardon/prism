package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// AggOp encodes one aggregate calculation: op (mean, sum, count, ...),
// the source field (empty for count(*) variants), and the output
// column name.
type AggOp struct {
	Op    string
	Field string
	As    string
}

// String returns a stable text form used in fingerprints and renderer
// labels.
func (a AggOp) String() string { return a.Op + "(" + a.Field + ")->" + a.As }

// GroupAggregateNode partitions its input by groupby fields and emits
// one row per partition with each aggregate. P03 stub.
type GroupAggregateNode struct {
	id      plan.NodeID
	input   plan.NodeID
	groupby []string
	aggs    []AggOp
}

// NewGroupAggregate constructs a GroupAggregateNode.
func NewGroupAggregate(id, input plan.NodeID, groupby []string, aggs []AggOp) *GroupAggregateNode {
	gb := make([]string, len(groupby))
	copy(gb, groupby)
	ag := make([]AggOp, len(aggs))
	copy(ag, aggs)
	return &GroupAggregateNode{id: id, input: input, groupby: gb, aggs: ag}
}

// ID implements plan.Node.
func (n *GroupAggregateNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *GroupAggregateNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is the projection of
// input down to groupby fields plus one F64 field per AggOp named by
// op.As. Result types come from aggregateOutputType (every shipped op
// is scalar F64 today).
func (n *GroupAggregateNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	s, err := requireSingleInput("GroupAggregateNode", in)
	if err != nil {
		return nil, err
	}
	out := &encoding.Schema{Fields: make([]encoding.Field, 0, len(n.groupby)+len(n.aggs))}
	if len(n.groupby) > 0 {
		gb, err := projectFields(s, n.groupby)
		if err != nil {
			return nil, err
		}
		out.Fields = append(out.Fields, gb.Fields...)
	}
	for _, a := range n.aggs {
		if a.As == "" {
			return nil, fmt.Errorf("GroupAggregateNode: aggregate %s missing 'as' name", a.Op)
		}
		out.Fields = append(out.Fields, encoding.Field{Name: a.As, Type: aggregateOutputType(a.Op)})
	}
	return out, nil
}

// Execute implements plan.Node. P03 stub.
func (n *GroupAggregateNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("GroupAggregateNode")
}

// Fingerprint implements plan.Node.
func (n *GroupAggregateNode) Fingerprint() string {
	parts := []string{string(n.input), strings.Join(n.groupby, ",")}
	for _, a := range n.aggs {
		parts = append(parts, a.String())
	}
	return fingerprintFor("GroupAggregateNode", parts...)
}

// Groupby exposes the partition keys for renderers + tests.
func (n *GroupAggregateNode) Groupby() []string { return n.groupby }

// Aggs exposes the aggregate operations for renderers + tests.
func (n *GroupAggregateNode) Aggs() []AggOp { return n.aggs }

// Kind implements plan.Labeled.
func (n *GroupAggregateNode) Kind() string { return "GroupAggregateNode" }

// Summary implements plan.Labeled — "by: a,b | aggs: mean(score)->m, ...".
func (n *GroupAggregateNode) Summary() string {
	out := "by: " + strings.Join(n.groupby, ",")
	if len(n.aggs) > 0 {
		aggStrs := make([]string, len(n.aggs))
		for i, a := range n.aggs {
			aggStrs[i] = a.String()
		}
		out += " | aggs: " + strings.Join(aggStrs, ",")
	}
	return out
}
