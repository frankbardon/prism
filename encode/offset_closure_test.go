package encode_test

import (
	"encoding/json"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// The encode half of epic E2's QA gate (E2-S4).
//
// Two bindings were live silent no-ops between E1-S4 and E2-S2, and
// both were closed by a VALIDATE rule rather than by an encode-side
// gate — on purpose, so the question has exactly one decision point.
// That choice only holds if the encoder really does stay total, and
// if the scene it produces is the one the rule's prose describes.
//
// These tests pin the encode-side behaviour the two rules are written
// against. If either moves, the rule text in errors/codes.go and in
// validate/rules/ has gone stale and must move with it.
//
// Every assertion is on the COMPILED SCENE, never on the spec.

// sceneJSON renders the first layer of a flat spec's scene to JSON so
// two encodes can be compared whole — geometry, style and mark kind
// together — rather than one field at a time.
func sceneJSON(t *testing.T, body string) string {
	t.Helper()
	doc := encodeFlatJSON(t, body)
	if len(doc.Grid.Cells) == 0 || len(doc.Grid.Cells[0].Scene.Layers) == 0 {
		t.Fatal("no scene layer produced")
	}
	b, err := json.Marshal(doc.Grid.Cells[0].Scene.Layers[0])
	if err != nil {
		t.Fatalf("marshal layer: %v", err)
	}
	return string(b)
}

// tickSpec is a band on x carrying two series per category. A `tick`
// seats itself in the band slot through CategorySlots, so it reaches
// the same rectAxisExtent normaliser a bar does.
func tickSpec(offset string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q2","s":"a","v":14},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"tick"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"}` + offset + `}}`
}

const xOffsetKey = `,"x_offset":{"field":"s","type":"nominal"}`

// TestPrismOffsetOnUnsupportedMarkStillEncodes pins why
// PRISM_SPEC_063 is load-bearing rather than cosmetic.
//
// The encoder is total on the mark-type question: it fills
// marks.Inputs.Offset for every mark and rectAxisExtent applies the
// nested band to whatever routes through it. A `tick` therefore
// encodes without error AND moves — so validate is the only thing
// standing between an author and a chart whose channel no mark
// documents. Nothing else would ever tell them.
func TestPrismOffsetOnUnsupportedMarkStillEncodes(t *testing.T) {
	plain := sceneJSON(t, tickSpec(""))
	dodged := sceneJSON(t, tickSpec(xOffsetKey))

	if plain == dodged {
		t.Fatalf("a tick with x_offset encoded identically to one without; " +
			"PRISM_SPEC_063's premise (the encoder stays total and applies the binding) has changed")
	}
}

// TestPrismOffsetWithSameAxisSpanIsDroppedAtEncode pins the silent
// no-op PRISM_SPEC_066 names.
//
// rectAxisExtent takes its span branch first and never consults the
// sub-band scale, so an x2 beside an x_offset leaves the offset
// entirely unread: the scene is byte-identical to the same spec with
// no offset channel at all. That is the definition of a silent
// no-op, and it is why the binding is rejected at validate instead of
// being accepted and ignored.
func TestPrismOffsetWithSameAxisSpanIsDroppedAtEncode(t *testing.T) {
	// A Gantt shape: the x extent comes from the pair of measure
	// columns, the y axis carries the category band. x_offset names
	// the SPANNED axis, so it has nothing left to subdivide.
	const head = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"task":"design","s":"a","lo":1,"hi":4},
	   {"task":"design","s":"b","lo":2,"hi":6},
	   {"task":"build","s":"a","lo":5,"hi":9},
	   {"task":"build","s":"b","lo":3,"hi":8}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "y":{"field":"task","type":"nominal"},
	   "x":{"field":"lo","type":"quantitative"},
	   "x2":{"field":"hi","type":"quantitative"}`

	plain := sceneJSON(t, head+`}}`)
	dodged := sceneJSON(t, head+xOffsetKey+`}}`)

	if plain != dodged {
		t.Fatalf("x2 + x_offset changed the scene; PRISM_SPEC_066's premise "+
			"(the span branch wins and the offset is never read) has changed\nplain:  %s\ndodged: %s", plain, dodged)
	}
}

// TestPrismOffsetRangedDodgedBarIsRealGeometry is the companion
// negative: the OTHER axis is untouched, so a y2 beside an x_offset
// really does dodge. PRISM_SPEC_066 must never widen to this case.
func TestPrismOffsetRangedDodgedBarIsRealGeometry(t *testing.T) {
	const head = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","lo":2,"hi":10},{"q":"Q1","s":"b","lo":1,"hi":6},
	   {"q":"Q2","s":"a","lo":3,"hi":14},{"q":"Q2","s":"b","lo":4,"hi":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"lo","type":"quantitative"},
	   "y2":{"field":"hi","type":"quantitative"}`

	plain := offsetRects(t, head+`}}`)
	dodged := offsetRects(t, head+xOffsetKey+`}}`)
	if len(plain) != 4 || len(dodged) != 4 {
		t.Fatalf("expected 4 rects each, got %d and %d", len(plain), len(dodged))
	}
	// Undodged: both rows of a category share the whole slot.
	if plain[0].X != plain[1].X || plain[0].W != plain[1].W {
		t.Fatalf("without an offset, Q1's two rows must share the slot: %+v vs %+v", plain[0], plain[1])
	}
	// Dodged: they take adjacent halves of it, and the y extent — the
	// ranged half — is untouched by the dodge.
	if dodged[0].X == dodged[1].X {
		t.Fatalf("with x_offset, Q1's two rows must take different sub-bands: %+v vs %+v", dodged[0], dodged[1])
	}
	if dodged[0].Y != plain[0].Y || dodged[0].H != plain[0].H {
		t.Fatalf("the y2 extent must survive the dodge: %+v vs %+v", dodged[0], plain[0])
	}
	if got, want := dodged[0].W+dodged[1].W, plain[0].W; !nearly(got, want) {
		t.Fatalf("the two sub-bands must fill the slot: %v + %v != %v", dodged[0].W, dodged[1].W, want)
	}
}

// TestPrismOffsetCollisionAbsentFromCleanScene is the warning half of
// the gate: the code appears on SceneDoc.Warnings only when a
// sub-band really is claimed twice.
func TestPrismOffsetCollisionAbsentFromCleanScene(t *testing.T) {
	doc := encodeFlatJSON(t, tickSpec(""))
	if got := offsetCollisions(doc.Warnings); len(got) != 0 {
		t.Fatalf("a spec with no offset channel must carry no collision warning, got %+v", got)
	}
	if code := scene.WarnOffsetCollision; code != "PRISM_WARN_OFFSET_COLLISION" {
		t.Fatalf("scene.WarnOffsetCollision = %q, want the registered code", code)
	}
}
