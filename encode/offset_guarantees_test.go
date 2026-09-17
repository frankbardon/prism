package encode_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// Offset position channels (E1-S5) — the epic's QA gate.
//
// encode/offset_geometry_test.go pins the geometry E1-S4 built. This
// file closes the three guarantees it does not reach:
//
//   - the whole compiled scene, not just the rect geometry, is
//     unchanged when one distinct offset value reduces the
//     subdivision to a single sub-band;
//   - sub-band assignment survives an arbitrary permutation of the
//     rows, not only the two-row swap;
//   - the aggregated Arc-shaped spec — offset + color + a synthetic
//     aggregate — dodges and does NOT stack, which is the integration
//     proof that E1-S2's stack suppression and E1-S3's group-by
//     widening actually compose.
//
// Everything here reads the COMPILED SCENE. A spec can carry a channel
// nothing consumes; asserting on the spec would prove nothing.

// offsetSceneJSON drives a flat spec through Build → Execute → Encode
// and returns the serialised SceneDoc. Serialising the whole document
// — not a hand-picked field list — is what makes an "unchanged"
// assertion honest: a scale, a legend, an axis or a warning that moved
// shows up as a diff instead of being quietly out of scope.
func offsetSceneJSON(t *testing.T, body string) []byte {
	t.Helper()
	doc := offsetSceneDoc(t, body)
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal scene: %v", err)
	}
	return raw
}

// offsetSceneDoc is offsetRects' sibling for the cases that need more
// than the first layer's rects.
func offsetSceneDoc(t *testing.T, body string) *scene.SceneDoc {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return doc
}

// TestPrismOffsetSingleValueSceneIsByteIdentical is the strongest form
// of the no-op guarantee. TestPrismOffsetSingleValueIsIdentity compares
// rect geometry; this compares the entire serialised SceneDoc, so a
// changed scale domain, an extra legend, a shifted axis tick or a
// spurious warning all count as a regression.
//
// The guarantee holds because the offset scale's padding defaults to
// inner 0 / outer 0 (encode/offset.go): one sub-band with no padding
// IS the parent slot, at zero displacement. If this test fails after a
// padding default changes, the padding default is the defect, not the
// test.
func TestPrismOffsetSingleValueSceneIsByteIdentical(t *testing.T) {
	const rows = `"data":{"values":[
	   {"q":"Q1","s":"only","v":10},
	   {"q":"Q2","s":"only","v":14},
	   {"q":"Q3","s":"only","v":3}]},`
	const base = `{"$schema":"urn:prism:schema:v1:spec",` + rows + `
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"}`

	plain := offsetSceneJSON(t, base+`}}`)
	dodged := offsetSceneJSON(t, base+`,"x_offset":{"field":"s","type":"nominal"}}}`)
	if string(plain) != string(dodged) {
		t.Errorf("one distinct offset value changed the scene\n without offset: %s\n with offset:    %s", plain, dodged)
	}
}

// TestPrismOffsetSingleValueSceneIsByteIdenticalHorizontal mirrors it
// onto a band on y, where the sub-band displacement and width are both
// negative. A sign slip that cancels out for a full slot in the
// vertical case can still survive here.
func TestPrismOffsetSingleValueSceneIsByteIdenticalHorizontal(t *testing.T) {
	const rows = `"data":{"values":[
	   {"q":"Q1","s":"only","v":10},
	   {"q":"Q2","s":"only","v":14},
	   {"q":"Q3","s":"only","v":3}]},`
	const base = `{"$schema":"urn:prism:schema:v1:spec",` + rows + `
	 "mark":{"type":"bar"},
	 "encoding":{
	   "y":{"field":"q","type":"nominal"},
	   "x":{"field":"v","type":"quantitative"}`

	plain := offsetSceneJSON(t, base+`}}`)
	dodged := offsetSceneJSON(t, base+`,"y_offset":{"field":"s","type":"nominal"}}}`)
	if string(plain) != string(dodged) {
		t.Errorf("one distinct offset value changed the scene\n without offset: %s\n with offset:    %s", plain, dodged)
	}
}

// permutedOffsetSpec builds the same four-row chart with the rows in
// the order given. Each entry names a (category, series) pair.
func permutedOffsetSpec(order [][2]string) string {
	body := `{"$schema":"urn:prism:schema:v1:spec","data":{"values":[`
	values := map[string]int{"Q1a": 10, "Q1b": 6, "Q1c": 2, "Q2a": 14, "Q2b": 9, "Q2c": 5}
	for i, row := range order {
		if i > 0 {
			body += ","
		}
		raw, err := json.Marshal(map[string]any{
			"q": row[0], "s": row[1], "v": values[row[0]+row[1]],
		})
		if err != nil {
			panic(err)
		}
		body += string(raw)
	}
	return body + `]},"mark":{"type":"bar"},"encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`
}

