package encode_test

import (
	"context"
	"math"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Offset position channels (E1-S4) — the geometry.
//
// Every assertion here is on the COMPILED SCENE, never on the spec: a
// spec can carry a channel nothing reads, and that is exactly the
// failure this channel was added to avoid.

// offsetRects drives a flat spec through Build → Execute → Encode and
// returns the rect geometry of every mark in the first layer, in
// emission order.
func offsetRects(t *testing.T, body string) []scene.RectGeom {
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
	if len(doc.Grid.Cells) == 0 || len(doc.Grid.Cells[0].Scene.Layers) == 0 {
		t.Fatal("no scene layer produced")
	}
	layer := doc.Grid.Cells[0].Scene.Layers[0]
	out := make([]scene.RectGeom, 0, len(layer.Marks))
	for _, m := range layer.Marks {
		if m.Rect == nil {
			t.Fatalf("mark %s carries no rect", m.ID)
		}
		out = append(out, *m.Rect)
	}
	return out
}

// verticalOffsetSpec is a band on x with two series per category.
func verticalOffsetSpec(offset string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q2","s":"a","v":14},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"}` + offset + `}}`
}

// horizontalOffsetSpec mirrors it onto a band on y. This is the case
// that catches a mis-signed BandWidth(): a y band scale runs
// bottom-to-top, so its step, its width and any sub-band displacement
// derived from them are all negative.
func horizontalOffsetSpec(offset string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q2","s":"a","v":14},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "y":{"field":"q","type":"nominal"},
	   "x":{"field":"v","type":"quantitative"}` + offset + `}}`
}

const offsetEps = 1e-6

func nearly(a, b float64) bool { return math.Abs(a-b) < offsetEps }

// TestPrismOffsetVerticalDodges is the primitive: two rows sharing one
// x category must produce two non-overlapping rects of equal width,
// both inside the slot the undodged bar occupied, together spanning it
// exactly.
func TestPrismOffsetVerticalDodges(t *testing.T) {
	plain := offsetRects(t, verticalOffsetSpec(""))
	dodged := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal"}`))
	if len(plain) != 4 || len(dodged) != 4 {
		t.Fatalf("want 4 rects each, got plain=%d dodged=%d", len(plain), len(dodged))
	}
	// Q1's slot, as the undodged chart drew it.
	slotX, slotW := plain[0].X, plain[0].W
	a, b := dodged[0], dodged[1]
	if !nearly(a.W, b.W) {
		t.Errorf("sub-bands differ in width: %v vs %v", a.W, b.W)
	}
	if !nearly(a.W, slotW/2) {
		t.Errorf("sub-band width %v, want half the slot (%v)", a.W, slotW/2)
	}
	if !nearly(a.X, slotX) {
		t.Errorf("first sub-band starts at %v, want the slot's leading edge %v", a.X, slotX)
	}
	if !nearly(b.X, slotX+slotW/2) {
		t.Errorf("second sub-band starts at %v, want %v", b.X, slotX+slotW/2)
	}
	if !nearly(a.X+a.W+b.W, slotX+slotW) {
		t.Errorf("sub-bands end at %v, want the slot's trailing edge %v", a.X+a.W+b.W, slotX+slotW)
	}
	// The measure axis is untouched by dodging.
	for i := range plain {
		if !nearly(plain[i].Y, dodged[i].Y) || !nearly(plain[i].H, dodged[i].H) {
			t.Errorf("row %d moved along the measure axis: %v → %v", i, plain[i], dodged[i])
		}
	}
}

// TestPrismOffsetHorizontalDodges is the same assertion mirrored onto a
// band on y. A naive subdivision of the negative step lands the
// sub-bands outside the slot, mirrored, or with a negative height —
// none of which the vertical case can show.
func TestPrismOffsetHorizontalDodges(t *testing.T) {
	plain := offsetRects(t, horizontalOffsetSpec(""))
	dodged := offsetRects(t, horizontalOffsetSpec(`,"y_offset":{"field":"s","type":"nominal"}`))
	if len(plain) != 4 || len(dodged) != 4 {
		t.Fatalf("want 4 rects each, got plain=%d dodged=%d", len(plain), len(dodged))
	}
	slotY, slotH := plain[0].Y, plain[0].H
	if slotH <= 0 {
		t.Fatalf("undodged slot height %v is not drawable", slotH)
	}
	a, b := dodged[0], dodged[1]
	for _, r := range dodged {
		if r.H <= 0 {
			t.Fatalf("sub-band height %v is not drawable (signed step mishandled)", r.H)
		}
	}
	if !nearly(a.H, b.H) {
		t.Errorf("sub-bands differ in height: %v vs %v", a.H, b.H)
	}
	if !nearly(a.H, slotH/2) {
		t.Errorf("sub-band height %v, want half the slot (%v)", a.H, slotH/2)
	}
	// A y band runs bottom-to-top, so the first offset category takes
	// the sub-band at the slot's LOWER pixel edge — the same direction
	// the parent band assigns its own categories in.
	if !nearly(a.Y, slotY+slotH/2) {
		t.Errorf("first sub-band starts at %v, want %v", a.Y, slotY+slotH/2)
	}
	if !nearly(b.Y, slotY) {
		t.Errorf("second sub-band starts at %v, want %v", b.Y, slotY)
	}
	if a.Y < slotY-offsetEps || a.Y+a.H > slotY+slotH+offsetEps {
		t.Errorf("sub-band %v escapes the slot [%v, %v]", a, slotY, slotY+slotH)
	}
	for i := range plain {
		if !nearly(plain[i].X, dodged[i].X) || !nearly(plain[i].W, dodged[i].W) {
			t.Errorf("row %d moved along the measure axis: %v → %v", i, plain[i], dodged[i])
		}
	}
}

