// Package layout holds the shared graph + tree layout helpers used
// by the encode/marks tree, dendrogram, and network encoders.
//
// All algorithms are pure Go, deterministic for fixed seeds, and live
// behind this small interface so encoders depend on data shapes
// rather than algorithm internals.
package layout

import "fmt"

// Node is one vertex in a graph or tree. ID is the user-supplied
// identity (e.g. the value of the spec's `target` channel); Label is
// optional and surfaces in the rendered mark.
type Node struct {
	ID    string
	Label string
}

// Edge is one directed connection. For trees, From is the parent and
// To is the child; for networks, the direction may be ignored.
type Edge struct {
	From   string
	To     string
	Weight float64
}

// Graph is an adjacency-list representation. Nodes are deduped by ID
// in insertion order; Edges keep their declaration order so layout
// output is deterministic.
type Graph struct {
	Nodes  []Node
	Edges  []Edge
	byID   map[string]int
	outAdj map[string][]string
	inAdj  map[string][]string
}

// NewGraph returns an empty graph ready for AddNode / AddEdge.
func NewGraph() *Graph {
	return &Graph{
		byID:   map[string]int{},
		outAdj: map[string][]string{},
		inAdj:  map[string][]string{},
	}
}

// AddNode registers a node; duplicate IDs update Label.
func (g *Graph) AddNode(n Node) {
	if idx, ok := g.byID[n.ID]; ok {
		if n.Label != "" {
			g.Nodes[idx].Label = n.Label
		}
		return
	}
	g.byID[n.ID] = len(g.Nodes)
	g.Nodes = append(g.Nodes, n)
}

// AddEdge registers a directed edge. Both endpoints are ensured to
// exist as nodes (with empty labels) before the edge lands.
func (g *Graph) AddEdge(e Edge) {
	if _, ok := g.byID[e.From]; !ok {
		g.AddNode(Node{ID: e.From})
	}
	if _, ok := g.byID[e.To]; !ok {
		g.AddNode(Node{ID: e.To})
	}
	g.Edges = append(g.Edges, e)
	g.outAdj[e.From] = append(g.outAdj[e.From], e.To)
	g.inAdj[e.To] = append(g.inAdj[e.To], e.From)
}

// NodeCount returns the number of unique nodes.
func (g *Graph) NodeCount() int { return len(g.Nodes) }

// EdgeCount returns the number of edges (no deduping).
func (g *Graph) EdgeCount() int { return len(g.Edges) }

// Children returns the IDs of every direct successor of id in
// declaration order.
func (g *Graph) Children(id string) []string {
	return append([]string(nil), g.outAdj[id]...)
}

// Parents returns the IDs of every direct predecessor of id.
func (g *Graph) Parents(id string) []string {
	return append([]string(nil), g.inAdj[id]...)
}

// Roots returns every node with zero incoming edges, in declaration
// order. A well-formed tree has exactly one root.
func (g *Graph) Roots() []string {
	var out []string
	for _, n := range g.Nodes {
		if len(g.inAdj[n.ID]) == 0 {
			out = append(out, n.ID)
		}
	}
	return out
}

// HasCycle reports whether the graph contains a directed cycle. Uses
// a DFS with WHITE/GRAY/BLACK colors; safe on graphs with multiple
// roots or disconnected components.
func (g *Graph) HasCycle() bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(g.Nodes))
	var dfs func(id string) bool
	dfs = func(id string) bool {
		color[id] = gray
		for _, child := range g.outAdj[id] {
			switch color[child] {
			case gray:
				return true
			case white:
				if dfs(child) {
					return true
				}
			}
		}
		color[id] = black
		return false
	}
	for _, n := range g.Nodes {
		if color[n.ID] == white {
			if dfs(n.ID) {
				return true
			}
		}
	}
	return false
}

// BuildTree validates that g is a rooted forest with a single root
// and returns the root ID. Returns an error when:
//   - the graph contains 0 or >1 roots,
//   - the graph contains a cycle.
//
// The validator-level rules (PRISM_SPEC_028/029) catch these earlier
// in the pipeline, but BuildTree is the defensive last line before
// the layout walks try to recurse on malformed inputs.
func (g *Graph) BuildTree() (string, error) {
	if g.NodeCount() == 0 {
		return "", fmt.Errorf("tree: empty graph")
	}
	if g.HasCycle() {
		return "", fmt.Errorf("tree: input graph contains a cycle")
	}
	roots := g.Roots()
	if len(roots) == 0 {
		return "", fmt.Errorf("tree: no root found (every node has a parent — cycle or self-loop?)")
	}
	if len(roots) > 1 {
		return "", fmt.Errorf("tree: %d roots found (multi-root forests not supported in v0.2)", len(roots))
	}
	return roots[0], nil
}