// TestPrismOffsetIsDeterministicUnderRowPermutation is the determinism
// clause at full strength. TestPrismOffsetOrderIsNotTableOrder swaps
// two rows of one category; this shuffles six rows across two
// categories and three series, and requires every series to take the
// SAME sub-band of its slot in every permutation.
//
// Scoped to the sub-band on purpose. The parent x band scale still
// orders its own categories first-seen, so reversing the rows does
// move Q1 and Q2 past each other — that is the documented rule for a
// position channel and is not what this guarantee is about. What must
// not move is which sub-band a series occupies inside whichever slot
// its category ended up in.
//
// First-seen ordering would pass the two-row swap for the category
// that happens to come first and fails here. The divergence is
// ratified: an offset decides which sub-band a series sits in, so
// deriving it from row order would make the same data draw
// differently after an upstream sort.
func TestPrismOffsetIsDeterministicUnderRowPermutation(t *testing.T) {
	// subBands returns, per category, the series in ascending pixel
	// order, plus the displacement of each series from its slot's
	// leading edge.
	subBands := func(order [][2]string) (map[string][]string, map[string]float64, float64) {
		t.Helper()
		rects := offsetRects(t, permutedOffsetSpec(order))
		if len(rects) != len(order) {
			t.Fatalf("got %d rects, want %d", len(rects), len(order))
		}
		byCat := map[string][][2]any{}
		width := rects[0].W
		for i, r := range rects {
			if !nearly(r.W, width) {
				t.Errorf("rect %d is %v wide, want %v", i, r.W, width)
			}
			byCat[order[i][0]] = append(byCat[order[i][0]], [2]any{r.X, order[i][1]})
		}
		seq := map[string][]string{}
		shift := map[string]float64{}
		for cat, entries := range byCat {
			sort.Slice(entries, func(a, b int) bool {
				return entries[a][0].(float64) < entries[b][0].(float64)
			})
			lead := entries[0][0].(float64)
			for _, e := range entries {
				series := e[1].(string)
				seq[cat] = append(seq[cat], series)
				shift[cat+"/"+series] = e[0].(float64) - lead
			}
		}
		return seq, shift, width
	}

	canonical := [][2]string{
		{"Q1", "a"}, {"Q1", "b"}, {"Q1", "c"},
		{"Q2", "a"}, {"Q2", "b"}, {"Q2", "c"},
	}
	wantSeq, wantShift, wantWidth := subBands(canonical)
	for cat, seq := range wantSeq {
		if len(seq) != 3 || seq[0] != "a" || seq[1] != "b" || seq[2] != "c" {
			t.Fatalf("baseline sub-band order for %s is %v, want [a b c]", cat, seq)
		}
	}

	permutations := [][][2]string{
		// Reversed.
		{{"Q2", "c"}, {"Q2", "b"}, {"Q2", "a"}, {"Q1", "c"}, {"Q1", "b"}, {"Q1", "a"}},
		// Interleaved by series, so no category's rows are contiguous.
		{{"Q2", "b"}, {"Q1", "c"}, {"Q2", "a"}, {"Q1", "a"}, {"Q2", "c"}, {"Q1", "b"}},
		// Series "c" seen first, which is what a first-seen domain
		// would promote to sub-band 0.
		{{"Q1", "c"}, {"Q2", "c"}, {"Q1", "b"}, {"Q2", "b"}, {"Q1", "a"}, {"Q2", "a"}},
	}
	for i, order := range permutations {
		gotSeq, gotShift, gotWidth := subBands(order)
		if !nearly(gotWidth, wantWidth) {
			t.Errorf("permutation %d: sub-band width %v, want %v", i, gotWidth, wantWidth)
		}
		for cat, want := range wantSeq {
			got := gotSeq[cat]
			if len(got) != len(want) {
				t.Errorf("permutation %d: %s has %d sub-bands, want %d", i, cat, len(got), len(want))
				continue
			}
			for j := range want {
				if got[j] != want[j] {
					t.Errorf("permutation %d: %s sub-band order is %v, want %v", i, cat, got, want)
					break
				}
			}
		}
		for key, want := range wantShift {
			if !nearly(gotShift[key], want) {
				t.Errorf("permutation %d: %s sits %v from its slot edge, want %v", i, key, gotShift[key], want)
			}
		}
	}
}

