package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// categories collapses groups to their (category, detail…) tuples so
// assertions read as the partition they describe.
func categories(groups []rowGroup) [][]string {
	out := make([][]string, 0, len(groups))
	for _, g := range groups {
		tuple := append([]string{g.category}, g.detail...)
		out = append(out, tuple)
	}
	return out
}

func tuplesEqual(got, want [][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			return false
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				return false
			}
		}
	}
	return true
}

// TestPrismGroupRowsUngrouped pins the no-grouping-channel contract:
// one group, every row index in raw upstream order, empty category,
// nil color. This is the path every color-free line/area fixture
// takes, so a regression here moves committed goldens.
func TestPrismGroupRowsUngrouped(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x": []float64{3, 1, 2},
		"y": []float64{30, 10, 20},
	})
	groups, err := groupRows(Inputs{Table: tbl}, 3)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if got := groups[0].indices; len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Errorf("indices = %v, want [0 1 2] (raw upstream order)", got)
	}
	if groups[0].category != "" || groups[0].color != nil || groups[0].varName != "" {
		t.Errorf("group = %+v, want empty category / nil color / empty varName", groups[0])
	}
	if len(groups[0].detail) != 0 {
		t.Errorf("detail = %v, want empty", groups[0].detail)
	}
}

// TestPrismGroupRowsColorOnly pins the historical groupRowsByColor
// contract: first-appearance color order, indices in upstream order,
// per-group palette color resolved from Categories/Palette.
func TestPrismGroupRowsColorOnly(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"series": []string{"b", "a", "b", "a"},
	})
	in := Inputs{
		Table: tbl,
		Color: &ColorChannel{
			Field:      "series",
			Categories: []string{"a", "b"},
			Palette:    []*scene.Color{mustColor("#3b82f6"), mustColor("#ef4444")},
		},
	}
	groups, err := groupRows(in, 4)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	want := [][]string{{"b"}, {"a"}}
	if got := categories(groups); !tuplesEqual(got, want) {
		t.Fatalf("groups = %v, want %v (first-appearance color order)", got, want)
	}
	if got := groups[0].indices; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("groups[0].indices = %v, want [0 2]", got)
	}
	if groups[0].color == nil || groups[0].color.Hex() != "#ef4444" {
		t.Errorf("groups[0].color = %v, want #ef4444 (category b → palette[1])", groups[0].color)
	}
	if groups[1].color == nil || groups[1].color.Hex() != "#3b82f6" {
		t.Errorf("groups[1].color = %v, want #3b82f6 (category a → palette[0])", groups[1].color)
	}
}

// TestPrismGroupRowsDetailOnly: detail splits rows without touching
// color — no palette slot consumed, no resolved color at all.
func TestPrismGroupRowsDetailOnly(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"sensor": []string{"s1", "s2", "s1", "s2"},
	})
	groups, err := groupRows(Inputs{Table: tbl, Detail: []string{"sensor"}}, 4)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	want := [][]string{{"", "s1"}, {"", "s2"}}
	if got := categories(groups); !tuplesEqual(got, want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
	for i, g := range groups {
		if g.color != nil || g.varName != "" {
			t.Errorf("groups[%d] resolved a color (%v / %q); detail must consume no palette", i, g.color, g.varName)
		}
	}
	if got := groups[0].indices; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("groups[0].indices = %v, want [0 2]", got)
	}
}

// TestPrismGroupRowsDetailNumeric: a numeric detail field must split
// per distinct value. Color's string-only coercion would collapse
// every row into one bucket, which is why detail uses its own key.
func TestPrismGroupRowsDetailNumeric(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"run": []float64{1, 2, 1, 3},
	})
	groups, err := groupRows(Inputs{Table: tbl, Detail: []string{"run"}}, 4)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("len(groups) = %d, want 3 (one per distinct run)", len(groups))
	}
	if got := groups[0].indices; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("groups[0].indices = %v, want [0 2] (run=1)", got)
	}
}

// TestPrismGroupRowsColorAndDetail: both bound produces one group per
// distinct (color, detail) pair, and every group sharing a color
// emits contiguously in legend (first-appearance) color order.
func TestPrismGroupRowsColorAndDetail(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"region": []string{"east", "west", "east", "west"},
		"store":  []string{"s1", "s1", "s2", "s2"},
	})
	in := Inputs{
		Table:  tbl,
		Detail: []string{"store"},
		Color: &ColorChannel{
			Field:      "region",
			Categories: []string{"east", "west"},
			Palette:    []*scene.Color{mustColor("#3b82f6"), mustColor("#ef4444")},
		},
	}
	groups, err := groupRows(in, 4)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	want := [][]string{
		{"east", "s1"}, {"east", "s2"},
		{"west", "s1"}, {"west", "s2"},
	}
	if got := categories(groups); !tuplesEqual(got, want) {
		t.Fatalf("groups = %v, want %v (color outer, detail inner)", got, want)
	}
	// Both east groups share the east palette entry — detail never
	// advances the palette.
	for _, i := range []int{0, 1} {
		if groups[i].color == nil || groups[i].color.Hex() != "#3b82f6" {
			t.Errorf("groups[%d].color = %v, want #3b82f6", i, groups[i].color)
		}
	}
	for _, i := range []int{2, 3} {
		if groups[i].color == nil || groups[i].color.Hex() != "#ef4444" {
			t.Errorf("groups[%d].color = %v, want #ef4444", i, groups[i].color)
		}
	}
}

