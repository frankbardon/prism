package encode_test

import (
	"context"
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

// encodeInline decodes an inline spec, runs it through build +
// execute, and encodes it — the whole pipeline without a fixture file.
func encodeInline(t *testing.T, body string) *scene.Scene {
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
		t.Fatalf("build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute: %d node errors: %v", len(res.Errors), res.Errors)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(doc.Grid.Cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(doc.Grid.Cells))
	}
	return &doc.Grid.Cells[0].Scene
}

const detailOnlyLineSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"t": 1, "v": 10, "sensor": "a"},
    {"t": 2, "v": 20, "sensor": "a"},
    {"t": 1, "v": 100, "sensor": "b"},
    {"t": 2, "v": 200, "sensor": "b"}
  ]},
  "mark": "line",
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"},
    "detail": {"field": "sensor", "type": "nominal"}
  }
}`

// TestPrismEncodeDetailOnlySplitsSeries: encoding.detail alone splits
// a line into one path per distinct value and produces NO legend —
// detail is a grouping channel, not a visual one.
func TestPrismEncodeDetailOnlySplitsSeries(t *testing.T) {
	sc := encodeInline(t, detailOnlyLineSpec)
	if len(sc.Layers) != 1 {
		t.Fatalf("layers = %d, want 1", len(sc.Layers))
	}
	if got := len(sc.Layers[0].Marks); got != 2 {
		t.Errorf("marks = %d, want 2 (one polyline per sensor)", got)
	}
	if len(sc.Legends) != 0 {
		t.Errorf("legends = %d, want 0 — detail must not build a legend", len(sc.Legends))
	}
	// Every path keeps the same stroke: detail consumes no palette.
	var first string
	for i, m := range sc.Layers[0].Marks {
		got := ""
		if m.Style.Stroke != nil {
			got = m.Style.Stroke.Hex()
		}
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Errorf("marks[%d] stroke = %q, want %q — detail must not restyle", i, got, first)
		}
	}
}

const detailArrayAreaSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"t": 1, "v": 10, "site": "a", "unit": "u1"},
    {"t": 2, "v": 20, "site": "a", "unit": "u1"},
    {"t": 1, "v": 11, "site": "a", "unit": "u2"},
    {"t": 2, "v": 21, "site": "a", "unit": "u2"},
    {"t": 1, "v": 30, "site": "b", "unit": "u1"},
    {"t": 2, "v": 40, "site": "b", "unit": "u1"}
  ]},
  "mark": "area",
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"},
    "detail": [
      {"field": "site", "type": "nominal"},
      {"field": "unit", "type": "nominal"}
    ]
  }
}`

// TestPrismEncodeDetailArray: encoding.detail accepts an array; the
// grouping tuple spans every listed field.
func TestPrismEncodeDetailArray(t *testing.T) {
	sc := encodeInline(t, detailArrayAreaSpec)
	if got := len(sc.Layers[0].Marks); got != 3 {
		t.Errorf("marks = %d, want 3 (a/u1, a/u2, b/u1)", got)
	}
}

const colorDetailLineSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"t": 1, "v": 10, "region": "east", "store": "s1"},
    {"t": 2, "v": 20, "region": "east", "store": "s1"},
    {"t": 1, "v": 11, "region": "east", "store": "s2"},
    {"t": 2, "v": 21, "region": "east", "store": "s2"},
    {"t": 1, "v": 30, "region": "west", "store": "s1"},
    {"t": 2, "v": 40, "region": "west", "store": "s1"}
  ]},
  "mark": "line",
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"},
    "color": {"field": "region", "type": "nominal"},
    "detail": {"field": "store", "type": "nominal"}
  }
}`

// TestPrismEncodeColorAndDetailCompose: one path per (color, detail)
// pair, the legend still carries one entry per color, and the color
// component of the emission order matches the legend order.
func TestPrismEncodeColorAndDetailCompose(t *testing.T) {
	sc := encodeInline(t, colorDetailLineSpec)
	marks := sc.Layers[0].Marks
	if len(marks) != 3 {
		t.Fatalf("marks = %d, want 3 (east/s1, east/s2, west/s1)", len(marks))
	}
	if len(sc.Legends) != 1 {
		t.Fatalf("legends = %d, want 1 (color only)", len(sc.Legends))
	}
	if got := len(sc.Legends[0].Entries); got != 2 {
		t.Errorf("legend entries = %d, want 2 — detail must add none", got)
	}
	hex := func(i int) string {
		if marks[i].Style.Stroke == nil {
			return ""
		}
		return marks[i].Style.Stroke.Hex()
	}
	if hex(0) != hex(1) {
		t.Errorf("marks[0]/[1] strokes = %q/%q, want equal (both east)", hex(0), hex(1))
	}
	if hex(2) == hex(0) {
		t.Errorf("marks[2] stroke = %q, want a different color from east", hex(2))
	}
	if want := sc.Legends[0].Entries[0].Swatch.Color; want != nil && hex(0) != want.Hex() {
		t.Errorf("marks[0] stroke = %q, want legend entry 0's %q (color order must match the legend)", hex(0), want.Hex())
	}
}

const detailWithAggregateSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"t": 1, "v": 10, "sensor": "a"},
    {"t": 1, "v": 12, "sensor": "a"},
    {"t": 2, "v": 20, "sensor": "a"},
    {"t": 1, "v": 100, "sensor": "b"},
    {"t": 2, "v": 200, "sensor": "b"}
  ]},
  "mark": "line",
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"aggregate": "mean", "field": "v", "type": "quantitative"},
    "detail": {"field": "sensor", "type": "nominal"}
  }
}`

// TestPrismEncodeDetailSurvivesEncodingAggregate: the synthetic
// GroupAggregateNode injected for an aggregated channel must carry
// the detail field into its groupby, or the column vanishes before
// the mark encoder can partition on it.
func TestPrismEncodeDetailSurvivesEncodingAggregate(t *testing.T) {
	sc := encodeInline(t, detailWithAggregateSpec)
	if got := len(sc.Layers[0].Marks); got != 2 {
		t.Errorf("marks = %d, want 2 (one polyline per sensor)", got)
	}
}
