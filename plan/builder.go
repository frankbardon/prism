package plan

import (
	"fmt"

	prismerrors "github.com/frankbardon/prism/errors"
)

// Builder constructs a DAG. AddNode + MarkRoot + MarkSink populate
// state; Build validates the result and produces an immutable *DAG.
//
// The Builder is intentionally narrow — no Edge() method, because
// edges are already implicit in each Node's Inputs(). Build verifies
// every input target exists in the node set and at least one sink
// exists.
type Builder struct {
	nodes map[NodeID]Node
	roots []NodeID
	sinks []NodeID
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder {
	return &Builder{nodes: map[NodeID]Node{}}
}

// NodeIDs returns a copy of the set of currently-registered ids.
// Used by the build subpackage to detect "the newest leaf" after a
// dataset registration.
func (b *Builder) NodeIDs() map[NodeID]struct{} {
	out := make(map[NodeID]struct{}, len(b.nodes))
	for id := range b.nodes {
		out[id] = struct{}{}
	}
	return out
}

// Node returns the registered node for id (or nil + false). Used by
// build helpers that need to inspect upstream node kinds before
// rewriting the DAG (e.g. the crosstab transform consumes a
// SourceNode directly).
func (b *Builder) Node(id NodeID) (Node, bool) {
	n, ok := b.nodes[id]
	return n, ok
}

// RemoveNode deletes id from the builder's pending node set + any
// root / sink marker that references it. Used by builders that swap
// a leaf for an upstream replacement (e.g. crosstab consuming a
// SourceNode directly). No-op when id is not registered.
func (b *Builder) RemoveNode(id NodeID) {
	delete(b.nodes, id)
	if len(b.roots) > 0 {
		filtered := b.roots[:0]
		for _, r := range b.roots {
			if r != id {
				filtered = append(filtered, r)
			}
		}
		b.roots = filtered
	}
	if len(b.sinks) > 0 {
		filtered := b.sinks[:0]
		for _, s := range b.sinks {
			if s != id {
				filtered = append(filtered, s)
			}
		}
		b.sinks = filtered
	}
}

// AddNode registers n. Returns an error if a node with the same id is
// already present (duplicates are always a bug in the spec → DAG
// translator).
func (b *Builder) AddNode(n Node) error {
	if n == nil {
		return fmt.Errorf("builder: nil node")
	}
	if _, dup := b.nodes[n.ID()]; dup {
		return fmt.Errorf("builder: duplicate node id %q", n.ID())
	}
	b.nodes[n.ID()] = n
	return nil
}

// MarkRoot declares id as a source node. Build verifies the node has
// no Inputs(); the marker is only for fast iteration (Roots()).
func (b *Builder) MarkRoot(id NodeID) error {
	if _, ok := b.nodes[id]; !ok {
		return fmt.Errorf("builder: MarkRoot on unknown id %q", id)
	}
	for _, r := range b.roots {
		if r == id {
			return nil
		}
	}
	b.roots = append(b.roots, id)
	return nil
}

// MarkSink declares id as a terminal node. Build verifies at least
// one sink is registered.
func (b *Builder) MarkSink(id NodeID) error {
	if _, ok := b.nodes[id]; !ok {
		return fmt.Errorf("builder: MarkSink on unknown id %q", id)
	}
	for _, s := range b.sinks {
		if s == id {
			return nil
		}
	}
	b.sinks = append(b.sinks, id)
	return nil
}

// Build validates the staged graph and returns the immutable *DAG.
//
// Validation:
//   - Every node's Inputs() target must exist in the node set.
//   - Every root node must have zero Inputs.
//   - At least one sink must be marked.
//
// Topology (cycles) is checked separately by DAG.TopoLevels so
// callers can build and inspect a graph that contains a cycle for
// debug renderers.
func (b *Builder) Build() (*DAG, error) {
	if len(b.nodes) == 0 {
		return nil, fmt.Errorf("builder: empty DAG")
	}
	for id, n := range b.nodes {
		for _, in := range n.Inputs() {
			if _, ok := b.nodes[in]; !ok {
				return nil, prismerrors.New(
					"PRISM_PLAN_003",
					fmt.Sprintf("Node %q depends on undefined input %q.", id, in),
					map[string]any{"Dataset": string(in), "Available": availableIDs(b.nodes)},
				)
			}
		}
	}
	for _, r := range b.roots {
		n := b.nodes[r]
		if len(n.Inputs()) != 0 {
			return nil, fmt.Errorf("builder: root %q has %d inputs (roots must be sources)", r, len(n.Inputs()))
		}
	}
	if len(b.sinks) == 0 {
		return nil, fmt.Errorf("builder: no sinks marked")
	}

	nodes := make(map[NodeID]Node, len(b.nodes))
	for k, v := range b.nodes {
		nodes[k] = v
	}
	roots := append([]NodeID(nil), b.roots...)
	sinks := append([]NodeID(nil), b.sinks...)
	return &DAG{nodes: nodes, roots: roots, sinks: sinks}, nil
}

func availableIDs(m map[NodeID]Node) string {
	out := ""
	first := true
	for id := range m {
		if !first {
			out += ", "
		}
		out += string(id)
		first = false
	}
	return out
}

// --- test-only helpers (still exported but suffixed Unchecked so the
// production code path is obvious). Used by plan/dag_cycle_test.go to
// inject a cycle without going through Build's input-validation.

// AddNodeUnchecked is identical to AddNode minus duplicate detection.
// Test-only.
func (b *Builder) AddNodeUnchecked(n Node) {
	b.nodes[n.ID()] = n
}

// BuildUnchecked skips input/root validation so tests can hand-build
// cycles and assert TopoLevels detects them. Test-only.
func (b *Builder) BuildUnchecked() *DAG {
	nodes := make(map[NodeID]Node, len(b.nodes))
	for k, v := range b.nodes {
		nodes[k] = v
	}
	return &DAG{
		nodes: nodes,
		roots: append([]NodeID(nil), b.roots...),
		sinks: append([]NodeID(nil), b.sinks...),
	}
}
