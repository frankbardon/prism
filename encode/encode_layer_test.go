package encode_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// runComposite drives a composite spec through Build → Execute and
// returns one per-child table map (positional) alongside the composite
// plan and the spec. Per-child maps avoid NodeID-collision between
// sibling sub-DAGs whose auto-counters land on the same string.
func runComposite(t *testing.T, fixturePath string) (*spec.Spec, *plan.CompositeDAG, []map[plan.NodeID]*table.Table) {
	t.Helper()
	body, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read %s: %v", fixturePath, err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode %s: %v", fixturePath, err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite %s: %v", fixturePath, err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute child %d: %v", i, err)
		}
		if len(res.Errors) > 0 {
			t.Fatalf("child %d had %d errors: %v", i, len(res.Errors), res.Errors)
		}
		per[i] = res.Tables
	}
	return s, c, per
}

func TestPrismEncodeLayerEmitsTwoSceneLayers(t *testing.T) {
	root := repoRootForTest(t)
	path := filepath.Join(root, "examples", "specs", "layer_actual_vs_benchmark.json")
	s, c, all := runComposite(t, path)

	doc, err := encode.EncodeComposite(s, c, all, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if doc.Grid.Layout.Rows != 1 || doc.Grid.Layout.Cols != 1 {
		t.Errorf("Layer grid = %dx%d, want 1x1", doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
	}
	if len(doc.Grid.Cells) != 1 {
		t.Fatalf("Cells = %d, want 1", len(doc.Grid.Cells))
	}
	sc := doc.Grid.Cells[0].Scene
	if len(sc.Layers) != 2 {
		t.Errorf("Layers = %d, want 2", len(sc.Layers))
	}
	for i, l := range sc.Layers {
		if l.ZIndex != i {
			t.Errorf("layer %d ZIndex=%d, want %d (positional)", i, l.ZIndex, i)
		}
		if len(l.Marks) == 0 {
			t.Errorf("layer %d has no marks", i)
		}
	}
}

func TestPrismEncodeLayerSharedYScale(t *testing.T) {
	// Two layers, both numeric y, resolve.scale.y: shared. The
	// resolved shared y scale must span the union of both layers'
	// y domains.
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"resolve": {"scale": {"y": "shared"}},
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
	s, c, all := decodeAndRunComposite(t, body)
	doc, err := encode.EncodeComposite(s, c, all, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if doc.Grid.Shared.Y == nil {
		t.Fatal("Grid.Shared.Y is nil; want shared y-axis")
	}
	// Per-cell axes must NOT contain y (it lives on Shared.Y).
	for _, ax := range doc.Grid.Cells[0].Scene.Axes {
		if ax.Channel == scene.ChannelY {
			t.Errorf("cell carries a y-axis as well as Shared.Y (D051 violation)")
		}
	}
	dom := doc.Grid.Shared.Y.Scale.Domain
	if len(dom) < 2 {
		t.Fatalf("shared y domain=%v, want [min,max]", dom)
	}
	mn := dom[0].(float64)
	mx := dom[1].(float64)
	if mn != 0 {
		t.Errorf("shared y min=%v, want 0 (padded)", mn)
	}
	if mx < 100 {
		t.Errorf("shared y max=%v, want >=100", mx)
	}
}

func TestPrismEncodeLayerSharedIncompatibleRaises005(t *testing.T) {
	// bar layer with nominal y (band) + line layer with quantitative y
	// (linear) under resolve.scale.y: shared.
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"resolve": {"scale": {"y": "shared"}},
		"layer": [
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": 1, "y": "alpha"}, {"x": 2, "y": "beta"}]},
				"mark": "point",
				"encoding": {
					"x": {"field": "x", "type": "quantitative"},
					"y": {"field": "y", "type": "nominal"}
				}
			},
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": 1, "y": 10.0}, {"x": 2, "y": 20.0}]},
				"mark": "line",
				"encoding": {
					"x": {"field": "x", "type": "quantitative"},
					"y": {"field": "y", "type": "quantitative"}
				}
			}
		]
	}`)
	s, c, all := decodeAndRunComposite(t, body)
	_, err := encode.EncodeComposite(s, c, all, encode.EncodeOpts{})
	if err == nil {
		t.Fatal("expected PRISM_PLAN_005, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_005" {
		t.Errorf("expected PRISM_PLAN_005, got %v", err)
	}
}

func TestPrismEncodeLayerMissingTipEmitsSkippedWarning(t *testing.T) {
	root := repoRootForTest(t)
	path := filepath.Join(root, "examples", "specs", "layer_actual_vs_benchmark.json")
	s, c, per := runComposite(t, path)
	// Drop the second child's tip table to simulate a partial failure.
	delete(per[1], c.Children[1].Tip)
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite (with skip): %v", err)
	}
	if len(doc.Grid.Cells[0].Scene.Layers) != 1 {
		t.Errorf("Layers=%d, want 1 (other layer skipped)", len(doc.Grid.Cells[0].Scene.Layers))
	}
	hasSkip := false
	for _, w := range doc.Warnings {
		if w.Code == scene.WarnLayerSkipped {
			hasSkip = true
		}
	}
	if !hasSkip {
		t.Errorf("expected PRISM_WARN_LAYER_SKIPPED, got warnings=%v", doc.Warnings)
	}
}

func TestPrismEncodeLayerIndependentColorEmitsLegendPerLayer(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"resolve": {"scale": {"color": "independent"}},
		"layer": [
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": "a", "y": 1, "k": "alpha"}, {"x": "b", "y": 2, "k": "beta"}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "x", "type": "nominal"},
					"y": {"field": "y", "type": "quantitative"},
					"color": {"field": "k", "type": "nominal"}
				}
			},
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": "a", "y": 5, "g": "one"}, {"x": "b", "y": 6, "g": "two"}]},
				"mark": "point",
				"encoding": {
					"x": {"field": "x", "type": "nominal"},
					"y": {"field": "y", "type": "quantitative"},
					"color": {"field": "g", "type": "nominal"}
				}
			}
		]
	}`)
	s, c, all := decodeAndRunComposite(t, body)
	doc, err := encode.EncodeComposite(s, c, all, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	colorLegends := 0
	for _, l := range doc.Grid.Cells[0].Scene.Legends {
		if l.Channel == scene.ChannelColor {
			colorLegends++
		}
	}
	if colorLegends < 2 {
		t.Errorf("color legends=%d, want >=2 (independent → per-layer)", colorLegends)
	}
}

// decodeAndRunComposite is the inline-spec counterpart to
// runComposite; tests that build their spec from bytes use it.
func decodeAndRunComposite(t *testing.T, body []byte) (*spec.Spec, *plan.CompositeDAG, []map[plan.NodeID]*table.Table) {
	t.Helper()
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
		if len(res.Errors) > 0 {
			t.Fatalf("child %d errors: %v", i, res.Errors)
		}
		per[i] = res.Tables
	}
	return s, c, per
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
