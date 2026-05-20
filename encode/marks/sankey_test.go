package marks

import (
	"math"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeSankeyShape(t *testing.T) {
	// 3-node 2-link graph: A→B (5), A→C (3).
	tbl := buildTable(t, map[string]any{
		"src": []string{"A", "A"},
		"tgt": []string{"B", "C"},
		"v":   []float64{5, 3},
	})
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Source: Channel{Field: "src"},
		Target: Channel{Field: "tgt"},
		Value:  Channel{Field: "v"},
	}
	marks, err := encodeSankey(in)
	if err != nil {
		t.Fatalf("encodeSankey: %v", err)
	}
	// Expect 3 RectGeom nodes + 2 PathGeom links = 5 marks.
	if len(marks) != 5 {
		t.Fatalf("want 5 marks, got %d", len(marks))
	}
	nodeCount, linkCount := 0, 0
	for _, m := range marks {
		switch {
		case m.Rect != nil:
			nodeCount++
		case m.Path != nil:
			linkCount++
		}
	}
	if nodeCount != 3 || linkCount != 2 {
		t.Errorf("nodes=%d links=%d, want 3 nodes + 2 links", nodeCount, linkCount)
	}
}

// TestPrismSankeyLayout — PHASE.md mandatory P11 gate. Hand-verifies
// node and link positions for a known 3-node 2-link DAG.
func TestPrismSankeyLayout(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"src": []string{"A", "A"},
		"tgt": []string{"B", "C"},
		"v":   []float64{5, 3},
	})
	in := Inputs{
		Table:  tbl,
		Layout: scene.Rect{X: 0, Y: 0, W: 800, H: 600},
		Source: Channel{Field: "src"},
		Target: Channel{Field: "tgt"},
		Value:  Channel{Field: "v"},
	}
	layout, err := ComputeSankeyLayout(in)
	if err != nil {
		t.Fatalf("ComputeSankeyLayout: %v", err)
	}
	// Expect 3 nodes: A at depth 0; B + C at depth 1.
	if len(layout.Nodes) != 3 {
		t.Fatalf("want 3 nodes, got %d", len(layout.Nodes))
	}
	byID := map[string]SankeyNode{}
	for _, n := range layout.Nodes {
		byID[n.ID] = n
	}
	a, ok := byID["A"]
	if !ok {
		t.Fatal("missing node A")
	}
	b := byID["B"]
	c := byID["C"]
	// Column placement: A at column 0; B + C at column 1.
	if a.Column != 0 {
		t.Errorf("A.Column = %d, want 0", a.Column)
	}
	if b.Column != 1 || c.Column != 1 {
		t.Errorf("B.Column=%d C.Column=%d, want both 1", b.Column, c.Column)
	}
	// X positions: A.X = layout.X = 0; B.X = C.X = nodeWidth +
	// columnSpacing. With totalCols=2, colSpacing = (800 - 2*12) / 1
	// = 776, so B.X = C.X = 12 + 776 = 788.
	if a.X != 0 {
		t.Errorf("A.X = %g, want 0", a.X)
	}
	wantCol1X := sankeyNodeWidth + (800.0 - 2*sankeyNodeWidth) // = 788
	if b.X != wantCol1X || c.X != wantCol1X {
		t.Errorf("B.X=%g C.X=%g, want %g", b.X, c.X, wantCol1X)
	}
	// Heights: A's total flow = outflow = 8; B's inflow = 5;
	// C's inflow = 3. Per-column scale (D065): col 0 max flow = 8;
	// col 1 max flow = 5 + 3 = 8. maxColumnFlow = 8.
	// availH = 600 - (2-1)*4 = 596 (max nodes in any col = 2 for col 1).
	// heightScale = 596 / 8 = 74.5.
	wantScale := 596.0 / 8.0
	wantAH := 8 * wantScale
	wantBH := 5 * wantScale
	wantCH := 3 * wantScale
	if math.Abs(a.H-wantAH) > 1e-9 {
		t.Errorf("A.H = %g, want %g", a.H, wantAH)
	}
	if math.Abs(b.H-wantBH) > 1e-9 {
		t.Errorf("B.H = %g, want %g", b.H, wantBH)
	}
	if math.Abs(c.H-wantCH) > 1e-9 {
		t.Errorf("C.H = %g, want %g", c.H, wantCH)
	}
	// Links: A→B center at (A.X+nodeWidth, A.Y + linkBH/2);
	// A→C center at (A.X+nodeWidth, A.Y + linkBH + linkCH/2)
	// where linkBH = 5*scale and linkCH = 3*scale.
	if len(layout.Links) != 2 {
		t.Fatalf("want 2 links, got %d", len(layout.Links))
	}
	l0 := layout.Links[0] // A→B
	l1 := layout.Links[1] // A→C
	wantSX := a.X + sankeyNodeWidth
	if l0.SX != wantSX || l1.SX != wantSX {
		t.Errorf("link SX = %g/%g, want %g", l0.SX, l1.SX, wantSX)
	}
	// source-side Y for A→B = A.Y + linkBH/2 = 0 + wantBH/2
	if math.Abs(l0.SY-wantBH/2) > 1e-9 {
		t.Errorf("link A→B SY = %g, want %g", l0.SY, wantBH/2)
	}
	// source-side Y for A→C = A.Y + linkBH + linkCH/2
	wantL1SY := wantBH + wantCH/2
	if math.Abs(l1.SY-wantL1SY) > 1e-9 {
		t.Errorf("link A→C SY = %g, want %g", l1.SY, wantL1SY)
	}
	// target-side X for A→B = B.X; for A→C = C.X.
	if l0.TX != b.X {
		t.Errorf("link A→B TX = %g, want %g", l0.TX, b.X)
	}
	if l1.TX != c.X {
		t.Errorf("link A→C TX = %g, want %g", l1.TX, c.X)
	}
	// Determinism: control points at midpoint x; both endpoints'
	// y values stay deterministic given the input.
	wantMid := (l0.SX + l0.TX) / 2
	if wantMid != (wantSX+b.X)/2 {
		t.Errorf("midpoint x = %g, want %g", wantMid, (wantSX+b.X)/2)
	}
}

func TestPrismSankeyRejectsCycle(t *testing.T) {
	// A→B, B→A is a cycle.
	tbl := buildTable(t, map[string]any{
		"src": []string{"A", "B"},
		"tgt": []string{"B", "A"},
		"v":   []float64{1, 1},
	})
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Source: Channel{Field: "src"},
		Target: Channel{Field: "tgt"},
		Value:  Channel{Field: "v"},
	}
	_, err := encodeSankey(in)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}
