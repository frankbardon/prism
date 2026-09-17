package encode_test

import (
	"encoding/json"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// Offset scales across composition children (E3-S1).
//
// Shared by default, `independent` on request — the rule x and y
// already follow. Every assertion is on the COMPILED SCENE, because a
// resolve block that decodes and then changes no geometry is exactly
// the silent no-op this channel was added to avoid.

// layerRects returns the rect geometry of one layer's marks, in
// emission order.
func layerRects(t *testing.T, doc *scene.SceneDoc, layer int) []scene.RectGeom {
	t.Helper()
	if len(doc.Grid.Cells) == 0 {
		t.Fatal("no scene cell produced")
	}
	layers := doc.Grid.Cells[0].Scene.Layers
	if layer >= len(layers) {
		t.Fatalf("layer %d absent: only %d produced", layer, len(layers))
	}
	out := make([]scene.RectGeom, 0, len(layers[layer].Marks))
	for _, m := range layers[layer].Marks {
		if m.Rect == nil {
			t.Fatalf("mark %s carries no rect", m.ID)
		}
		out = append(out, *m.Rect)
	}
	return out
}

// cellRects returns the rect geometry of one grid cell's first layer.
func cellRects(t *testing.T, doc *scene.SceneDoc, cell int) []scene.RectGeom {
	t.Helper()
	if cell >= len(doc.Grid.Cells) {
		t.Fatalf("cell %d absent: only %d produced", cell, len(doc.Grid.Cells))
	}
	layers := doc.Grid.Cells[cell].Scene.Layers
	if len(layers) == 0 {
		t.Fatalf("cell %d has no layer", cell)
	}
	out := make([]scene.RectGeom, 0, len(layers[0].Marks))
	for _, m := range layers[0].Marks {
		if m.Rect == nil {
			t.Fatalf("mark %s carries no rect", m.ID)
		}
		out = append(out, *m.Rect)
	}
	return out
}

// offsetConfigConflicts returns every PRISM_WARN_OFFSET_CONFIG_CONFLICT
// on a document.
func offsetConfigConflicts(warnings []scene.Warning) []scene.Warning {
	var out []scene.Warning
	for _, w := range warnings {
		if w.Code == scene.WarnOffsetConfigConflict {
			out = append(out, w)
		}
	}
	return out
}

// unevenLayerSpec layers two bar charts over one category. The first
// layer carries both series; the second carries only "a". Under a
// shared offset scale the second layer must still divide the slot in
// two, so its single bar lines up with the first layer's "a" bar.
func unevenLayerSpec(resolveBlock string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",` + resolveBlock + `
	 "layer":[
	  {"data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}},
	  {"data":{"values":[{"q":"Q1","s":"a","v":4}]},
	   "mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}]}`
}

// The default. Two layers binding the same offset field divide the
// band slot the same way, so a series drawn in both lands in the same
// place — even when one layer never saw the other series.
func TestPrismOffsetSharedAcrossLayersByDefault(t *testing.T) {
	doc := encodeCompositeJSON(t, unevenLayerSpec(""))

	first := layerRects(t, doc, 0)
	second := layerRects(t, doc, 1)
	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("mark counts = %d / %d, want 2 / 1", len(first), len(second))
	}
	if !nearly(second[0].W, first[0].W) {
		t.Errorf("shared offset: layer-1 sub-band width %v, want layer-0's %v", second[0].W, first[0].W)
	}
	if !nearly(second[0].X, first[0].X) {
		t.Errorf("shared offset: series \"a\" at x=%v in layer-1 but x=%v in layer-0", second[0].X, first[0].X)
	}
}

// The opt-out. Each layer divides its own slot, so the layer that saw
// one series gives it the whole slot.
func TestPrismOffsetIndependentAcrossLayers(t *testing.T) {
	doc := encodeCompositeJSON(t, unevenLayerSpec(
		`"resolve":{"scale":{"x_offset":"independent"}},`))

	first := layerRects(t, doc, 0)
	second := layerRects(t, doc, 1)
	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("mark counts = %d / %d, want 2 / 1", len(first), len(second))
	}
	if nearly(second[0].W, first[0].W) {
		t.Errorf("independent offset: layer-1 sub-band width %v equals layer-0's; it should span the whole slot", second[0].W)
	}
	if !nearly(second[0].W, first[0].W*2) {
		t.Errorf("independent offset: layer-1 width %v, want the full slot %v", second[0].W, first[0].W*2)
	}
}

// A facet resolves the sub-band order once for the whole grid. A
// series present in only some cells would otherwise make a bar a
// different width from one cell to the next, which is precisely the
// comparison a facet exists to support.
func TestPrismOffsetSharedAcrossFacetCells(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"g":"L","q":"Q1","s":"a","v":10},{"g":"L","q":"Q1","s":"b","v":6},
	   {"g":"R","q":"Q1","s":"a","v":3}]},
	 "facet":{"column":{"field":"g","type":"nominal"}},
	 "spec":{"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}}`)

	left := cellRects(t, doc, 0)
	right := cellRects(t, doc, 1)
	if len(left) != 2 || len(right) != 1 {
		t.Fatalf("mark counts = %d / %d, want 2 / 1", len(left), len(right))
	}
	if !nearly(right[0].W, left[0].W) {
		t.Errorf("shared offset: cell R sub-band width %v, want cell L's %v", right[0].W, left[0].W)
	}
}