// TestPrismOffsetDomainIsGlobal is the property that makes bar widths
// comparable between groups: a category with one member and a category
// with two produce bars of the SAME width, and the lone member takes
// its own sub-band instead of widening to fill the slot.
func TestPrismOffsetDomainIsGlobal(t *testing.T) {
	const rows = `"data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q2","s":"a","v":14}]},`
	const base = `{"$schema":"urn:prism:schema:v1:spec",` + rows + `
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"}`
	plain := offsetRects(t, base+`}}`)
	dodged := offsetRects(t, base+`,"x_offset":{"field":"s","type":"nominal"}}}`)
	if len(plain) != 3 || len(dodged) != 3 {
		t.Fatalf("want 3 rects each, got %d / %d", len(plain), len(dodged))
	}
	// Every bar is half a slot wide, in the two-member group and in
	// the one-member group alike. A per-category domain would give the
	// lone member the full slot and make the widths incomparable.
	for i, r := range dodged {
		if !nearly(r.W, plain[0].W/2) {
			t.Errorf("rect %d is %v wide, want half a slot (%v)", i, r.W, plain[0].W/2)
		}
	}
	// Series "a" sorts first, so it takes sub-band 0 of its own slot in
	// BOTH groups — Q2's bar sits at the leading edge of Q2's slot, and
	// the sub-band "b" would have taken is simply left vacant.
	if !nearly(dodged[0].X, plain[0].X) {
		t.Errorf("Q1's \"a\" is at %v, want its slot's leading edge %v", dodged[0].X, plain[0].X)
	}
	if !nearly(dodged[2].X, plain[2].X) {
		t.Errorf("Q2's lone member is at %v, want its slot's leading edge %v", dodged[2].X, plain[2].X)
	}
	if !nearly(dodged[1].X, plain[1].X+plain[1].W/2) {
		t.Errorf("Q1's \"b\" is at %v, want the second sub-band %v", dodged[1].X, plain[1].X+plain[1].W/2)
	}
}

// TestPrismOffsetSingleValueIsIdentity pins the byte-identical clause
// at its sharpest: one distinct offset value must reproduce the
// undodged geometry exactly, because a single sub-band with zero
// padding IS the slot.
func TestPrismOffsetSingleValueIsIdentity(t *testing.T) {
	plain := offsetRects(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q2","s":"a","v":14}]},
	 "mark":{"type":"bar"},
	 "encoding":{"x":{"field":"q","type":"nominal"},"y":{"field":"v","type":"quantitative"}}}`)
	dodged := offsetRects(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q2","s":"a","v":14}]},
	 "mark":{"type":"bar"},
	 "encoding":{"x":{"field":"q","type":"nominal"},"y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`)
	if len(plain) != len(dodged) {
		t.Fatalf("mark count changed: %d → %d", len(plain), len(dodged))
	}
	for i := range plain {
		if plain[i] != dodged[i] {
			t.Errorf("row %d moved: %+v → %+v", i, plain[i], dodged[i])
		}
	}
}

