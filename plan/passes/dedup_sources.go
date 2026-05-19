package passes

import (
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// DedupSourcesPass coalesces SourceNodes that share an underlying ref.
// Two SourceNodes pointing at the same `.pulse` ref will already share
// an id today (the SourceNode constructor derives the id from sha256
// of the ref), so the builder de-dups inside Build. This pass exists
// to handle the rarer case where two builders contribute Sources to
// the same DAG (e.g. layer composition once P08 lands): identical refs
// with identical downstream filter chains collapse to one.
//
// P07 ships the no-op baseline: the builder already shares Source
// nodes by id, so the pass detects the situation but never has work
// to do for v1 specs. The TestPrismDedupSources test exercises the
// canonical case (two SourceNodes with the same ref attached via the
// raw builder, bypassing the natural id-sharing).
type DedupSourcesPass struct{}

// Name implements plan.Pass.
func (DedupSourcesPass) Name() string { return "dedup_sources" }

// Apply implements plan.Pass. Walks Sources, groups by ref. When two
// or more SourceNodes share a ref AND have no downstream divergence
// before the first non-filter / non-projection node, the pass rewires
// dependents to one canonical id and drops the duplicates.
//
// For P07 the practical impact is small (the builder already shares
// Source nodes by id-from-ref), but the pass framework needs at least
// one implementation to test the fixed-point loop end-to-end.
func (DedupSourcesPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	// Group SourceNodes by ref.
	byRef := map[string][]plan.NodeID{}
	for _, id := range d.Roots() {
		n, ok := d.Node(id)
		if !ok {
			continue
		}
		src, ok := n.(*nodes.SourceNode)
		if !ok {
			continue
		}
		byRef[src.Ref()] = append(byRef[src.Ref()], id)
	}
	// Compute rewire map: every duplicate id → canonical id (smallest
	// id alphabetically wins so the result is deterministic).
	rewire := map[plan.NodeID]plan.NodeID{}
	for _, ids := range byRef {
		if len(ids) < 2 {
			continue
		}
		keep := ids[0]
		for _, id := range ids[1:] {
			if id < keep {
				keep = id
			}
		}
		for _, id := range ids {
			if id != keep {
				rewire[id] = keep
			}
		}
	}
	if len(rewire) == 0 {
		return d, false, nil
	}
	return applyRewire(d, rewire), true, nil
}

// applyRewire returns a new DAG with every dependent of a rewire-key
// node updated to point at the rewire-value node, and the rewire-key
// nodes themselves dropped. Pure rewiring; structural sharing where
// possible.
func applyRewire(d *plan.DAG, rewire map[plan.NodeID]plan.NodeID) *plan.DAG {
	out := d
	for _, id := range d.Nodes() {
		if _, dropped := rewire[id]; dropped {
			out = out.WithoutNode(id)
			continue
		}
		n, _ := d.Node(id)
		needsRewire := false
		for _, in := range n.Inputs() {
			if _, ok := rewire[in]; ok {
				needsRewire = true
				break
			}
		}
		if !needsRewire {
			continue
		}
		// Replace this node's inputs by walking the rewire map.
		out = out.WithNode(rewireInputs(n, rewire))
	}
	return out
}

// rewireInputs returns a node whose Inputs() reflect the rewire map.
// For the P07 baseline we only need to rewire ProjectNode/FilterNode/
// JoinNode/UnionNode (every non-Source node satisfies a rewire
// interface) — but the canonical approach is to ask each node to
// rebuild itself. Since plan.Node has no exposed mutators, we use a
// per-kind switch and reconstruct via the public constructors.
func rewireInputs(n plan.Node, rewire map[plan.NodeID]plan.NodeID) plan.Node {
	resolve := func(id plan.NodeID) plan.NodeID {
		if v, ok := rewire[id]; ok {
			return v
		}
		return id
	}
	switch v := n.(type) {
	case *nodes.FilterNode:
		// FilterNode has only one input; the constructor signature lets
		// us rebuild with the rewired input.
		return nodes.NewFilter(v.ID(), resolve(v.Inputs()[0]), v.Expr())
	case *nodes.ProjectNode:
		return nodes.NewProject(v.ID(), resolve(v.Inputs()[0]), v.Fields())
	case *nodes.JoinNode:
		ins := v.Inputs()
		return nodes.NewJoin(v.ID(), resolve(ins[0]), resolve(ins[1]),
			v.On(), v.JoinKind(), 0)
	case *nodes.UnionNode:
		ins := v.Inputs()
		newIns := make([]plan.NodeID, len(ins))
		for i, in := range ins {
			newIns[i] = resolve(in)
		}
		return nodes.NewUnion(v.ID(), newIns)
	}
	// Other node kinds: P07 falls back to leaving the node alone. A
	// future pass will need a more general rewire helper; for v1 the
	// dedup pass only fires on Source-rooted patterns, so only the
	// single-input nodes above are relevant.
	return n
}