// The same facet with the channel opted out: the one-series cell
// takes the whole slot again.
func TestPrismOffsetIndependentAcrossFacetCells(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "resolve":{"scale":{"x_offset":"independent"}},
	 "data":{"values":[
	   {"g":"L","q":"Q1","s":"a","v":10},{"g":"L","q":"Q1","s":"b","v":6},
	   {"g":"R","q":"Q1","s":"a","v":3}]},
	 "facet":{"column":{"field":"g","type":"nominal"}},
	 "spec":{"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}}`)

	left := cellRects(t, doc, 0)
	right := cellRects(t, doc, 1)
	if len(left) != 2 || len(right) != 1 {
		t.Fatalf("mark counts = %d / %d, want 2 / 1", len(left), len(right))
	}
	if !nearly(right[0].W, left[0].W*2) {
		t.Errorf("independent offset: cell R width %v, want the full slot %v", right[0].W, left[0].W*2)
	}
}

// Two children disagreeing about a shared offset scale is reported,
// never settled in silence. First specified wins, as it does for a
// shared axis property.
func TestPrismOffsetConflictingDomainWarns(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	 "layer":[
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","scale":{"domain":["a","b"]}}}},
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","scale":{"domain":["b","a"]}}}}]}`)

	warns := offsetConfigConflicts(doc.Warnings)
	if len(warns) != 1 {
		t.Fatalf("offset-config conflicts = %d, want exactly 1: %+v", len(warns), doc.Warnings)
	}
	if got := warns[0].Details["Property"]; got != "domain" {
		t.Errorf("conflict property = %v, want \"domain\"", got)
	}
	if got := warns[0].Details["Winner"]; got != "layer-0" {
		t.Errorf("conflict winner = %v, want \"layer-0\"", got)
	}
	// First specified wins: both layers draw "a" in the leading
	// sub-band, so the two layers' first marks coincide.
	first := layerRects(t, doc, 0)
	second := layerRects(t, doc, 1)
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("mark counts = %d / %d, want 2 / 2", len(first), len(second))
	}
	if !nearly(first[0].X, second[0].X) {
		t.Errorf("losing domain was honoured: layer-0 x=%v, layer-1 x=%v", first[0].X, second[0].X)
	}
}

// Children that agree are not reported. A conflict warning on a
// coherent spec would train authors to skip the whole warning block.
func TestPrismOffsetAgreeingDomainIsSilent(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6}]},
	 "layer":[
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","scale":{"domain":["b","a"]}}}},
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal","scale":{"domain":["b","a"]}}}}]}`)

	if warns := offsetConfigConflicts(doc.Warnings); len(warns) != 0 {
		t.Fatalf("offset-config conflicts = %d on an agreeing spec: %+v", len(warns), warns)
	}
}

// A composition binding no offset must be untouched by the fold: the
// resolve entry is inert and the scene is byte-identical with or
// without it. This is the guarantee the committed layer / facet /
// repeat goldens encode; asserting it here says why they did not move.
func TestPrismOffsetUnboundCompositionIsByteIdentical(t *testing.T) {
	const layers = `
	 "layer":[
	  {"mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"}}},
	  {"mark":{"type":"line"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"}}}]}`
	const data = `"data":{"values":[{"q":"Q1","v":10},{"q":"Q2","v":6}]},`

	plain := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",`+data+layers)
	withResolve := encodeCompositeJSON(t, `{"$schema":"urn:prism:schema:v1:spec",`+
		`"resolve":{"scale":{"x_offset":"independent"}},`+data+layers)

	a, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain: %v", err)
	}
	b, err := json.Marshal(withResolve)
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
