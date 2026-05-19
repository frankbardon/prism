package encode_test

import (
	"path/filepath"
	"testing"

	"github.com/frankbardon/prism/encode"
)

func TestPrismEncodeConcatHorizontalProducesRowMajorCells(t *testing.T) {
	root := repoRootForTest(t)
	path := filepath.Join(root, "testdata", "specs", "concat_h.json")
	s, c, per := runComposite(t, path)

	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if doc.Grid.Layout.Rows != 1 || doc.Grid.Layout.Cols != 2 {
		t.Errorf("hconcat grid = %dx%d, want 1x2", doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
	}
	if len(doc.Grid.Cells) != 2 {
		t.Fatalf("Cells=%d, want 2", len(doc.Grid.Cells))
	}
	for i, cell := range doc.Grid.Cells {
		if cell.Row != 0 {
			t.Errorf("cell %d Row=%d, want 0", i, cell.Row)
		}
		if cell.Col != i {
			t.Errorf("cell %d Col=%d, want %d", i, cell.Col, i)
		}
	}
}

func TestPrismEncodeConcatVerticalProducesRowMajorCells(t *testing.T) {
	root := repoRootForTest(t)
	path := filepath.Join(root, "testdata", "specs", "concat_v.json")
	s, c, per := runComposite(t, path)

	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if doc.Grid.Layout.Rows != 2 || doc.Grid.Layout.Cols != 1 {
		t.Errorf("vconcat grid = %dx%d, want 2x1", doc.Grid.Layout.Rows, doc.Grid.Layout.Cols)
	}
	if len(doc.Grid.Cells) != 2 {
		t.Fatalf("Cells=%d, want 2", len(doc.Grid.Cells))
	}
	for i, cell := range doc.Grid.Cells {
		if cell.Col != 0 {
			t.Errorf("cell %d Col=%d, want 0", i, cell.Col)
		}
		if cell.Row != i {
			t.Errorf("cell %d Row=%d, want %d", i, cell.Row, i)
		}
	}
}

func TestPrismEncodeConcatChildOffsetsApplied(t *testing.T) {
	root := repoRootForTest(t)
	pathH := filepath.Join(root, "testdata", "specs", "concat_h.json")
	s, c, per := runComposite(t, pathH)

	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{Width: 800, Height: 400})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if len(doc.Grid.Cells) != 2 {
		t.Fatalf("Cells=%d, want 2", len(doc.Grid.Cells))
	}
	// Cell 1's Plot.X should be offset by approximately the first cell's
	// width (cell width = (800 - gap) / 2 = 390; gap = 20). Cell[0].Plot.X
	// = pad.Left (40); cell[1].Plot.X = 40 + 390 + 20 = 450.
	c0 := doc.Grid.Cells[0].Scene.Plot.X
	c1 := doc.Grid.Cells[1].Scene.Plot.X
	if c1 <= c0+100 {
		t.Errorf("hconcat: cell[1].Plot.X=%v <= cell[0].Plot.X+100=%v (no offset applied?)", c1, c0+100)
	}

	pathV := filepath.Join(root, "testdata", "specs", "concat_v.json")
	sv, cv, perv := runComposite(t, pathV)
	docV, err := encode.EncodeComposite(sv, cv, perv, encode.EncodeOpts{Width: 600, Height: 800})
	if err != nil {
		t.Fatalf("EncodeComposite vconcat: %v", err)
	}
	if len(docV.Grid.Cells) != 2 {
		t.Fatalf("vconcat Cells=%d, want 2", len(docV.Grid.Cells))
	}
	r0 := docV.Grid.Cells[0].Scene.Plot.Y
	r1 := docV.Grid.Cells[1].Scene.Plot.Y
	if r1 <= r0+100 {
		t.Errorf("vconcat: cell[1].Plot.Y=%v <= cell[0].Plot.Y+100=%v", r1, r0+100)
	}
}
