package svg_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// renderCompositeFixture loads a composite fixture and runs it
// through the BuildComposite → per-child Execute → EncodeComposite →
// svg.Render pipeline.
func renderCompositeFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "specs", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute child %d: %v", i, err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	bytes, err := svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return bytes
}

// TestPrismRenderSingleCellGridUnchanged asserts that pre-existing
// goldens (1×1 grids produced by Encode) still round-trip byte-
// identical through the post-P08 renderer. The grid changes are
// additive — the flat-chart path stays untouched.
func TestPrismRenderSingleCellGridUnchanged(t *testing.T) {
	// Walk the existing 1×1 goldens; renderFixture goes through the
	// flat Encode path. The post-P08 renderer must produce the same
	// bytes.
	fixtures := []string{
		"bar_basic.json",
		"line_basic.json",
		"area_basic.json",
		"point_scatter.json",
		"rule_basic.json",
	}
	for _, fix := range fixtures {
		fix := fix
		t.Run(fix, func(t *testing.T) {
			got, err := renderFixture(t, fix)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			goldenName := strings.TrimSuffix(fix, ".json") + ".svg"
			goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", goldenName)
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("golden drift on %s (%d vs %d bytes)", fix, len(got), len(want))
			}
		})
	}
}

func TestPrismRenderConcatEmitsCellCount(t *testing.T) {
	out := renderCompositeFixture(t, "concat_h.json")
	count := strings.Count(string(out), `<g class="prism-scene"`)
	if count != 2 {
		t.Errorf(`prism-scene blocks = %d, want 2 (one per concat cell); got SVG:\n%s`, count, truncate(out, 800))
	}
}

func TestPrismRenderSharedAxisEmittedOnce(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"resolve": {"scale": {"y": "shared"}, "axis": {"y": "shared"}},
		"layer": [
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": "a", "y": 10}, {"x": "b", "y": 50}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "x", "type": "nominal"},
					"y": {"field": "y", "type": "quantitative"}
				}
			},
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": "a", "y": 40}, {"x": "b", "y": 100}]},
				"mark": "rule",
				"encoding": {
					"x": {"field": "x", "type": "nominal"},
					"y": {"field": "y", "type": "quantitative"}
				}
			}
		]
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute child %d: %v", i, err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if doc.Grid.Shared.Y == nil {
		t.Fatal("Grid.Shared.Y is nil; cannot test renderer emit-once")
	}
	out, err := svg.New().Render(doc, render.RenderOpts{Format: "svg"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Expect exactly one shared-axis group (data-shared="true").
	if got := strings.Count(string(out), `data-shared="true"`); got != 1 {
		t.Errorf(`data-shared="true" count=%d, want 1`, got)
	}
	// Cell-level y axis must not be present in the shared-y path —
	// the encoder strips it (D051).
	if !strings.Contains(string(out), `data-shared="true"`) {
		t.Errorf("missing data-shared attribute on shared axis")
	}
}
