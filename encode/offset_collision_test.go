package encode_test

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// Duplicate sub-band keys (E2-S3).
//
// A sub-band is the pair (category value, offset value). Two rows
// carrying the same pair draw the same rect, so the marks overlap —
// the one thing binding an offset was meant to stop. The chart still
// renders; the overlap is named rather than hidden.
//
// Every assertion is on the COMPILED SCENE, never on the spec.

// offsetCollisions returns every PRISM_WARN_OFFSET_COLLISION on doc.
func offsetCollisions(warnings []scene.Warning) []scene.Warning {
	var out []scene.Warning
	for _, w := range warnings {
		if w.Code == scene.WarnOffsetCollision {
			out = append(out, w)
		}
	}
	return out
}

// collidingSpec repeats (Q1, a) twice: four rows, three distinct
// sub-bands, one collision.
const collidingSpec = `{"$schema":"urn:prism:schema:v1:spec",
 "data":{"values":[
   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
   {"q":"Q1","s":"a","v":4},{"q":"Q2","s":"b","v":9}]},
 "mark":{"type":"bar"},
 "encoding":{
   "x":{"field":"q","type":"nominal"},
   "y":{"field":"v","type":"quantitative"},
   "x_offset":{"field":"s","type":"nominal"}}}`

// cleanSpec is the same shape with every pair distinct.
const cleanSpec = `{"$schema":"urn:prism:schema:v1:spec",
 "data":{"values":[
   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
   {"q":"Q2","s":"a","v":14},{"q":"Q2","s":"b","v":9}]},
 "mark":{"type":"bar"},
 "encoding":{
   "x":{"field":"q","type":"nominal"},
   "y":{"field":"v","type":"quantitative"},
   "x_offset":{"field":"s","type":"nominal"}}}`

func TestPrismOffsetCollisionWarnsOnce(t *testing.T) {
	doc := encodeFlatJSON(t, collidingSpec)

	warns := offsetCollisions(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("offset-collision warnings = %d, want exactly 1: %+v", len(warns), doc.Warnings)
	}
	w := warns[0]
	if got, ok := w.Details["count"].(int); !ok || got != 1 {
		t.Errorf("details count = %v, want 1", w.Details["count"])
	}
	// The key an author reads must name the fields they wrote, not a
	// Go struct dump.
	key, _ := w.Details["key"].(string)
	if key != "q=Q1, s=a" {
		t.Errorf("details key = %q, want %q", key, "q=Q1, s=a")
	}
	if !strings.Contains(w.Message, key) {
		t.Errorf("message %q does not name the key %q", w.Message, key)
	}
	if ch, _ := w.Details["channel"].(string); ch != "x_offset" {
		t.Errorf("details channel = %v, want x_offset", w.Details["channel"])
	}

	// Geometry is untouched: all four rows still draw a rect, and the
	// two colliding rows land on exactly the same one.
	rects := offsetRects(t, collidingSpec)
	if len(rects) != 4 {
		t.Fatalf("rects = %d, want 4 (both colliding rows still render)", len(rects))
	}
	if rects[0].X != rects[2].X || rects[0].W != rects[2].W {
		t.Errorf("colliding rows do not share a sub-band: %+v vs %+v", rects[0], rects[2])
	}
}

// A collision count is the number of rows BEYOND the first for each
// repeated pair, not the number of rows involved: three rows on one
// key are two repeats.
func TestPrismOffsetCollisionCountsRepeatsNotRows(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"a","v":6},
	   {"q":"Q1","s":"a","v":4},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`)

	warns := offsetCollisions(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("warnings = %d, want 1", len(warns))
	}
	if got, ok := warns[0].Details["count"].(int); !ok || got != 2 {
		t.Errorf("count = %v, want 2", warns[0].Details["count"])
	}
}

// The measure aggregating makes the pair unique by construction: the
// synthetic group-by keeps the category field and the offset field,
// so the duplicates collapse into one bar per sub-band.
func TestPrismOffsetCollisionSilentUnderAggregate(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q1","s":"a","v":4},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"aggregate":"sum","field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`)

	if w := offsetCollisions(doc.Warnings); len(w) != 0 {
		t.Fatalf("aggregated spec warned: %+v", w)
	}
}

