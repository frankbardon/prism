package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// TestPrismEncodeLineOrderedKeepsTableOrder pins the path sense of the
// order channel (E5-S4): with Ordered set, a grouped line traces its
// points in table order instead of re-sorting them by x.
func TestPrismEncodeLineOrderedKeepsTableOrder(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x":      []float64{2, 3, 1},
		"y":      []float64{20, 30, 10},
		"sensor": []string{"a", "a", "a"},
	})
	xs := &linScale{dmin: 1, dmax: 3, rmin: 1, rmax: 3}
	ys := &linScale{dmin: 0, dmax: 30, rmin: 0, rmax: 30}

	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: xs},
		Y:      Channel{Field: "y", Scale: ys},
		Layout: plotRect(),
		Style:  scene.Style{StrokeWidth: 1.5},
		Detail: []string{"sensor"},
	}

	// Default: grouped, so points sort left-to-right.
	def, _, err := Encode("line", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	wantSorted := [][2]float64{{1, 10}, {2, 20}, {3, 30}}
	if got := def[0].Line.Points; !pointsEqual(got, wantSorted) {
		t.Fatalf("default points = %v, want %v (sorted by x)", got, wantSorted)
	}

	// Ordered: the plan already sequenced the rows, so keep them.
	in.Ordered = true
	ord, _, err := Encode("line", in)
	if err != nil {
		t.Fatalf("Encode ordered: %v", err)
	}
	wantTable := [][2]float64{{2, 20}, {3, 30}, {1, 10}}
	if got := ord[0].Line.Points; !pointsEqual(got, wantTable) {
		t.Fatalf("ordered points = %v, want %v (table order)", got, wantTable)
	}
}

// TestPrismEncodeAreaOrderedKeepsTableOrder is the area twin.
func TestPrismEncodeAreaOrderedKeepsTableOrder(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x":      []float64{2, 3, 1},
		"y":      []float64{20, 30, 10},
		"sensor": []string{"a", "a", "a"},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: &linScale{dmin: 1, dmax: 3, rmin: 1, rmax: 3}},
		Y:      Channel{Field: "y", Scale: &linScale{dmin: 0, dmax: 30, rmin: 0, rmax: 30}},
		Layout: plotRect(),
		Style:  scene.Style{},
		Detail: []string{"sensor"},
	}

	def, _, err := Encode("area", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := def[0].Area.Upper[0][0]; got != 1 {
		t.Fatalf("default first upper x = %v, want 1 (sorted by x)", got)
	}

	in.Ordered = true
	ord, _, err := Encode("area", in)
	if err != nil {
		t.Fatalf("Encode ordered: %v", err)
	}
	if got := ord[0].Area.Upper[0][0]; got != 2 {
		t.Fatalf("ordered first upper x = %v, want 2 (table order)", got)
	}
}

// TestPrismEncodeLineUngroupedIgnoresOrdered pins that the flag
// changes nothing for the ungrouped path, which never sorted anyway —
// that path backs every color-free line golden.
func TestPrismEncodeLineUngroupedIgnoresOrdered(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x": []float64{2, 3, 1},
		"y": []float64{20, 30, 10},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: &linScale{dmin: 1, dmax: 3, rmin: 1, rmax: 3}},
		Y:      Channel{Field: "y", Scale: &linScale{dmin: 0, dmax: 30, rmin: 0, rmax: 30}},
		Layout: plotRect(),
	}
	want := [][2]float64{{2, 20}, {3, 30}, {1, 10}}
	for _, ordered := range []bool{false, true} {
		in.Ordered = ordered
		out, _, err := Encode("line", in)
		if err != nil {
			t.Fatalf("Encode(ordered=%v): %v", ordered, err)
		}
		if got := out[0].Line.Points; !pointsEqual(got, want) {
			t.Fatalf("ordered=%v points = %v, want %v", ordered, got, want)
		}
	}
}
