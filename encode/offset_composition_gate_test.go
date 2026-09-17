package encode_test

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// Offset scales across composition (E3-S2) — the epic's QA gate.
//
// encode/offset_composition_test.go pins the fold E3-S1 built. This
// file closes what it does not reach:
//
//   - the shared domain is the UNION of the children's category sets,
//     not one child's and not their intersection, which is the case
//     that actually separates shared from independent;
//   - the `independent` opt-out resolves visibly DIFFERENT domains per
//     child, asserted as two positions for one series rather than only
//     as a width;
//   - a facet dodges in EVERY cell off one grid-wide domain, and a
//     category absent from a cell still holds its sub-band open there;
//   - the ratified ordering guarantee — distinct values ascending,
//     unmoved by row order — survives the cross-child fold, which is
//     the property the fold was written not to route through
//     resolve.Unify in order to protect;
//   - the conflict warning covers `sort` and the reflective `scale`
//     fold, not only `scale.domain`, and stays silent under
//     `independent`;
//   - facet and repeat compositions binding no offset encode
//     byte-identically, as the layer case already does.
//
// Every assertion reads the COMPILED SCENE. A resolve entry that
// decodes and then moves no geometry is the silent no-op this channel
// exists to avoid.

// relX returns each rect's x displacement from its cell's plot rect,
// so cells at different grid positions compare directly.
func relX(t *testing.T, doc *scene.SceneDoc, cell int) []float64 {
	t.Helper()
	origin := doc.Grid.Cells[cell].Scene.Plot.X
	rects := cellRects(t, doc, cell)
	out := make([]float64, len(rects))
	for i, r := range rects {
		out[i] = r.X - origin
	}
	return out
}

// bandGrid returns the ascending sub-band leading edges of a slot
// divided n ways, given one observed rect. Used to state "these marks
// sit on one common n-slot grid" without hard-coding pixel literals.
func bandGrid(lead, width float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = lead + width*float64(i)
	}
	return out
}

