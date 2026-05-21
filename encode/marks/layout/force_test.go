package layout

import (
	"math"
	"testing"
)

// TestForceLayoutDeterministic — a fixed seed must produce identical
// output across runs. Critical for the network mark's SVG goldens.
func TestForceLayoutDeterministic(t *testing.T) {
	build := func() *Graph {
		g := NewGraph()
		g.AddEdge(Edge{From: "a", To: "b"})
		g.AddEdge(Edge{From: "b", To: "c"})
		g.AddEdge(Edge{From: "c", To: "d"})
		g.AddEdge(Edge{From: "a", To: "c"})
		return g
	}
	first, err := ForceLayout(build(), ForceOpts{Iterations: 50, Seed: 7})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := ForceLayout(build(), ForceOpts{Iterations: 50, Seed: 7})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("len mismatch")
	}
	for i := range first {
		if first[i].X != second[i].X || first[i].Y != second[i].Y {
			t.Errorf("node %s drift between runs: %+v vs %+v", first[i].ID, first[i], second[i])
		}
	}
}

// TestForceLayoutBoundedToBox — every output position lands within
// the requested width × height bounding box.
func TestForceLayoutBoundedToBox(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})
	g.AddEdge(Edge{From: "c", To: "d"})
	pos, err := ForceLayout(g, ForceOpts{Iterations: 100, Width: 200, Height: 150, Seed: 1})
	if err != nil {
		t.Fatalf("ForceLayout: %v", err)
	}
	for _, p := range pos {
		if p.X < 0 || p.X > 200 {
			t.Errorf("%s: X = %v out of [0, 200]", p.ID, p.X)
		}
		if p.Y < 0 || p.Y > 150 {
			t.Errorf("%s: Y = %v out of [0, 150]", p.ID, p.Y)
		}
	}
}

// TestForceLayoutConvergesOnTriangle — three fully-connected nodes
// converge to roughly equidistant positions (within a tolerance).
func TestForceLayoutConvergesOnTriangle(t *testing.T) {
	g := NewGraph()
	g.AddEdge(Edge{From: "a", To: "b"})
	g.AddEdge(Edge{From: "b", To: "c"})
	g.AddEdge(Edge{From: "c", To: "a"})
	pos, err := ForceLayout(g, ForceOpts{Iterations: 500, LinkDistance: 30, Seed: 7})
	if err != nil {
		t.Fatalf("ForceLayout: %v", err)
	}
	idx := map[string]ForcePosition{}
	for _, p := range pos {
		idx[p.ID] = p
	}
	d := func(a, b string) float64 {
		return math.Hypot(idx[a].X-idx[b].X, idx[a].Y-idx[b].Y)
	}
	dab, dbc, dca := d("a", "b"), d("b", "c"), d("c", "a")
	avg := (dab + dbc + dca) / 3
	// Within 50% of the average — generous, but proves no degenerate
	// collapse. Real applications care about visual readability more
	// than exact equidistance.
	for _, dd := range []float64{dab, dbc, dca} {
		if math.Abs(dd-avg)/avg > 0.5 {
			t.Errorf("edge distance %v deviates from avg %v by >50%%", dd, avg)
		}
	}
}
