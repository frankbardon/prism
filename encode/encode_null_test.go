package encode_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// encodeInlineDoc is encodeInline's sibling that keeps the whole
// SceneDoc, because the null policy is observable through
// doc.Warnings as much as through the geometry.
func encodeInlineDoc(t *testing.T, body string) (*scene.SceneDoc, error) {
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
	return encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{})
}

func nullWarningOf(t *testing.T, doc *scene.SceneDoc) *scene.Warning {
	t.Helper()
	for i := range doc.Warnings {
		if doc.Warnings[i].Code == scene.WarnNullDropped {
			return &doc.Warnings[i]
		}
	}
	return nil
}

const midSeriesNullSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"t": 1, "v": 100}, {"t": 2, "v": null}, {"t": 3, "v": 120}]},
  "mark": {"type": "line"},
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"}
  }
}`

// TestPrismEncodeDropsNullRowsWithWarning: a mid-series null used to
// reach LinearScale.Apply and hard-error with PRISM_ENCODE_001. It is
// now dropped, the surviving rows render, and PRISM_WARN_NULL_DROPPED
// reports the count + the offending channel.
func TestPrismEncodeDropsNullRowsWithWarning(t *testing.T) {
	doc, err := encodeInlineDoc(t, midSeriesNullSpec)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := &doc.Grid.Cells[0].Scene
	if len(sc.Layers) != 1 || len(sc.Layers[0].Marks) != 1 {
		t.Fatalf("layers/marks = %d/%v, want one line mark", len(sc.Layers), sc.Layers)
	}
	line := sc.Layers[0].Marks[0].Line
	if line == nil {
		t.Fatal("mark is not a line")
	}
	if len(line.Points) != 2 {
		t.Errorf("points = %d, want 2 (the null row dropped)", len(line.Points))
	}

	warn := nullWarningOf(t, doc)
	if warn == nil {
		t.Fatalf("no PRISM_WARN_NULL_DROPPED in %+v", doc.Warnings)
	}
	if warn.Details["count"] != 1 {
		t.Errorf("count = %v, want 1", warn.Details["count"])
	}
	chans, _ := warn.Details["channels"].([]string)
	if len(chans) != 1 || chans[0] != "y" {
		t.Errorf("channels = %v, want [y]", warn.Details["channels"])
	}
}

const groupedNullSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"t": 1, "v": 10, "s": "a"},
    {"t": 2, "v": null, "s": "a"},
    {"t": 3, "v": 30, "s": "a"},
    {"t": 1, "v": 15, "s": "b"},
    {"t": 2, "v": 25, "s": "b"},
    {"t": 3, "v": null, "s": "b"}
  ]},
  "mark": {"type": "line"},
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"},
    "color": {"field": "s", "type": "nominal"}
  }
}`

// TestPrismEncodeNullDropComposesWithGrouping: dropping rows must not
// corrupt the row partitioner in encode/marks/group.go — each series
// keeps exactly the rows it should, with no cross-group bleed.
func TestPrismEncodeNullDropComposesWithGrouping(t *testing.T) {
	doc, err := encodeInlineDoc(t, groupedNullSpec)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := &doc.Grid.Cells[0].Scene
	if len(sc.Layers[0].Marks) != 2 {
		t.Fatalf("marks = %d, want 2 (one per colour group)", len(sc.Layers[0].Marks))
	}
	for i, m := range sc.Layers[0].Marks {
		if m.Line == nil {
			t.Fatalf("mark %d is not a line", i)
		}
		if len(m.Line.Points) != 2 {
			t.Errorf("group %d points = %d, want 2", i, len(m.Line.Points))
		}
	}
	if warn := nullWarningOf(t, doc); warn == nil || warn.Details["count"] != 2 {
		t.Errorf("warning = %+v, want count 2", warn)
	}
}

const tooltipNullSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"t": 1, "v": 100, "note": "a"},
    {"t": 2, "v": 110, "note": null},
    {"t": 3, "v": 120, "note": "c"}
  ]},
  "mark": {"type": "point"},
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"},
    "tooltip": {"field": "note", "type": "nominal"}
  }
}`

// TestPrismEncodeKeepsRowsNullOnlyOffScale: a null in a channel that
// never reaches a scale (tooltip here) is cosmetic — the row stays and
// no warning fires.
func TestPrismEncodeKeepsRowsNullOnlyOffScale(t *testing.T) {
	doc, err := encodeInlineDoc(t, tooltipNullSpec)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := &doc.Grid.Cells[0].Scene
	if got := len(sc.Layers[0].Marks); got != 3 {
		t.Errorf("marks = %d, want 3 (no row dropped)", got)
	}
	if warn := nullWarningOf(t, doc); warn != nil {
		t.Errorf("unexpected warning %+v", warn)
	}
}

const allNullSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"t": 1, "v": null}, {"t": 2, "v": null}, {"t": 3, "v": null}]},
  "mark": {"type": "line"},
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"}
  }
}`

// TestPrismEncodeAllNullBoundFieldErrors: "some rows dropped" is a
// warning, "nothing left to draw" is an error — an empty chart would
// hide the mistake.
func TestPrismEncodeAllNullBoundFieldErrors(t *testing.T) {
	_, err := encodeInlineDoc(t, allNullSpec)
	if err == nil {
		t.Fatal("expected an error for an all-null bound field")
	}
	var app *prismerrors.AppError
	if !errors.As(err, &app) {
		t.Fatalf("error %v is not an *AppError", err)
	}
	if app.Code != "PRISM_ENCODE_NULL_ALL_ROWS" {
		t.Errorf("code = %q, want PRISM_ENCODE_NULL_ALL_ROWS", app.Code)
	}
}

const layeredNullSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "layer": [
    {
      "data": {"values": [{"t": 1, "v": 10}, {"t": 2, "v": null}, {"t": 3, "v": 30}]},
      "mark": {"type": "line"},
      "encoding": {
        "x": {"field": "t", "type": "quantitative"},
        "y": {"field": "v", "type": "quantitative"}
      }
    },
    {
      "data": {"values": [{"t": 1, "v": 5}, {"t": 2, "v": 15}, {"t": 3, "v": 25}]},
      "mark": {"type": "point"},
      "encoding": {
        "x": {"field": "t", "type": "quantitative"},
        "y": {"field": "v", "type": "quantitative"}
      }
    }
  ]
}`

// TestPrismEncodeLayerNullDropIsPerLayer: the layer composite runs the
// same policy, and the warning names the layer that shed rows while
// the other layer keeps every row.
func TestPrismEncodeLayerNullDropIsPerLayer(t *testing.T) {
	s, err := spec.DecodeBytes([]byte(layeredNullSpec))
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
		res, rerr := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if rerr != nil {
			t.Fatalf("Execute child %d: %v", i, rerr)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	sc := &doc.Grid.Cells[0].Scene
	if len(sc.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(sc.Layers))
	}
	if n := len(sc.Layers[0].Marks[0].Line.Points); n != 2 {
		t.Errorf("layer-0 points = %d, want 2", n)
	}
	if n := len(sc.Layers[1].Marks); n != 3 {
		t.Errorf("layer-1 marks = %d, want 3 (untouched)", n)
	}
	warn := nullWarningOf(t, doc)
	if warn == nil {
		t.Fatalf("no PRISM_WARN_NULL_DROPPED in %+v", doc.Warnings)
	}
	if warn.Layer != "layer-0" {
		t.Errorf("layer = %q, want layer-0", warn.Layer)
	}
}