// disjointLayerSpec layers two bar charts over one category whose
// offset value sets OVERLAP but do not coincide: layer 0 draws
// {a, b} and layer 1 draws {b, c}.
//
// This is the shape that separates the two resolutions. A spec where
// both children happen to see the same categories resolves the same
// domain either way and proves nothing; a strict-subset child proves
// the wider domain reached the narrower child but not that the union
// ran in both directions. Only a pair where each child contributes a
// category the other never saw does both.
func disjointLayerSpec(resolveBlock string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",` + resolveBlock + `
	 "layer":[
	  {"data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}},
	  {"data":{"values":[{"q":"Q1","s":"b","v":4},{"q":"Q1","s":"c","v":2}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}]}`
}

// The default, on the case that distinguishes it. Each layer knows
// two of the three series; shared resolution must divide the slot
// three ways in BOTH layers and seat "b" — the only series they share
// — at one position.
//
// A domain that intersected instead of unioning would leave one
// sub-band and collapse four bars onto each other; a domain taken
// from whichever child resolved first would divide one layer's slot
// two ways and the other's three.
func TestPrismOffsetSharedDomainUnionsDisjointLayers(t *testing.T) {
	doc := encodeCompositeJSON(t, disjointLayerSpec(""))

	first := layerRects(t, doc, 0)  // a, b
	second := layerRects(t, doc, 1) // b, c
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}

	// Three sub-bands, so each is a third of the slot. The slot is
	// read off the scene as three widths rather than written as a
	// literal, so an unrelated layout change cannot masquerade as an
	// offset regression.
	width := first[0].W
	for i, r := range append(append([]scene.RectGeom{}, first...), second...) {
		if !nearly(r.W, width) {
			t.Errorf("rect %d is %v wide, want %v — the layers divided the slot differently", i, r.W, width)
		}
	}

	// The shared series lands in one place.
	if !nearly(first[1].X, second[0].X) {
		t.Errorf("series \"b\" sits at x=%v in layer-0 but x=%v in layer-1; the domain was not shared",
			first[1].X, second[0].X)
	}

	// All three series sit on one common three-slot grid: a, b, c
	// ascending, adjacent, in that order.
	want := bandGrid(first[0].X, width, 3)
	got := []float64{first[0].X, first[1].X, second[1].X} // a, b, c
	for i := range want {
		if !nearly(got[i], want[i]) {
			t.Errorf("sub-band %d starts at %v, want %v — the union is not one contiguous three-slot grid", i, got[i], want[i])
		}
	}
}

// The opt-out, on the same spec. Each layer divides its own slot two
// ways from its own value set, so "b" — the second sub-band for
// layer 0 and the first for layer 1 — draws in two different places.
//
// Asserting the two positions, not only the two widths, is what makes
// this a statement about the DOMAIN rather than about the padding.
func TestPrismOffsetIndependentDomainsDifferPerLayer(t *testing.T) {
	doc := encodeCompositeJSON(t, disjointLayerSpec(
		`"resolve":{"scale":{"x_offset":"independent"}},`))

	first := layerRects(t, doc, 0)  // a, b
	second := layerRects(t, doc, 1) // b, c
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}

	if nearly(first[1].X, second[0].X) {
		t.Errorf("independent offset: series \"b\" sits at x=%v in both layers; each layer should divide its own slot",
			first[1].X)
	}
	// Two sub-bands each, so each layer's leading bar takes the same
	// leading half and its trailing bar the same trailing half.
	if !nearly(first[0].X, second[0].X) {
		t.Errorf("independent offset: leading sub-bands at %v and %v, want the same slot edge", first[0].X, second[0].X)
	}
	if !nearly(first[1].X, second[1].X) {
		t.Errorf("independent offset: trailing sub-bands at %v and %v, want the same offset", first[1].X, second[1].X)
	}
	// And each layer's pair together fills the slot the shared
	// resolution divided three ways.
	shared := layerRects(t, encodeCompositeJSON(t, disjointLayerSpec("")), 0)
	if !nearly(first[0].W, shared[0].W*1.5) {
		t.Errorf("independent sub-band width %v, want 1.5x the shared width %v (a half-slot vs a third)",
			first[0].W, shared[0].W)
	}
}

// The ratified default order — distinct values ASCENDING — is
// resolved over the UNION, not per child and not in first-seen order.
//
// Layer 0's rows arrive c-then-a and layer 1 contributes b. A
// first-seen union would seat c in sub-band 0; ascending seats a, b,
// c in that order regardless of which child or which row introduced
// them. This is the guarantee E3-S1 kept the fold out of
// resolve.Unify to protect, whose categorical arm is first-seen.
func TestPrismOffsetSharedDomainIsAscendingAcrossLayers(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "layer":[
	  {"data":{"values":[{"q":"Q1","s":"c","v":10},{"q":"Q1","s":"a","v":6}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}},
	  {"data":{"values":[{"q":"Q1","s":"b","v":4}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}]}`)

	first := layerRects(t, doc, 0)  // c, a — in row order
	second := layerRects(t, doc, 1) // b
	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("mark counts = %d / %d, want 2 / 1", len(first), len(second))
	}
	width := first[0].W
	byName := map[string]float64{"c": first[0].X, "a": first[1].X, "b": second[0].X}
	want := bandGrid(byName["a"], width, 3)
	for i, name := range []string{"a", "b", "c"} {
		if !nearly(byName[name], want[i]) {
			t.Errorf("series %q occupies sub-band at x=%v, want %v (ascending a,b,c across the union)",
				name, byName[name], want[i])
		}
	}
}