func TestPrismOffsetCollisionSilentOnCleanChart(t *testing.T) {
	doc := encodeFlatJSON(t, cleanSpec)
	if w := offsetCollisions(doc.Warnings); len(w) != 0 {
		t.Fatalf("clean grouped bar warned: %+v", w)
	}
}

// With no offset bound, overlapping rows are the author's explicit
// choice — Prism has always drawn them that way and must not start
// second-guessing it now.
func TestPrismOffsetCollisionSilentWhenUnbound(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q1","s":"a","v":4},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"}}}`)

	if w := offsetCollisions(doc.Warnings); len(w) != 0 {
		t.Fatalf("unbound offset warned: %+v", w)
	}
}

// The horizontal orientation reads the category off y, and must
// report the same collision the vertical one does.
func TestPrismOffsetCollisionHorizontal(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q1","s":"a","v":4},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "y":{"field":"q","type":"nominal"},
	   "x":{"field":"v","type":"quantitative"},
	   "y_offset":{"field":"s","type":"nominal"}}}`)

	warns := offsetCollisions(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("warnings = %d, want 1: %+v", len(warns), doc.Warnings)
	}
	if ch, _ := warns[0].Details["channel"].(string); ch != "y_offset" {
		t.Errorf("channel = %v, want y_offset", warns[0].Details["channel"])
	}
	if key, _ := warns[0].Details["key"].(string); key != "q=Q1, s=a" {
		t.Errorf("key = %q, want %q", key, "q=Q1, s=a")
	}
}

// A bound offset channel is honoured, so it must not also be reported
// inert. A false positive on a working feature trains authors to skip
// the warning block — which is the thing that would have hidden the
// collision warning above.
func TestPrismOffsetCollisionNoInertFalsePositive(t *testing.T) {
	for _, body := range []string{cleanSpec, collidingSpec} {
		doc := encodeFlatJSON(t, body)
		for _, w := range doc.Warnings {
			if w.Code != scene.WarnChannelInert {
				continue
			}
			if ch, _ := w.Details["channel"].(string); strings.HasSuffix(ch, "_offset") {
				t.Errorf("bound offset channel reported inert: %+v", w)
			}
		}
	}
}

// The author wrote one chart. A composition encodes one mark set per
// layer and one scene per cell, each resolving its own offset binding
// off its own table — so the leaves report and the top of the tree
// folds the reports into a single entry carrying the total.
func TestPrismOffsetCollisionCollapsedAcrossLayers(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"a","v":6},
	   {"q":"Q2","s":"b","v":9}]},
	 "layer":[
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}},
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}]}`)

	warns := offsetCollisions(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("warnings = %d, want exactly 1 for the whole chart: %+v", len(warns), doc.Warnings)
	}
	// One repeat per layer, two layers, one entry naming both.
	if got, ok := warns[0].Details["count"].(int); !ok || got != 2 {
		t.Errorf("collapsed count = %v, want 2", warns[0].Details["count"])
	}
}

// Faceting is the case that would otherwise fire once per cell.
func TestPrismOffsetCollisionCollapsedAcrossFacetCells(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"g":"L","q":"Q1","s":"a","v":10},{"g":"L","q":"Q1","s":"a","v":6},
	   {"g":"R","q":"Q1","s":"a","v":3},{"g":"R","q":"Q1","s":"a","v":8}]},
	 "facet":{"column":{"field":"g","type":"nominal"}},
	 "spec":{"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}}`)

	warns := offsetCollisions(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("warnings = %d, want exactly 1 for the whole chart: %+v", len(warns), doc.Warnings)
	}
	if got, ok := warns[0].Details["count"].(int); !ok || got != 2 {
		t.Errorf("collapsed count = %v, want 2 (one repeat per cell)", warns[0].Details["count"])
	}
}
