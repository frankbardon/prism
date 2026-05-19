package encode_test

import (
	"context"
	"os"
	"path/filepath"
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

// TestPrismEncodeRepeatRowMajor pins repeat row layout: two cells
// stacked vertically, each holding the corresponding substituted-y
// chart.
func TestPrismEncodeRepeatRowMajor(t *testing.T) {
	root := repoRootForFacetTest(t)
	path := filepath.Join(root, "testdata", "specs", "repeat_metrics.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
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
	if doc.Grid.Layout.Rows != 2 || doc.Grid.Layout.Cols != 1 {
		t.Errorf("Layout = %dx%d, want 2x1", doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
	}
	if len(doc.Grid.Cells) != 2 {
		t.Fatalf("Cells = %d, want 2", len(doc.Grid.Cells))
	}
	// Row 0 stacked above row 1: cells[0].Row == 0, cells[1].Row == 1.
	if doc.Grid.Cells[0].Row != 0 || doc.Grid.Cells[1].Row != 1 {
		t.Errorf("row order = %d, %d; want 0, 1",
			doc.Grid.Cells[0].Row, doc.Grid.Cells[1].Row)
	}
	// Default repeat resolve is independent: no shared Y axis.
	if doc.Grid.Shared.Y != nil {
		t.Error("Grid.Shared.Y unexpectedly populated under default repeat resolve (D057)")
	}
}

// TestPrismEncodeRepeatColumnMajor mirrors the row case for column.
func TestPrismEncodeRepeatColumnMajor(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [
			{"day": "2026-01-01", "score": 0.4, "lift": 1.2, "share": 0.5},
			{"day": "2026-01-02", "score": 0.5, "lift": 1.3, "share": 0.55}
		]},
		"repeat": {"column": ["score", "lift", "share"]},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "line",
			"encoding": {
				"x": {"field": "day", "type": "temporal"},
				"y": {"field": {"repeat": "column"}, "type": "quantitative"}
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
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{Width: 1200, Height: 400})
	if err != nil {
		t.Fatalf("Enc: %v", err)
	}
	if doc.Grid.Layout.Rows != 1 || doc.Grid.Layout.Cols != 3 {
		t.Errorf("Layout = %dx%d, want 1x3", doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
	}
	if len(doc.Grid.Cells) != 3 {
		t.Fatalf("Cells = %d, want 3", len(doc.Grid.Cells))
	}
	for i, cell := range doc.Grid.Cells {
		if cell.Col != i {
			t.Errorf("cell %d Col=%d, want %d", i, cell.Col, i)
		}
	}
}