// crossLayerPermutedSpec lays out the same six (layer, series) rows in
// whatever order is given. Layer 0 carries {a, b, c} and layer 1
// carries {a, c}, so the union is {a, b, c} and layer 1 relies on the
// fold for its middle sub-band.
func crossLayerPermutedSpec(l0, l1 []string) string {
	values := map[string]int{"a": 10, "b": 6, "c": 2}
	rows := func(series []string) string {
		out := ""
		for i, s := range series {
			if i > 0 {
				out += ","
			}
			raw, err := json.Marshal(map[string]any{"q": "Q1", "s": s, "v": values[s]})
			if err != nil {
				panic(err)
			}
			out += string(raw)
		}
		return out
	}
	const enc = `"encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}`
	return `{"$schema":"urn:prism:schema:v1:spec","layer":[
	  {"data":{"values":[` + rows(l0) + `]},"mark":{"type":"bar"},` + enc + `},
	  {"data":{"values":[` + rows(l1) + `]},"mark":{"type":"bar"},` + enc + `}]}`
}

// The determinism clause, at the composition level. This is the
// invariant the fold exists to protect and nothing pinned it across
// children until now.
//
// The sub-band a series occupies must not depend on the order its
// rows arrived in — not within a child, and not across children. A
// downstream template is authored once and rendered against many
// datasets; if a series moved sub-band because an upstream sort
// changed, the same chart would read differently for no visible
// reason.
//
// A first-seen union (which is what routing through resolve.Unify
// would have produced) passes the baseline and fails every
// permutation below.
func TestPrismOffsetSharedDomainIsStableUnderRowPermutation(t *testing.T) {
	// subBands returns each series' sub-band leading edge, gathered
	// across both layers, plus the common sub-band width.
	subBands := func(l0, l1 []string) (map[string]float64, float64) {
		t.Helper()
		doc := encodeCompositeJSON(t, crossLayerPermutedSpec(l0, l1))
		first := layerRects(t, doc, 0)
		second := layerRects(t, doc, 1)
		if len(first) != len(l0) || len(second) != len(l1) {
			t.Fatalf("mark counts = %d / %d, want %d / %d", len(first), len(second), len(l0), len(l1))
		}
		out := map[string]float64{}
		width := first[0].W
		record := func(series []string, rects []scene.RectGeom) {
			for i, s := range series {
				if !nearly(rects[i].W, width) {
					t.Errorf("series %q is %v wide, want %v", s, rects[i].W, width)
				}
				if prev, seen := out[s]; seen && !nearly(prev, rects[i].X) {
					t.Errorf("series %q drew at x=%v in one layer and x=%v in the other", s, prev, rects[i].X)
				}
				out[s] = rects[i].X
			}
		}
		record(l0, first)
		record(l1, second)
		return out, width
	}

	wantPos, wantWidth := subBands([]string{"a", "b", "c"}, []string{"a", "c"})
	// Baseline sanity: three adjacent ascending sub-bands.
	lead := wantPos["a"]
	for i, name := range []string{"a", "b", "c"} {
		if !nearly(wantPos[name], lead+wantWidth*float64(i)) {
			t.Fatalf("baseline: series %q at %v, want %v", name, wantPos[name], lead+wantWidth*float64(i))
		}
	}

	permutations := []struct {
		name   string
		l0, l1 []string
	}{
		// Both layers reversed.
		{"reversed", []string{"c", "b", "a"}, []string{"c", "a"}},
		// "c" is the first row of the first layer, which is what a
		// first-seen domain would promote to sub-band 0.
		{"c first", []string{"c", "a", "b"}, []string{"a", "c"}},
		// The layer that does not carry "b" leads with the series
		// the other layer ends on.
		{"interleaved", []string{"b", "c", "a"}, []string{"c", "a"}},
	}
	for _, p := range permutations {
		gotPos, gotWidth := subBands(p.l0, p.l1)
		if !nearly(gotWidth, wantWidth) {
			t.Errorf("%s: sub-band width %v, want %v", p.name, gotWidth, wantWidth)
		}
		for name, want := range wantPos {
			if !nearly(gotPos[name], want) {
				t.Errorf("%s: series %q moved to x=%v, want %v — the shared domain followed row order",
					p.name, name, gotPos[name], want)
			}
		}
	}
}

