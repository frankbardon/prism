package encode_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

func repoRootForFacetTest(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Dir(here)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", here)
		}
		dir = parent
	}
}

// runFacetSpec is the facet-test driver: load fixture, build, execute,
// encode, return (spec, composite, tables, doc).
func runFacetSpec(t *testing.T, fixture string) (*spec.Spec, *plan.CompositeDAG, []map[plan.NodeID]*table.Table, *scene.SceneDoc) {
	t.Helper()
	root := repoRootForFacetTest(t)
	path := filepath.Join(root, "testdata", "specs", fixture)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
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
	return s, c, per, doc
}

// TestPrismEncodeFacetByColumn renders facet_by_region.json (1-row
// fixture facets by column over 2-3 distinct regions). Default
// shared y-axis must produce Grid.Shared.Y and zero per-cell y-axes.
func TestPrismEncodeFacetByColumn(t *testing.T) {
	_, _, _, doc := runFacetSpec(t, "facet_by_region.json")
	if len(doc.Grid.Cells) == 0 {
		t.Fatal("no cells")
	}
	for i, cell := range doc.Grid.Cells {
		for _, ax := range cell.Scene.Axes {
			if ax.Channel == scene.ChannelY {
				t.Errorf("cell %d still carries y-axis under shared resolve (D051)", i)
			}
		}
	}
	if doc.Grid.Shared.Y == nil {
		t.Error("Grid.Shared.Y is nil; expected one shared y-axis under default facet resolve (D057)")
	}
}

// TestPrismEncodeFacetPartitionOrder pins first-seen partition
// ordering: NA appears before EU in the inline-row order.
func TestPrismEncodeFacetPartitionOrder(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [
			{"r": "NA", "b": "alpha", "v": 1.0},
			{"r": "EU", "b": "alpha", "v": 2.0},
			{"r": "NA", "b": "beta",  "v": 3.0},
			{"r": "EU", "b": "beta",  "v": 4.0}
		]},
		"facet": {"column": {"field": "r", "type": "nominal"}},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "bar",
			"encoding": {
				"x": {"field": "b", "type": "nominal"},
				"y": {"field": "v", "type": "quantitative"}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS: afero.NewOsFs(), Resolver: resolve.New(nil), Backend: inmem.New(),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, ch := range c.Children {
		res, err := plan.Execute(context.Background(), ch.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Exec: %v", err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Enc: %v", err)
	}
	if len(doc.Grid.Cells) != 2 {
		t.Fatalf("cells=%d, want 2", len(doc.Grid.Cells))
	}
	// First-seen order: NA (col 0), EU (col 1).
	if doc.Grid.Cells[0].Col != 0 || doc.Grid.Cells[1].Col != 1 {
		t.Errorf("cell column order = %d, %d; want 0, 1",
			doc.Grid.Cells[0].Col, doc.Grid.Cells[1].Col)
	}
}

// TestPrismEncodeFacetPerCellIndependentY confirms that an
// `independent` y-resolve produces per-cell y axes (no shared y).
func TestPrismEncodeFacetPerCellIndependentY(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [
			{"r": "A", "x": "p", "v": 1.0},
			{"r": "A", "x": "q", "v": 2.0},
			{"r": "B", "x": "p", "v": 50.0},
			{"r": "B", "x": "q", "v": 80.0}
		]},
		"facet":   {"column": {"field": "r", "type": "nominal"}},
		"resolve": {"scale": {"y": "independent"}},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "bar",
			"encoding": {
				"x": {"field": "x", "type": "nominal"},
				"y": {"field": "v", "type": "quantitative"}
			}
		}
	}`)
	s, _ := spec.DecodeBytes(body)
	c, err := build.BuildComposite(s, build.Options{
		FS: afero.NewOsFs(), Resolver: resolve.New(nil), Backend: inmem.New(),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, ch := range c.Children {
		res, err := plan.Execute(context.Background(), ch.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Exec: %v", err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Enc: %v", err)
	}
	if doc.Grid.Shared.Y != nil {
		t.Error("Shared.Y must be nil under independent y resolve")
	}
	gotYAxes := 0
	for _, cell := range doc.Grid.Cells {
		for _, ax := range cell.Scene.Axes {
			if ax.Channel == scene.ChannelY {
				gotYAxes++
			}
		}
	}
	if gotYAxes != len(doc.Grid.Cells) {
		t.Errorf("per-cell y-axis count = %d, want %d (one per cell)", gotYAxes, len(doc.Grid.Cells))
	}
}
