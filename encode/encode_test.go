package encode_test

import (
	"context"
	"os"
	"path/filepath"
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

// runPipeline loads a fixture spec, builds the DAG, executes through
// the in-memory backend, and returns (spec, tables, tip id) ready
// for an Encode call.
func runPipeline(t *testing.T, fixture string) (*spec.Spec, map[plan.NodeID]*table.Table, plan.NodeID) {
	t.Helper()
	path := repoFixture(t, fixture)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build %s: %v", path, err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("execute %s: %v", path, err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute %s: %d node errors: %v", path, len(res.Errors), res.Errors)
	}
	return s, res.Tables, tipID
}

func TestPrismEncodeBarBasic(t *testing.T) {
	s, tables, tipID := runPipeline(t, "bar_basic.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if doc.Grid.Layout.Rows != 1 || doc.Grid.Layout.Cols != 1 {
		t.Errorf("Grid layout = %dx%d, want 1x1", doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
	}
	if len(doc.Grid.Cells) != 1 {
		t.Fatalf("Cells len = %d, want 1", len(doc.Grid.Cells))
	}
	sceneObj := doc.Grid.Cells[0].Scene
	if len(sceneObj.Axes) != 2 {
		t.Errorf("axes = %d, want 2", len(sceneObj.Axes))
	}
	if len(sceneObj.Layers) != 1 {
		t.Fatalf("layers = %d, want 1", len(sceneObj.Layers))
	}
	layer := sceneObj.Layers[0]
	if layer.Mark != scene.MarkRect {
		t.Errorf("layer.Mark = %q, want rect", layer.Mark)
	}
	if len(layer.Marks) != 3 {
		t.Errorf("marks = %d, want 3", len(layer.Marks))
	}
	for i, m := range layer.Marks {
		if m.Rect == nil {
			t.Errorf("marks[%d].Rect is nil", i)
		}
	}
}

func TestPrismEncodeNestingAlwaysFull(t *testing.T) {
	fixtures := []string{
		"bar_basic.json",
		"line_basic.json",
		"area_basic.json",
		"point_scatter.json",
	}
	for _, f := range fixtures {
		t.Run(f, func(t *testing.T) {
			s, tables, tipID := runPipeline(t, f)
			doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if doc.Grid.Layout.Rows != 1 || doc.Grid.Layout.Cols != 1 {
				t.Errorf("Grid layout = %dx%d, want 1x1 (full nesting always)",
					doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
			}
			if len(doc.Grid.Cells) != 1 {
				t.Errorf("Cells = %d, want 1", len(doc.Grid.Cells))
			}
		})
	}
}

// TestPrismEncodeNoTimeStubWarning asserts that T06.04's calendar-aware
// time ticks supersede the P05 placeholder warning.
func TestPrismEncodeNoTimeStubWarning(t *testing.T) {
	s, tables, tipID := runPipeline(t, "line_basic.json")
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for _, w := range doc.Warnings {
		if w.Code == scene.WarnTimeScaleStubbed {
			t.Errorf("unexpected WarnTimeScaleStubbed in doc.Warnings: %+v", w)
		}
	}
}

// repoFixture resolves a path under testdata/specs/ relative to the
// repository root.
func repoFixture(t *testing.T, name string) string {
	t.Helper()
	root := repoRoot(t)
	return filepath.Join(root, "testdata", "specs", name)
}

// repoRoot walks up from the current directory looking for go.mod.
func repoRoot(t *testing.T) string {
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
			t.Fatalf("could not find go.mod from %s", dir)
		}
		dir = parent
	}
}