// TestPrismGroupRowsMultipleDetailFields: encoding.detail accepts an
// array; every field participates in the grouping tuple, in order.
func TestPrismGroupRowsMultipleDetailFields(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"site": []string{"a", "a", "b", "b"},
		"unit": []string{"u1", "u2", "u1", "u2"},
	})
	groups, err := groupRows(Inputs{Table: tbl, Detail: []string{"site", "unit"}}, 4)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	want := [][]string{
		{"", "a", "u1"}, {"", "a", "u2"},
		{"", "b", "u1"}, {"", "b", "u2"},
	}
	if got := categories(groups); !tuplesEqual(got, want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
}

// TestPrismGroupRowsKeyCollision: components are length-prefixed, so
// two tuples whose naive concatenation would collide stay distinct.
func TestPrismGroupRowsKeyCollision(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"left":  []string{"a", "a|b"},
		"right": []string{"b|c", "c"},
	})
	groups, err := groupRows(Inputs{Table: tbl, Detail: []string{"left", "right"}}, 2)
	if err != nil {
		t.Fatalf("groupRows: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2 (tuple keys must not collide)", len(groups))
	}
}

// TestPrismGroupRowsMissingDetailField surfaces PRISM_ENCODE_001 when
// the detail column isn't on the upstream table.
func TestPrismGroupRowsMissingDetailField(t *testing.T) {
	tbl := buildTable(t, map[string]any{"x": []float64{1, 2}})
	if _, err := groupRows(Inputs{Table: tbl, Detail: []string{"nope"}}, 2); err == nil {
		t.Fatal("groupRows: want an error for a missing detail column, got nil")
	}
}

// TestPrismEncodeLineDetailOnly: a detail-only line splits into one
// polyline per distinct value, x-sorted within the group, and leaves
// the caller's Style untouched (no per-group stroke).
func TestPrismEncodeLineDetailOnly(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x":      []float64{2, 1, 1, 3, 3, 2},
		"y":      []float64{20, 10, 100, 30, 300, 200},
		"sensor": []string{"a", "a", "b", "a", "b", "b"},
	})
	xs := &linScale{dmin: 1, dmax: 3, rmin: 1, rmax: 3}
	ys := &linScale{dmin: 0, dmax: 300, rmin: 0, rmax: 300}
	base := scene.Style{StrokeWidth: 1.5, Stroke: mustColor("#111111")}
	marksOut, _, err := Encode("line", Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: xs},
		Y:      Channel{Field: "y", Scale: ys},
		Layout: plotRect(),
		Style:  base,
		Detail: []string{"sensor"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marksOut) != 2 {
		t.Fatalf("len(marks) = %d, want 2 (one polyline per sensor)", len(marksOut))
	}
	wantA := [][2]float64{{1, 10}, {2, 20}, {3, 30}}
	wantB := [][2]float64{{1, 100}, {2, 200}, {3, 300}}
	if got := marksOut[0].Line.Points; !pointsEqual(got, wantA) {
		t.Errorf("marks[0] points = %v, want %v (sorted by x)", got, wantA)
	}
	if got := marksOut[1].Line.Points; !pointsEqual(got, wantB) {
		t.Errorf("marks[1] points = %v, want %v (sorted by x)", got, wantB)
	}
	for i, m := range marksOut {
		if m.Style.Stroke == nil || m.Style.Stroke.Hex() != "#111111" {
			t.Errorf("marks[%d].Style.Stroke = %v, want the unmodified #111111 — detail must not restyle", i, m.Style.Stroke)
		}
		if m.Style.StrokeVar != "" {
			t.Errorf("marks[%d].Style.StrokeVar = %q, want empty", i, m.Style.StrokeVar)
		}
	}
}

// TestPrismEncodeAreaColorAndDetail: color + detail compose on area —
// one ribbon per pair, both ribbons of a color sharing its fill.
func TestPrismEncodeAreaColorAndDetail(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"day":    []float64{1, 2, 1, 2, 1, 2, 1, 2},
		"vol":    []float64{10, 20, 11, 21, 100, 200, 101, 201},
		"region": []string{"east", "east", "east", "east", "west", "west", "west", "west"},
		"store":  []string{"s1", "s1", "s2", "s2", "s1", "s1", "s2", "s2"},
	})
	xs := &linScale{dmin: 1, dmax: 2, rmin: 1, rmax: 2}
	ys := &linScale{dmin: 0, dmax: 300, rmin: 0, rmax: 300}
	marksOut, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "day", Scale: xs},
		Y:      Channel{Field: "vol", Scale: ys},
		Layout: plotRect(),
		Detail: []string{"store"},
		Color: &ColorChannel{
			Field:      "region",
			Categories: []string{"east", "west"},
			Palette:    []*scene.Color{mustColor("#3b82f6"), mustColor("#ef4444")},
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marksOut) != 4 {
		t.Fatalf("len(marks) = %d, want 4 (one ribbon per region×store pair)", len(marksOut))
	}
	wantFill := []string{"#3b82f6", "#3b82f6", "#ef4444", "#ef4444"}
	for i, want := range wantFill {
		if marksOut[i].Style.Fill == nil || marksOut[i].Style.Fill.Hex() != want {
			t.Errorf("marks[%d].Style.Fill = %v, want %s", i, marksOut[i].Style.Fill, want)
		}
	}
}
