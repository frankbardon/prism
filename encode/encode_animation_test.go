package encode_test

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

func buildAndExecute(t *testing.T, s *spec.Spec) (map[plan.NodeID]*table.Table, plan.NodeID) {
	t.Helper()
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
	return res.Tables, tipID
}

// TestEncodeNoAnimationLocksGoldens asserts that encoding a spec
// without an `animation` block produces a SceneDoc whose top scene
// has Animation == nil and whose per-row marks carry no Key. This is
// the regression lock that protects every existing SVG / PDF golden
// — both renderers ignore the new fields, but if the encoder ever
// starts emitting them by accident, omitempty on the JSON tags
// would still surface a diff in scene IR fixtures.
func TestEncodeNoAnimationLocksGoldens(t *testing.T) {
	s, tables, tipID := runPipeline(t, "bar_basic.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	cell := doc.Grid.Cells[0]
	if cell.Scene.Animation != nil {
		t.Errorf("scene.Animation = %+v; want nil (no animation block in spec)", cell.Scene.Animation)
	}
	for li, layer := range cell.Scene.Layers {
		for mi, m := range layer.Marks {
			if m.Key != "" {
				t.Errorf("layer[%d].mark[%d].Key = %q; want empty (no key:true channel)", li, mi, m.Key)
			}
		}
	}
}

// TestEncodeAnimationPopulatesScene confirms that a spec carrying an
// animation block + a key:true channel produces a scene whose
// Animation field is populated with defaults applied and whose per-
// row marks carry a non-empty Key string.
func TestEncodeAnimationPopulatesScene(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {
			"name": "brand_scores",
			"values": [
				{"brand_id": "alpha", "score": 0.42},
				{"brand_id": "beta",  "score": 0.71}
			]
		},
		"mark": "bar",
		"encoding": {
			"x": {"field": "brand_id", "type": "nominal", "key": true},
			"y": {"field": "score",    "type": "quantitative"}
		},
		"animation": {"duration_ms": 800}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	tables, tipID := buildAndExecute(t, s)
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	scene0 := doc.Grid.Cells[0].Scene
	if scene0.Animation == nil {
		t.Fatalf("scene.Animation = nil; want populated")
	}
	if got, want := scene0.Animation.DurationMs, 800; got != want {
		t.Errorf("Animation.DurationMs = %d; want %d", got, want)
	}
	if got, want := scene0.Animation.Easing, spec.AnimationDefaultEasing; got != want {
		t.Errorf("Animation.Easing = %q; want default %q", got, want)
	}
	if got, want := scene0.Animation.Enter, spec.AnimationDefaultEnter; got != want {
		t.Errorf("Animation.Enter = %q; want default %q", got, want)
	}

	if len(scene0.Layers) == 0 || len(scene0.Layers[0].Marks) == 0 {
		t.Fatalf("scene has no marks")
	}
	for i, m := range scene0.Layers[0].Marks {
		if !strings.HasPrefix(m.Key, "brand_id=") {
			t.Errorf("mark[%d].Key = %q; want prefix %q", i, m.Key, "brand_id=")
		}
	}
}
