package encode_test

import (
	"encoding/json"
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// E8-S2 end to end: `title_padding` reaches the Scene IR from all
// three levels the renderer resolves between — the channel's own
// `axis` block, the theme's per-axis `axis_x` / `axis_y` block, and
// the shared `axis` block.
const axisTitlePaddingSpec = `{
	"$schema": "urn:prism:schema:v1:spec",
	"data": {"values": [{"region": "west", "score": 0.42}, {"region": "east", "score": 0.91}]},
	"mark": "line",
	"encoding": {
		"x": {"field": "region", "type": "nominal", "axis": {"title_padding": 24}},
		"y": {"field": "score",  "type": "quantitative"}
	},
	"theme": {
		"name": "light",
		"axis":   {"title_padding": 12},
		"axis_x": {"title_padding": 30},
		"axis_y": {"grid_color": "#ff0000"}
	}
}`

func TestEncodeAxisTitlePaddingReachesTheScene(t *testing.T) {
	body := []byte(axisTitlePaddingSpec)

	sv, err := validate.NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if errs := sv.Validate(raw); len(errs) != 0 {
		t.Fatalf("shape validation rejected axis.title_padding: %v", errs)
	}

	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	if s.Encoding.X.Axis == nil || s.Encoding.X.Axis.TitlePadding == nil || *s.Encoding.X.Axis.TitlePadding != 24 {
		t.Fatalf("axis.title_padding did not decode: %+v", s.Encoding.X.Axis)
	}

	tables, tipID := buildAndExecute(t, s)
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// The spec level lands on the axis itself; the renderer gives it
	// priority over both theme levels.
	sc := doc.Grid.Cells[0].Scene
	var sawX bool
	for _, a := range sc.Axes {
		if a.Channel != "x" {
			continue
		}
		sawX = true
		if a.TitlePadding == nil || *a.TitlePadding != 24 {
			t.Errorf("spec axis.title_padding did not reach scene.Axis: %+v", a.TitlePadding)
		}
	}
	if !sawX {
		t.Fatal("no x axis in the encoded scene")
	}

	// The shared theme token and the per-axis override both ride the
	// Scene IR, since a CSS variable cannot move a <text> coordinate.
	if doc.Theme.AxisTitlePadding == nil || *doc.Theme.AxisTitlePadding != 12 {
		t.Errorf("theme axis.title_padding did not reach the Scene IR: %+v", doc.Theme.AxisTitlePadding)
	}
	if doc.Theme.AxisX == nil || doc.Theme.AxisX.TitlePadding == nil || *doc.Theme.AxisX.TitlePadding != 30 {
		t.Errorf("theme axis_x.title_padding did not reach the Scene IR: %+v", doc.Theme.AxisX)
	}
	// E8-S1's zero-byte property survives: axis_y states colour only.
	if doc.Theme.AxisY != nil {
		t.Errorf("a colour-only axis_y added Scene IR bytes: %+v", doc.Theme.AxisY)
	}
}
