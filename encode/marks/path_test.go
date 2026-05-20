package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func TestPrismEncodePathPrimitive(t *testing.T) {
	tbl := buildTable(t, map[string]any{"_": []float64{0}})
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Mark:   &spec.MarkDef{Type: "path", Path: "M 0 0 L 10 10 L 5 5 Z"},
	}
	marks, err := encodePath(in)
	if err != nil {
		t.Fatalf("encodePath: %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("want 1 mark, got %d", len(marks))
	}
	if marks[0].Type != scene.MarkPath {
		t.Errorf("type = %v, want MarkPath", marks[0].Type)
	}
	if marks[0].Path == nil {
		t.Fatal("Path geom nil")
	}
	if marks[0].Path.D != "M 0 0 L 10 10 L 5 5 Z" {
		t.Errorf("d = %q", marks[0].Path.D)
	}
}

func TestPrismEncodePathEmptyRejected(t *testing.T) {
	tbl := buildTable(t, map[string]any{"_": []float64{0}})
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Mark:   &spec.MarkDef{Type: "path", Path: ""},
	}
	_, err := encodePath(in)
	if err == nil {
		t.Fatal("expected error for empty d")
	}
}
