package encode

import (
	"strings"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Drives the geoshape mark from a synthetic table through Encode and
// asserts a scene polygon falls inside the plot rectangle. Acts as
// the smoke test until P18 gets golden SVG fixtures.
func TestEncodeGeoshape(t *testing.T) {
	tbl := newGeoTable(t)
	s := &spec.Spec{
		Mark: &spec.Mark{Shorthand: "geoshape"},
		Encoding: &spec.Encoding{
			Feature: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "id", Type: "nominal"}},
		},
		Projection: &spec.Projection{Type: "equirectangular"},
	}
	const tipID plan.NodeID = "tip"
	doc, err := Encode(s, map[plan.NodeID]*table.Table{tipID: tbl}, tipID, EncodeOpts{Width: 800, Height: 400})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if doc == nil || len(doc.Grid.Cells) == 0 {
		t.Fatal("no grid cells")
	}
	cell := doc.Grid.Cells[0]
	if len(cell.Scene.Layers) == 0 {
		t.Fatal("no layers")
	}
	layer := cell.Scene.Layers[0]
	if layer.Mark != scene.MarkGeoshape {
		t.Fatalf("layer mark = %q, want %q", layer.Mark, scene.MarkGeoshape)
	}
	if len(layer.Marks) == 0 {
		t.Fatal("no marks emitted")
	}
	first := layer.Marks[0]
	if first.Geoshape == nil || len(first.Geoshape.Outer) < 3 {
		t.Fatalf("first mark missing polygon")
	}
}

func TestEncodeGeoshape_MissingFeatureChannel(t *testing.T) {
	tbl := newGeoTable(t)
	s := &spec.Spec{
		Mark:       &spec.Mark{Shorthand: "geoshape"},
		Encoding:   &spec.Encoding{},
		Projection: &spec.Projection{Type: "mercator"},
	}
	const tipID plan.NodeID = "tip"
	_, err := Encode(s, map[plan.NodeID]*table.Table{tipID: tbl}, tipID, EncodeOpts{})
	if err == nil || !strings.Contains(err.Error(), "feature") {
		t.Fatalf("expected feature-channel error, got %v", err)
	}
}

func TestEncodeGeoshape_UnknownProjection(t *testing.T) {
	tbl := newGeoTable(t)
	s := &spec.Spec{
		Mark: &spec.Mark{Shorthand: "geoshape"},
		Encoding: &spec.Encoding{
			Feature: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "id", Type: "nominal"}},
		},
		Projection: &spec.Projection{Type: "fishbowl"},
	}
	const tipID plan.NodeID = "tip"
	_, err := Encode(s, map[plan.NodeID]*table.Table{tipID: tbl}, tipID, EncodeOpts{})
	if err == nil || !strings.Contains(err.Error(), "fishbowl") {
		t.Fatalf("expected unknown-projection error, got %v", err)
	}
}

func newGeoTable(t *testing.T) *table.Table {
	t.Helper()
	rows := []string{"USA", "CAN", "MEX"}
	sch := &encoding.Schema{Fields: []encoding.Field{{Name: "id", Type: encoding.FieldTypeCategoricalU8}}}
	tbl, err := table.NewTable(sch, map[string]table.Column{"id": table.StringColumn(rows)}, len(rows), "test-hash")
	if err != nil {
		t.Fatalf("table build: %v", err)
	}
	return tbl
}
