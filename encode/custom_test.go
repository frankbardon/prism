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

// customFixtureJSON is a custom-mark spec whose transform chain
// filters to region=="west", exercising the same standard transform-
// pipeline resolution any other mark's upstream table gets (E2-S2
// story point 4).
const customFixtureJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "region": "west", "revenue": 120},
      {"name": "Globex", "region": "east", "revenue": 80},
      {"name": "Initech", "region": "west", "revenue": 200}
    ]
  },
  "transform": [
    {"filter": {"op": "eq", "field": "region", "value": "west"}}
  ],
  "mark": {"type": "custom", "renderer": "my-viz"},
  "encoding": {}
}`

// runCustomFixture decodes, builds, executes, and encodes body,
// returning the resulting SceneDoc.
func runCustomFixture(t *testing.T, body string) *scene.SceneDoc {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
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
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return doc
}

// TestPrismCustomMarkTransformPipeline proves a custom mark's data
// resolves through the standard transform pipeline exactly as for any
// other mark, and that the renderer name / row set / box land on
// scene.Custom.
func TestPrismCustomMarkTransformPipeline(t *testing.T) {
	doc := runCustomFixture(t, customFixtureJSON)
	if len(doc.Grid.Cells) != 1 {
		t.Fatalf("want 1 cell, got %d", len(doc.Grid.Cells))
	}
	custom := doc.Grid.Cells[0].Scene.Custom
	if custom == nil {
		t.Fatalf("Scene.Custom is nil")
	}
	if custom.Renderer != "my-viz" {
		t.Errorf("Renderer = %q, want %q", custom.Renderer, "my-viz")
	}
	if len(custom.Rows) != 2 {
		t.Fatalf("want 2 filtered rows, got %d: %v", len(custom.Rows), custom.Rows)
	}
	for _, row := range custom.Rows {
		if row["region"] != "west" {
			t.Errorf("row region = %v, want %q (filter should have already applied)", row["region"], "west")
		}
	}
	if custom.Box.W <= 0 || custom.Box.H <= 0 {
		t.Errorf("Box = %+v, want positive dimensions", custom.Box)
	}

	layer := doc.Grid.Cells[0].Scene.Layers
	if len(layer) != 1 || layer[0].Mark != scene.MarkCustom {
		t.Errorf("Layers = %+v, want single MarkCustom layer", layer)
	}
}

// TestPrismCustomMarkEmptyRendererName covers the spec-valid corner
// case of a custom mark with no renderer name set — encode has no
// opinion on whether it's registered, so this must still succeed
// (an unregistered/empty name is a render-time error, not an
// encode-time one).
func TestPrismCustomMarkEmptyRendererName(t *testing.T) {
	const body = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"x": 1}]},
  "mark": {"type": "custom"},
  "encoding": {}
}`
	doc := runCustomFixture(t, body)
	custom := doc.Grid.Cells[0].Scene.Custom
	if custom == nil {
		t.Fatalf("Scene.Custom is nil")
	}
	if custom.Renderer != "" {
		t.Errorf("Renderer = %q, want empty", custom.Renderer)
	}
}