// TestPrismOffsetAggregatedEndToEnd is the integration proof for epic
// E1, on the spec shape the downstream request was written against.
//
// Three things have to compose for it to pass, and each of them is a
// separate story's work:
//
//   - E1-S3 — the offset field joins the synthetic aggregate's
//     group-by, so `sum(value)` collapses to six rows and not three.
//     A dropped offset field shows up here as three rects.
//   - E1-S2 — an implicit stack yields to a bound offset. A bar with
//     x band + quantitative y + a color grouping stacks by default;
//     if it still did, the six rects would sit at three distinct
//     baselines instead of one.
//   - E1-S4 — the geometry. Equal widths, adjacent sub-bands, inside
//     the parent slot.
func TestPrismOffsetAggregatedEndToEnd(t *testing.T) {
	const arc = `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [
	    {"metric": "familiarity", "series": "Nike",     "value": 72},
	    {"metric": "familiarity", "series": "Category", "value": 64},
	    {"metric": "regard",      "series": "Nike",     "value": 68},
	    {"metric": "regard",      "series": "Category", "value": 59},
	    {"metric": "trust",       "series": "Nike",     "value": 55},
	    {"metric": "trust",       "series": "Category", "value": 61}
	  ]},
	  "mark": {"type": "bar"},
	  "encoding": {
	    "x":        {"field": "metric", "type": "nominal"},
	    "y":        {"aggregate": "sum", "field": "value", "type": "quantitative"},
	    "color":    {"field": "series", "type": "nominal"},
	    "x_offset": {"field": "series", "type": "nominal"}
	  }
	}`

	doc := offsetSceneDoc(t, arc)
	cell := doc.Grid.Cells[0].Scene
	rects := offsetRects(t, arc)
	// One rect per input row. Three would mean the offset field was
	// dropped from the synthetic aggregate's group-by and the two
	// series summed together.
	if len(rects) != 6 {
		t.Fatalf("got %d rects, want 6 (one per row)", len(rects))
	}

	// NOT stacked: every bar rises from the plot rect's bottom edge. A
	// stacked chart seats the second series on top of the first, so
	// its Y+H would be the first series' Y — three distinct baselines
	// instead of one. Read off the scene rather than written as a
	// literal so an unrelated layout change does not masquerade as a
	// stacking regression.
	baseline := cell.Plot.Y + cell.Plot.H
	for i, r := range rects {
		if !nearly(r.Y+r.H, baseline) {
			t.Errorf("rect %d has baseline %v, want %v — the chart is stacked, not dodged",
				i, r.Y+r.H, baseline)
		}
		if r.H <= 0 {
			t.Errorf("rect %d has height %v, which is not drawable", i, r.H)
		}
	}

	// Equal widths across every group, which is what makes the bars
	// comparable at all.
	for i, r := range rects {
		if !nearly(r.W, rects[0].W) {
			t.Errorf("rect %d is %v wide, want %v", i, r.W, rects[0].W)
		}
	}

	// Three groups of two adjacent sub-bands. Rows arrive metric-major
	// and the offset domain is {Category, Nike} ascending, so within
	// each pair the SECOND row ("Nike") takes the trailing sub-band.
	for g := 0; g < 3; g++ {
		lead, trail := rects[2*g+1], rects[2*g]
		if !nearly(trail.X, lead.X+lead.W) {
			t.Errorf("group %d does not dodge: %v then %v (width %v)", g, lead.X, trail.X, lead.W)
		}
	}
	// The three groups are evenly spaced and strictly increasing —
	// a group that failed to move would collapse two metrics onto one
	// slot.
	step := rects[3].X - rects[1].X
	if step <= 0 {
		t.Fatalf("group step %v is not positive", step)
	}
	if !nearly(rects[5].X-rects[3].X, step) {
		t.Errorf("group spacing is uneven: %v then %v", step, rects[5].X-rects[3].X)
	}

	// The color channel still resolves per series: the two members of
	// a group must not share a fill, or the dodge is unreadable.
	marksOut := cell.Layers[0].Marks
	if len(marksOut) != 6 {
		t.Fatalf("got %d marks, want 6", len(marksOut))
	}
	if fillsEqual(marksOut[0].Style, marksOut[1].Style) {
		t.Error("both series in a group share one fill; the color channel did not survive the offset")
	}
	// And the same series keeps one fill across groups.
	if !fillsEqual(marksOut[0].Style, marksOut[2].Style) {
		t.Error(`series "Category" changed fill between groups`)
	}
	// An offset-driven chart must not report itself inert.
	for _, w := range doc.Warnings {
		if w.Code == "PRISM_WARN_CHANNEL_INERT" {
			t.Errorf("a working offset channel reported itself inert: %s", w.Message)
		}
	}
}

// fillsEqual compares the two ways a fill reaches the renderer — a
// literal colour and a theme/palette reference — so a palette-backed
// style is not read as "no fill" and silently passed.
func fillsEqual(a, b scene.Style) bool {
	fill := func(s scene.Style) (string, scene.Color) {
		key := s.FillRef + "|" + s.FillVar
		if s.Fill == nil {
			return key, scene.Color{}
		}
		return key + "|rgba", *s.Fill
	}
	aKey, aCol := fill(a)
	bKey, bCol := fill(b)
	return aKey == bKey && aCol == bCol
}
