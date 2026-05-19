package passes

import (
	"sort"
	"strings"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// AggregateFusionPass merges sibling GroupAggregateNodes that share an
// input AND a groupby key list. The merged node carries the union of
// the source nodes' aggregate ops; downstream consumers that referenced
// either pre-merge node's id rewire to the merged node id (computed
// deterministically as `fuse:<sorted-ids-join('+')>`).
//
// Limitation (P07): the pass merges at most one fusion group per Apply
// call. The fixed-point loop runs Apply repeatedly until no further
// merges happen, so the pass eventually fuses every eligible group.
// This keeps the rewire logic simple (one merged node per call =
// straightforward dependent-rewire pass).
type AggregateFusionPass struct{}

// Name implements plan.Pass.
func (AggregateFusionPass) Name() string { return "aggregate_fusion" }

// Apply implements plan.Pass.
func (AggregateFusionPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	// Group GroupAggregateNodes by (input id, sorted groupby key list).
	groups := map[string][]*nodes.GroupAggregateNode{}
	keyFor := func(g *nodes.GroupAggregateNode) string {
		gb := append([]string(nil), g.Groupby()...)
		sort.Strings(gb)
		return string(g.Inputs()[0]) + "|" + strings.Join(gb, ",")
	}
	for _, id := range d.Nodes() {
		n, ok := d.Node(id)
		if !ok {
			continue
		}
		ga, ok := n.(*nodes.GroupAggregateNode)
		if !ok {
			continue
		}
		k := keyFor(ga)
		groups[k] = append(groups[k], ga)
	}
	// Find the first group with ≥2 members and merge it.
	for _, members := range groups {
		if len(members) < 2 {
			continue
		}
		return fuseGroup(d, members), true, nil
	}
	return d, false, nil
}

// fuseGroup constructs the merged node, rewires every dependent of
// every member to the merged node, and removes the original members.
func fuseGroup(d *plan.DAG, members []*nodes.GroupAggregateNode) *plan.DAG {
	sort.Slice(members, func(i, j int) bool {
		return members[i].ID() < members[j].ID()
	})
	// Merge aggregates (dedup by output name).
	seen := map[string]struct{}{}
	merged := make([]nodes.AggOp, 0)
	for _, m := range members {
		for _, op := range m.Aggs() {
			key := op.Op + "(" + op.Field + ")->" + op.As
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, op)
		}
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = string(m.ID())
	}
	mergedID := plan.NodeID("fuse:" + strings.Join(ids, "+"))
	mergedNode := nodes.NewGroupAggregate(mergedID,
		members[0].Inputs()[0],
		members[0].Groupby(),
		merged,
	)
	out := d.WithNode(mergedNode)
	// Rewire every dependent of every member to mergedID.
	for _, m := range members {
		for _, depID := range d.Dependents(m.ID()) {
			depNode, _ := out.Node(depID)
			rebuilt := rewireSingleInput(depNode, m.ID(), mergedID)
			if rebuilt != nil {
				out = out.WithNode(rebuilt)
			}
		}
		// Drop the original member (unless it was a sink — preserve the
		// sink marker by keeping the merged node as the new sink via
		// WithoutNode's existing logic).
		out = out.WithoutNode(m.ID())
	}
	return out
}