// facetPermutedSpec facets two cells over `g` with three series. The
// rows are emitted in the order given.
//
// Every permutation must keep the first "L" row ahead of the first
// "R" row: the facet's own column order is first-seen, which is the
// documented rule for a position channel and is not what this test is
// about. Fixing the cell order isolates the sub-band question.
func facetPermutedSpec(rows [][2]string) string {
	values := map[string]int{"a": 10, "b": 6, "c": 2}
	body := `{"$schema":"urn:prism:schema:v1:spec","data":{"values":[`
	for i, r := range rows {
		if i > 0 {
			body += ","
		}
		raw, err := json.Marshal(map[string]any{"g": r[0], "q": "Q1", "s": r[1], "v": values[r[1]]})
		if err != nil {
			panic(err)
		}
		body += string(raw)
	}
	return body + `]},
	 "facet":{"column":{"field":"g","type":"nominal"}},
	 "spec":{"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}}`
}

// A facet dodges in every cell off ONE grid-wide sub-band order, and
// a cell that never saw a series still holds that series' sub-band
// open.
//
// Reserving the gap is what keeps the bars comparable: if the R cell
// gave its one series the whole slot, two bars of different widths
// would sit side by side representing the same measurement, which is
// the comparison a facet exists to make.
func TestPrismOffsetFacetDodgesEveryCellOnOneDomain(t *testing.T) {
	// L draws a, b, c; R draws only b — the MIDDLE sub-band, so a
	// cell that simply left-packed its one bar would be caught.
	doc := encodeCompositeJSON(t, facetPermutedSpec([][2]string{
		{"L", "a"}, {"L", "b"}, {"L", "c"}, {"R", "b"},
	}))
	if len(doc.Grid.Cells) != 2 {
		t.Fatalf("got %d facet cells, want 2", len(doc.Grid.Cells))
	}

	left := cellRects(t, doc, 0)
	right := cellRects(t, doc, 1)
	if len(left) != 3 || len(right) != 1 {
		t.Fatalf("mark counts = %d / %d, want 3 / 1", len(left), len(right))
	}

	width := left[0].W
	for i, r := range left {
		if !nearly(r.W, width) {
			t.Errorf("cell L rect %d is %v wide, want %v", i, r.W, width)
		}
	}
	if !nearly(right[0].W, width) {
		t.Errorf("cell R sub-band is %v wide, want cell L's %v — the cells divided their slots differently",
			right[0].W, width)
	}

	// Every cell is dodged: L's three bars are adjacent and ascending.
	leftRel := relX(t, doc, 0)
	for i := 1; i < len(leftRel); i++ {
		if !nearly(leftRel[i], leftRel[0]+width*float64(i)) {
			t.Errorf("cell L sub-band %d at %v from the plot edge, want %v", i, leftRel[i], leftRel[0]+width*float64(i))
		}
	}

	// R's only series is "b", the middle sub-band, and it sits at the
	// same displacement from its own plot edge as L's "b" does from
	// its own. The sub-bands for the absent "a" and "c" stay empty.
	rightRel := relX(t, doc, 1)
	if !nearly(rightRel[0], leftRel[1]) {
		t.Errorf("cell R drew series \"b\" %v from its plot edge, want %v — the absent sub-bands were not reserved",
			rightRel[0], leftRel[1])
	}
}

