package layout

import "testing"

func TestGraphAddDedupesNodes(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "a", Label: "Updated"})
	if g.NodeCount() != 1 {
		t.Fatalf("NodeCount = %d, want 1", g.NodeCount())
	}
	if g.Nodes[0].Label != "Updated" {
		t.Errorf("dedupe should update Label, got %q", g.Nodes[0].Label)
	}
}

func TestGraphEdgeRegistersEndpoints(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{From: "root", To: "child"})
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes after AddEdge, got %d", g.NodeCount())
	}
	kids := g.Children("root")
	if len(kids) != 1 || kids[0] != "child" {
		t.Errorf("Children(root) = %v", kids)
	}
}

func TestRootsAndCycleDetection(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "a", To: "c"})
	if roots := g.Roots(); len(roots) != 1 || roots[0] != "a" {
		t.Errorf("Roots = %v, want [a]", roots)
	}
	if g.HasCycle() {
		t.Error("acyclic graph reported a cycle")
	}

	g.AddEdge(Edge{From: "c", To: "a"}) // cycle
	if !g.HasCycle() {
		t.Error("cycle not detected")
	}
}

func TestBuildTreeRejectsMultiRoot(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: "a"})
	g.AddNode(Node{ID: "b"})
	if _, err := g.BuildTree(); err == nil {
		t.Error("multi-root should error")
	}
}

func TestBuildTreeRejectsCycle(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "a"})
	if _, err := g.BuildTree(); err == nil {
		t.Error("cycle should error")
	}
}

func TestTidyTreeBalanced(t *testing.T) {
	// Balanced tree:
	//        root
	//        /  \
	//       a    b
	//      / \  / \
	//     c  d e  f
	g := NewGraph()
	g.AddEdge(Edge{From: "root", To: "a"})
	g.AddEdge(Edge{From: "root", To: "b"})
	g.AddEdge(Edge{From: "a", To: "c"})
	g.AddEdge(Edge{From: "a", To: "d"})
	g.AddEdge(Edge{From: "b", To: "e"})
	g.AddEdge(Edge{From: "b", To: "f"})
	rootID, err := g.BuildTree()
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}
	pos, err := TidyTree(g, rootID, 10, 20)
	if err != nil {
		t.Fatalf("TidyTree: %v", err)
	}
	if len(pos) != 7 {
		t.Fatalf("expected 7 positions, got %d", len(pos))
	}
	// Root sits at horizontal midpoint of leaves (4 leaves at 0,10,20,30 → midpoint 15).
	if pos[0].ID != "root" || pos[0].X != 15 {
		t.Errorf("root pos = %+v, want X=15", pos[0])
	}
	// Y values march by verticalGap=20.
	for _, p := range pos {
		want := float64(p.Depth) * 20
		if p.Y != want {
			t.Errorf("node %s: Y=%v, want %v", p.ID, p.Y, want)
		}
	}
}

func TestTidyTreeLinearChain(t *testing.T) {
	// Linear: a → b → c → d.
	g := NewGraph()
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})
	g.AddEdge(Edge{From: "c", To: "d"})
	pos, err := TidyTree(g, "a", 10, 20)
	if err != nil {
		t.Fatalf("TidyTree: %v", err)
	}
	// Every node has exactly one child + one leaf at the end →
	// every X collapses to the leaf's X = 0.
	for _, p := range pos {
		if p.X != 0 {
			t.Errorf("linear chain node %s X=%v, want 0", p.ID, p.X)
		}
	}
}

func TestTidyTreeDeterministic(t *testing.T) {
	build := func() *Graph {
		g := NewGraph()
		g.AddEdge(Edge{From: "r", To: "a"})
		g.AddEdge(Edge{From: "r", To: "b"})
		g.AddEdge(Edge{From: "a", To: "x"})
		return g
	}
	first, err := TidyTree(build(), "r", 10, 20)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := TidyTree(build(), "r", 10, 20)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("non-deterministic at %d: %+v vs %+v", i, first[i], second[i])
		}
	}
}