// TestPrismOffsetOrderIsNotTableOrder pins the determinism clause: the
// sub-band a series takes follows the offset scale's domain order, so
// reordering the rows must not reorder the bars.
func TestPrismOffsetOrderIsNotTableOrder(t *testing.T) {
	forward := offsetRects(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	 "mark":{"type":"bar"},
	 "encoding":{"x":{"field":"q","type":"nominal"},"y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`)
	reversed := offsetRects(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"b","v":6},{"q":"Q1","s":"a","v":10}]},
	 "mark":{"type":"bar"},
	 "encoding":{"x":{"field":"q","type":"nominal"},"y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`)
	if len(forward) != 2 || len(reversed) != 2 {
		t.Fatalf("want 2 rects each, got %d / %d", len(forward), len(reversed))
	}
	// "a" is emitted first in one spec and second in the other, but it
	// must land in the same sub-band both times.
	if !nearly(forward[0].X, reversed[1].X) {
		t.Errorf(`series "a" moved with table order: %v vs %v`, forward[0].X, reversed[1].X)
	}
	if !nearly(forward[1].X, reversed[0].X) {
		t.Errorf(`series "b" moved with table order: %v vs %v`, forward[1].X, reversed[0].X)
	}
}

// TestPrismOffsetSortDescending pins that the direction vocabulary is
// acted on rather than accepted and dropped.
func TestPrismOffsetSortDescending(t *testing.T) {
	asc := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal"}`))
	desc := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal","sort":"descending"}`))
	if len(asc) != 4 || len(desc) != 4 {
		t.Fatalf("want 4 rects each, got %d / %d", len(asc), len(desc))
	}
	if nearly(asc[0].X, desc[0].X) {
		t.Fatal(`sort:"descending" left the sub-band order unchanged`)
	}
	if !nearly(asc[0].X, desc[1].X) || !nearly(asc[1].X, desc[0].X) {
		t.Errorf("descending did not swap the two sub-bands: %v/%v vs %v/%v",
			asc[0].X, asc[1].X, desc[0].X, desc[1].X)
	}
}

// TestPrismOffsetPaddingIsIndependent pins that within-group spacing is
// configured on the offset channel's own scale block and leaves the
// spacing between groups alone.
func TestPrismOffsetPaddingIsIndependent(t *testing.T) {
	tight := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal"}`))
	gapped := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal","scale":{"padding_inner":0.5}}`))
	if len(tight) != 4 || len(gapped) != 4 {
		t.Fatalf("want 4 rects each, got %d / %d", len(tight), len(gapped))
	}
	if gapped[0].W >= tight[0].W {
		t.Errorf("padding_inner did not narrow the sub-band: %v vs %v", gapped[0].W, tight[0].W)
	}
	// The parent band is untouched: Q2's slot still begins where it did.
	tightStep := tight[2].X - tight[0].X
	gappedStep := gapped[2].X - gapped[0].X
	if !nearly(tightStep, gappedStep) {
		t.Errorf("offset padding moved the parent band step: %v → %v", tightStep, gappedStep)
	}
}

// TestPrismOffsetExplicitDomainPinsOrder pins the highest-precedence
// order source.
func TestPrismOffsetExplicitDomainPinsOrder(t *testing.T) {
	pinned := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal","scale":{"domain":["b","a"]}}`))
	asc := offsetRects(t, verticalOffsetSpec(`,"x_offset":{"field":"s","type":"nominal"}`))
	if len(pinned) != 4 || len(asc) != 4 {
		t.Fatalf("want 4 rects each, got %d / %d", len(pinned), len(asc))
	}
	if !nearly(pinned[0].X, asc[1].X) || !nearly(pinned[1].X, asc[0].X) {
		t.Errorf("explicit domain did not pin the order: %v/%v vs %v/%v",
			pinned[0].X, pinned[1].X, asc[0].X, asc[1].X)
	}
}

// TestPrismOffsetInsideALayer covers the SECOND marks.Inputs
// construction site. encode/encode.go (flat) and
// encode/encode_composite.go (per-layer) both have to fill the offset
// binding; filling only the first leaves the channel dead inside
// layer / facet / repeat, which is a whole class of silent bug and one
// this repo has shipped before.
func TestPrismOffsetInsideALayer(t *testing.T) {
	body := []byte(`{
	  "$schema": "urn:prism:schema:v1:spec",
	  "layer": [{
	    "$schema": "urn:prism:schema:v1:spec",
	    "data": {"values": [
	      {"q": "Q1", "s": "a", "v": 10}, {"q": "Q1", "s": "b", "v": 6},
	      {"q": "Q2", "s": "a", "v": 14}, {"q": "Q2", "s": "b", "v": 9}]},
	    "mark": "bar",
	    "encoding": {
	      "x": {"field": "q", "type": "nominal"},
	      "y": {"field": "v", "type": "quantitative"},
	      "x_offset": {"field": "s", "type": "nominal"}
	    }
	  }]
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, execErr := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if execErr != nil {
			t.Fatalf("Execute child %d: %v", i, execErr)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	marksOut := doc.Grid.Cells[0].Scene.Layers[0].Marks
	if len(marksOut) != 4 {
		t.Fatalf("want 4 marks, got %d", len(marksOut))
	}
	a, b := marksOut[0].Rect, marksOut[1].Rect
	if a == nil || b == nil {
		t.Fatal("layered bars carry no rect geometry")
	}
	if !nearly(a.W, b.W) {
		t.Errorf("sub-bands differ in width inside a layer: %v vs %v", a.W, b.W)
	}
	if !nearly(b.X, a.X+a.W) {
		t.Errorf("layered bars did not dodge: %v then %v (+%v)", a.X, b.X, a.W)
	}
}