// The facet's grid-wide order is the ratified ascending one and does
// not follow row order either. Same guarantee as the layer case, on
// the path that resolves it from partitions rather than from layers.
func TestPrismOffsetFacetSharedDomainIsStableUnderRowPermutation(t *testing.T) {
	// cellSubBands returns, per cell, the series ordered by their
	// displacement from that cell's plot edge, plus each series'
	// displacement.
	cellSubBands := func(rows [][2]string) ([][]string, map[string]float64, float64) {
		t.Helper()
		doc := encodeCompositeJSON(t, facetPermutedSpec(rows))
		if len(doc.Grid.Cells) != 2 {
			t.Fatalf("got %d facet cells, want 2", len(doc.Grid.Cells))
		}
		// Rows reach their cell in table order, so the nth rect of a
		// cell is the nth row carrying that cell's facet value.
		perCell := map[string][]string{}
		for _, r := range rows {
			perCell[r[0]] = append(perCell[r[0]], r[1])
		}
		order := make([][]string, 2)
		shift := map[string]float64{}
		width := 0.0
		for ci, g := range []string{"L", "R"} {
			rel := relX(t, doc, ci)
			series := perCell[g]
			if len(rel) != len(series) {
				t.Fatalf("cell %s drew %d marks, want %d", g, len(rel), len(series))
			}
			if width == 0 {
				width = cellRects(t, doc, ci)[0].W
			}
			pairs := make([][2]any, len(rel))
			for i := range rel {
				pairs[i] = [2]any{rel[i], series[i]}
				shift[g+"/"+series[i]] = rel[i]
			}
			sort.Slice(pairs, func(a, b int) bool {
				return pairs[a][0].(float64) < pairs[b][0].(float64)
			})
			for _, p := range pairs {
				order[ci] = append(order[ci], p[1].(string))
			}
		}
		return order, shift, width
	}

	baseline := [][2]string{{"L", "a"}, {"L", "b"}, {"L", "c"}, {"R", "a"}, {"R", "c"}}
	wantOrder, wantShift, wantWidth := cellSubBands(baseline)
	if len(wantOrder[0]) != 3 || wantOrder[0][0] != "a" || wantOrder[0][1] != "b" || wantOrder[0][2] != "c" {
		t.Fatalf("baseline cell L sub-band order is %v, want [a b c]", wantOrder[0])
	}

	permutations := []struct {
		name string
		rows [][2]string
	}{
		{"series reversed", [][2]string{{"L", "c"}, {"L", "b"}, {"L", "a"}, {"R", "c"}, {"R", "a"}}},
		{"cells interleaved", [][2]string{{"L", "c"}, {"R", "c"}, {"L", "a"}, {"R", "a"}, {"L", "b"}}},
		{"b introduced last", [][2]string{{"L", "c"}, {"L", "a"}, {"R", "a"}, {"R", "c"}, {"L", "b"}}},
	}
	for _, p := range permutations {
		gotOrder, gotShift, gotWidth := cellSubBands(p.rows)
		if !nearly(gotWidth, wantWidth) {
			t.Errorf("%s: sub-band width %v, want %v", p.name, gotWidth, wantWidth)
		}
		for ci := range wantOrder {
			if len(gotOrder[ci]) != len(wantOrder[ci]) {
				t.Errorf("%s: cell %d has %d sub-bands, want %d", p.name, ci, len(gotOrder[ci]), len(wantOrder[ci]))
				continue
			}
			for i := range wantOrder[ci] {
				if gotOrder[ci][i] != wantOrder[ci][i] {
					t.Errorf("%s: cell %d sub-band order is %v, want %v", p.name, ci, gotOrder[ci], wantOrder[ci])
					break
				}
			}
		}
		for key, want := range wantShift {
			if !nearly(gotShift[key], want) {
				t.Errorf("%s: %s sits %v from its plot edge, want %v — the grid-wide domain followed row order",
					p.name, key, gotShift[key], want)
			}
		}
	}
}

