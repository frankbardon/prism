package passes

import (
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// FilterPushdownPass moves FilterNodes that sit immediately downstream
// of a JoinNode to whichever side of the join exclusively supplies the
// columns the filter references. Filters touching columns from both
// sides stay where they are.
//
// Column extraction (E2-S1): the referenced columns come straight from
// the structured predicate via FilterNode.ReferencedFields() — no more
// lexing an expression string. The pass only pushes when EVERY column
// maps exclusively to one side, so a filter spanning both sides bails
// safely.
type FilterPushdownPass struct{}

// Name implements plan.Pass.
func (FilterPushdownPass) Name() string { return "filter_pushdown" }

// Apply implements plan.Pass.
func (FilterPushdownPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	out := d
	changed := false
	for _, id := range d.Nodes() {
		n, ok := out.Node(id)
		if !ok {
			continue
		}
		fn, ok := n.(*nodes.FilterNode)
		if !ok {
			continue
		}
		if len(fn.Inputs()) != 1 {
			continue
		}
		upID := fn.Inputs()[0]
		up, ok := out.Node(upID)
		if !ok {
			continue
		}
		jn, ok := up.(*nodes.JoinNode)
		if !ok {
			continue
		}
		cols := fn.ReferencedFields()
		if len(cols) == 0 {
			continue
		}
		leftNode, lok := out.Node(jn.Inputs()[0])
		rightNode, rok := out.Node(jn.Inputs()[1])
		if !lok || !rok {
			continue
		}
		leftSchema, err := leftNode.Schema(nil)
		if err != nil {
			continue
		}
		rightSchema, err := rightNode.Schema(nil)
		if err != nil {
			continue
		}
		leftCols := schemaColSet(leftSchema)
		rightCols := schemaColSet(rightSchema)

		onlyLeft, onlyRight := true, true
		for _, c := range cols {
			_, inLeft := leftCols[c]
			_, inRight := rightCols[c]
			if !inLeft {
				onlyLeft = false
			}
			if !inRight {
				onlyRight = false
			}
		}
		switch {
		case onlyLeft && !onlyRight:
			out = pushFilterUnderJoin(out, fn, jn, "left")
			changed = true
		case onlyRight && !onlyLeft:
			out = pushFilterUnderJoin(out, fn, jn, "right")
			changed = true
		}
	}
	return out, changed, nil
}

// pushFilterUnderJoin rewires the DAG so the filter sits below the
// join on the named side. The original filter id is reused so any
// downstream reference still resolves; the join is reconstructed
// with the filter as its new input on that side.
func pushFilterUnderJoin(
	d *plan.DAG, fn *nodes.FilterNode, jn *nodes.JoinNode, side string,
) *plan.DAG {
	leftIn, rightIn := jn.Inputs()[0], jn.Inputs()[1]
	var newFilterInput, newJoinLeft, newJoinRight plan.NodeID
	switch side {
	case "left":
		newFilterInput = leftIn
		newJoinLeft = fn.ID()
		newJoinRight = rightIn
	case "right":
		newFilterInput = rightIn
		newJoinLeft = leftIn
		newJoinRight = fn.ID()
	default:
		return d
	}
	rebuiltFilter := nodes.NewFilter(fn.ID(), newFilterInput, fn.Predicate())
	rebuiltJoin := nodes.NewJoin(jn.ID(), newJoinLeft, newJoinRight,
		jn.On(), jn.JoinKind(), 0)
	out := d.WithNode(rebuiltFilter)
	out = out.WithNode(rebuiltJoin)
	return out
}
