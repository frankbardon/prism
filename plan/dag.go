package plan

import (
	"sort"
)

// DAG is the immutable plan graph. Constructed via Builder; mutated
// only by optimizer passes that return new DAG instances with
// structural sharing (D017). All public accessors return sorted /
// stable views so downstream code (renderers, executor, tests) sees
// deterministic ordering across runs.
type DAG struct {
	nodes map[NodeID]Node
	roots []NodeID
	sinks []NodeID
}

// Node looks up a node by id. The second return is false when no such
// node exists in the DAG.
func (d *DAG) Node(id NodeID) (Node, bool) {
	n, ok := d.nodes[id]
	return n, ok
}

// Nodes returns every node id in the DAG, sorted lexicographically.
// Determinism is important for goldens; using map iteration order
// would produce flaky DOT/JSON output.
func (d *DAG) Nodes() []NodeID {
	out := make([]NodeID, 0, len(d.nodes))
	for id := range d.nodes {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Roots returns the source node ids (no upstream). Sorted.
func (d *DAG) Roots() []NodeID {
	out := append([]NodeID(nil), d.roots...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Sinks returns the terminal node ids (the ones the Scene encoder
// reads in P05+). Sorted.
func (d *DAG) Sinks() []NodeID {
	out := append([]NodeID(nil), d.sinks...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Dependents returns the ids of nodes whose Inputs() include id, in
// stable sorted order. O(N) over the node set; fine for the small
// DAGs Prism builds. P07 may cache this if profiling motivates.
func (d *DAG) Dependents(id NodeID) []NodeID {
	out := []NodeID{}
	for other, n := range d.nodes {
		for _, in := range n.Inputs() {
			if in == id {
				out = append(out, other)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Size returns the node count. Cheap; useful in tests and metrics.
func (d *DAG) Size() int { return len(d.nodes) }

// WithNode returns a new DAG with n added or replaced. Roots/sinks are
// copied verbatim; the caller is responsible for re-marking roots and
// sinks if the structure change requires it. All other node pointers
// are shared (structural sharing per D017).
func (d *DAG) WithNode(n Node) *DAG {
	out := &DAG{
		nodes: make(map[NodeID]Node, len(d.nodes)+1),
		roots: append([]NodeID(nil), d.roots...),
		sinks: append([]NodeID(nil), d.sinks...),
	}
	for k, v := range d.nodes {
		out.nodes[k] = v
	}
	out.nodes[n.ID()] = n
	return out
}

// WithoutNode returns a new DAG with id removed. If id was a root or
// sink, it is removed from those lists too.
func (d *DAG) WithoutNode(id NodeID) *DAG {
	out := &DAG{
		nodes: make(map[NodeID]Node, len(d.nodes)),
		roots: filterID(d.roots, id),
		sinks: filterID(d.sinks, id),
	}
	for k, v := range d.nodes {
		if k == id {
			continue
		}
		out.nodes[k] = v
	}
	return out
}

func filterID(ids []NodeID, drop NodeID) []NodeID {
	out := make([]NodeID, 0, len(ids))
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}