// conflictingSortLayerSpec disagrees on `sort` rather than on
// `scale.domain`, so the non-scale half of the fold is covered too.
func conflictingSortLayerSpec(resolveBlock string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",` + resolveBlock + `
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	 "layer":[
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","sort":"descending"}}},
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","sort":"ascending"}}}]}`
}

// Two children disagreeing on `sort` are reported, and the first
// specified wins outright — both layers draw in layer-0's descending
// order, so the losing "ascending" is visibly not honoured.
func TestPrismOffsetConflictingSortWarns(t *testing.T) {
	doc := encodeCompositeJSON(t, conflictingSortLayerSpec(""))

	warns := offsetConfigConflicts(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("offset-config conflicts = %d, want exactly 1: %+v", len(warns), doc.Warnings)
	}
	if got := warns[0].Details["Property"]; got != "sort" {
		t.Errorf("conflict property = %v, want \"sort\"", got)
	}
	if got := warns[0].Details["Winner"]; got != "layer-0" {
		t.Errorf("conflict winner = %v, want \"layer-0\"", got)
	}
	if got := warns[0].Details["Channel"]; got != "x_offset" {
		t.Errorf("conflict channel = %v, want \"x_offset\"", got)
	}

	// Rows are a then b in both layers. Under layer-0's descending
	// order b takes the leading sub-band, so each layer's FIRST rect
	// (series "a") is the trailing one — and the two layers coincide.
	first := layerRects(t, doc, 0)
	second := layerRects(t, doc, 1)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}
	if !nearly(first[0].X, second[0].X) || !nearly(first[1].X, second[1].X) {
		t.Errorf("the losing sort was honoured: layer-0 x=[%v %v], layer-1 x=[%v %v]",
			first[0].X, first[1].X, second[0].X, second[1].X)
	}
	if !(first[0].X > first[1].X) {
		t.Errorf("series \"a\" at x=%v and \"b\" at x=%v; descending should put \"b\" first",
			first[0].X, first[1].X)
	}
}

// A knob on the offset channel's own `scale` block conflicts the same
// way. `domain` is already covered by E3-S1; a padding knob proves the
// fold really walks spec.Scale reflectively rather than special-casing
// one property.
func TestPrismOffsetConflictingScalePaddingWarns(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	 "layer":[
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","scale":{"padding_inner":0.5}}}},
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","scale":{"padding_inner":0.25}}}}]}`)

	warns := offsetConfigConflicts(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("offset-config conflicts = %d, want exactly 1: %+v", len(warns), doc.Warnings)
	}
	if got := warns[0].Details["Property"]; got != "padding_inner" {
		t.Errorf("conflict property = %v, want \"padding_inner\"", got)
	}
	// Layer-0's padding is what both layers draw with.
	first := layerRects(t, doc, 0)
	second := layerRects(t, doc, 1)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}
	for i := range first {
		if !nearly(first[i].W, second[i].W) || !nearly(first[i].X, second[i].X) {
			t.Errorf("rect %d: layer-0 x=%v w=%v, layer-1 x=%v w=%v — the losing padding was honoured",
				i, first[i].X, first[i].W, second[i].X, second[i].W)
		}
	}
}

// Under `independent` there is no shared scale to disagree about, so
// the same conflicting spec must produce NO conflict warning and each
// layer must honour its own `sort`.
//
// Warning on an independent resolution would be a false positive on a
// spec that is doing exactly what it asked for, which is how a
// warnings block stops being read.
func TestPrismOffsetIndependentResolutionDoesNotWarn(t *testing.T) {
	doc := encodeCompositeJSON(t, conflictingSortLayerSpec(
		`"resolve":{"scale":{"x_offset":"independent"}},`))

	if warns := offsetConfigConflicts(doc.Warnings); len(warns) != 0 {
		t.Fatalf("offset-config conflicts = %d under independent resolution: %+v", len(warns), warns)
	}

	first := layerRects(t, doc, 0)  // sort: descending
	second := layerRects(t, doc, 1) // sort: ascending
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}
	// Rows are a then b in both. Descending puts "a" second;
	// ascending puts it first. Each layer got its own answer.
	if !(first[0].X > first[1].X) {
		t.Errorf("layer-0 (descending) drew \"a\" at %v and \"b\" at %v", first[0].X, first[1].X)
	}
	if !(second[0].X < second[1].X) {
		t.Errorf("layer-1 (ascending) drew \"a\" at %v and \"b\" at %v", second[0].X, second[1].X)
	}
}

// A `facet` binding no offset is untouched by the fold: naming the
// channel in `resolve` changes nothing. The layer case is pinned by
// TestPrismOffsetUnboundCompositionIsByteIdentical; the two remaining
// composition operators are pinned here, because the fold reaches
// facet through a different resolver (buildSharedOffsetForFacet) and
// repeat through a third path that nils the override outright.
func TestPrismOffsetUnboundFacetIsByteIdentical(t *testing.T) {
	const body = `"data":{"values":[
	   {"g":"L","q":"Q1","v":10},{"g":"L","q":"Q2","v":4},
	   {"g":"R","q":"Q1","v":6},{"g":"R","q":"Q2","v":9}]},
	 "facet":{"column":{"field":"g","type":"nominal"}},
	 "spec":{"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"}}}}`

	assertResolveEntryIsInert(t, body)
}

