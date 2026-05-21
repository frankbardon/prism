package layout

import "fmt"

// TreePosition is one node's resolved coordinates after a tidy-tree
// layout pass. Y is the depth (0 at root) × verticalGap; X is the
// horizontal offset within the layout's local coordinate space.
type TreePosition struct {
	ID    string
	X     float64
	Y     float64
	Depth int
}

// TidyTree runs a tidy-tree layout on g rooted at rootID. Y is the
// depth × verticalGap; X is positioned so:
//
//   - leaves are spaced horizontalGap apart in declaration order,
//   - every internal node sits at the midpoint of its children's
//     X coordinates (so the root centres over the leaf span).
//
// The algorithm runs in O(n) via one post-order assignment of leaf
// X + one final pass that resolves internal nodes from their child
// positions. It's the "naïve tidy" variant of Reingold-Tilford;
// good enough for org charts, decision trees, and the gallery
// fixtures while staying easy to read and golden-stable.
//
// BuildTree validation guarantees a single root + no cycle before
// this function runs.
func TidyTree(g *Graph, rootID string, horizontalGap, verticalGap float64) ([]TreePosition, error) {
	if g == nil {
		return nil, fmt.Errorf("tidy-tree: nil graph")
	}
	if _, ok := g.byID[rootID]; !ok {
		return nil, fmt.Errorf("tidy-tree: root %q not in graph", rootID)
	}
	if horizontalGap <= 0 {
		horizontalGap = 1
	}
	if verticalGap <= 0 {
		verticalGap = 1
	}

	type wnode struct {
		id    string
		depth int
		x     float64
		kids  []*wnode
	}

	all := map[string]*wnode{}
	var build func(id string, depth int) *wnode
	build = func(id string, depth int) *wnode {
		n := &wnode{id: id, depth: depth}
		all[id] = n
		for _, c := range g.Children(id) {
			n.kids = append(n.kids, build(c, depth+1))
		}
		return n
	}
	root := build(rootID, 0)

	// Post-order: assign leaf X in declaration order; internal nodes
	// take the midpoint of their kids' X.
	leafCursor := 0.0
	var assign func(n *wnode)
	assign = func(n *wnode) {
		if len(n.kids) == 0 {
			n.x = leafCursor
			leafCursor += horizontalGap
			return
		}
		for _, k := range n.kids {
			assign(k)
		}
		n.x = (n.kids[0].x + n.kids[len(n.kids)-1].x) / 2
	}
	assign(root)

	// Collect in pre-order for stable output.
	var out []TreePosition
	var collect func(n *wnode)
	collect = func(n *wnode) {
		out = append(out, TreePosition{
			ID:    n.id,
			X:     n.x,
			Y:     float64(n.depth) * verticalGap,
			Depth: n.depth,
		})
		for _, k := range n.kids {
			collect(k)
		}
	}
	collect(root)
	return out, nil
}