func TestPrismOffsetUnboundRepeatIsByteIdentical(t *testing.T) {
	const body = `"data":{"values":[
	   {"q":"Q1","m1":10,"m2":3},{"q":"Q2","m1":6,"m2":9}]},
	 "repeat":{"column":["m1","m2"]},
	 "spec":{"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":{"repeat":"column"},"type":"quantitative"}}}}`

	assertResolveEntryIsInert(t, body)
}

// assertResolveEntryIsInert encodes one composition body twice — once
// plain, once with `resolve: {"scale": {"x_offset": "independent"}}`
// — and requires the two serialised scenes to match exactly.
//
// This is the encode-side statement of the "unbound is a byte-
// identical no-op" criterion. The committed goldens under
// render/svg/testdata and docs/src/gallery carry the same guarantee
// for the rendered bytes; this says why they did not move.
func assertResolveEntryIsInert(t *testing.T, body string) {
	t.Helper()
	plain := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",`+body)
	named := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",`+
		`"resolve":{"scale":{"x_offset":"independent"}},`+body)

	a, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain: %v", err)
	}
	b, err := json.Marshal(named)
	if err != nil {
		t.Fatalf("marshal with resolve: %v", err)
	}
	if string(a) != string(b) {
		t.Error("an unbound offset channel changed the scene when resolve named it")
	}
	if len(plain.Warnings) != 0 {
		t.Errorf("unbound offset produced warnings: %+v", plain.Warnings)
	}
}

// The horizontal mirror of the union case. A y band scale runs
// bottom-to-top and reports a NEGATIVE BandWidth(), so a shared
// sub-band order folded across layers has to survive a signed step as
// well as a shared one — a sign slip that cancels inside one layer
// can still separate two.
func TestPrismOffsetSharedDomainUnionsDisjointLayersHorizontal(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "layer":[
	  {"data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "y":{"field":"q","type":"nominal"},
	     "x":{"field":"v","type":"quantitative"},
	     "y_offset":{"field":"s","type":"nominal"}}},
	  {"data":{"values":[{"q":"Q1","s":"b","v":4},{"q":"Q1","s":"c","v":2}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "y":{"field":"q","type":"nominal"},
	     "x":{"field":"v","type":"quantitative"},
	     "y_offset":{"field":"s","type":"nominal"}}}]}`)

	first := layerRects(t, doc, 0)  // a, b
	second := layerRects(t, doc, 1) // b, c
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}

	height := first[0].H
	if height <= 0 {
		t.Fatalf("sub-band height %v is not drawable", height)
	}
	for i, r := range append(append([]scene.RectGeom{}, first...), second...) {
		if !nearly(r.H, height) {
			t.Errorf("rect %d is %v tall, want %v", i, r.H, height)
		}
	}
	if !nearly(first[1].Y, second[0].Y) {
		t.Errorf("series \"b\" sits at y=%v in layer-0 but y=%v in layer-1; the domain was not shared",
			first[1].Y, second[0].Y)
	}
	// Prism's y band runs bottom-to-top, so the ascending order
	// a, b, c walks DOWN the pixel axis: each sub-band's top edge is
	// one height above the previous one. (Vega-Lite's y band runs the
	// other way, which is why a side-by-side comparison looks
	// mirrored — a ratified divergence, not a defect.)
	got := []float64{first[0].Y, first[1].Y, second[1].Y} // a, b, c
	for i := 1; i < len(got); i++ {
		if !nearly(got[i], got[0]-height*float64(i)) {
			t.Errorf("sub-band %d starts at y=%v, want %v — the union is not one contiguous three-slot grid",
				i, got[i], got[0]-height*float64(i))
		}
	}
}
