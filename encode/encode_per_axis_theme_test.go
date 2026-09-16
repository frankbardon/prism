package encode_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// E8-S1 end to end: a spec-level `theme.axis_x` / `theme.axis_y` block
// survives decode + schema validation and reaches the encoded scene as
// both a scoped CSS declaration (colour) and a Scene IR token
// (geometry).
const perAxisThemeSpec = `{
	"$schema": "urn:prism:schema:v1:spec",
	"data": {"values": [{"region": "west", "score": 0.42}, {"region": "east", "score": 0.91}]},
	"mark": "line",
	"encoding": {
		"x": {"field": "region", "type": "nominal", "axis": {"grid": true}},
		"y": {"field": "score",  "type": "quantitative", "axis": {"grid": true}}
	},
	"theme": {
		"name": "light",
		"axis":   {"grid_color": "#cccccc"},
		"axis_x": {"grid_color": "#ff0000"},
		"axis_y": {"tick_size": 12}
	}
}`

func TestEncodePerAxisThemeBlocksReachTheScene(t *testing.T) {
	body := []byte(perAxisThemeSpec)

	// The schema bundle must accept the new keys — `theme_override`
	// declares additionalProperties:false, so an unregistered key here
	// would be a hard shape error rather than a silent no-op.
	sv, err := validate.NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if errs := sv.Validate(raw); len(errs) != 0 {
		t.Fatalf("shape validation rejected axis_x/axis_y: %v", errs)
	}

	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	if s.Theme == nil || s.Theme.AxisX == nil || s.Theme.AxisX.GridColor != "#ff0000" {
		t.Fatalf("axis_x did not decode: %+v", s.Theme)
	}
	tables, tipID := buildAndExecute(t, s)
	doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Colour rides the CSS cascade, scoped to the x axis's own group.
	css := doc.Theme.CSS
	if !strings.Contains(css, ".prism-axis-x{--prism-grid-color:#ff0000;}") {
		t.Errorf("axis_x.grid_color did not reach the scoped CSS:\n%s", css)
	}
	if !strings.Contains(css, "--prism-grid-color:#cccccc;") {
		t.Error("the shared axis.grid_color left the :root block")
	}

	// Geometry cannot ride a CSS variable, so it rides the Scene IR.
	if doc.Theme.AxisY == nil || doc.Theme.AxisY.TickSize == nil || *doc.Theme.AxisY.TickSize != 12 {
		t.Errorf("axis_y.tick_size did not reach the Scene IR: %+v", doc.Theme.AxisY)
	}
	if doc.Theme.AxisX != nil {
		t.Errorf("a colour-only axis_x added Scene IR bytes: %+v", doc.Theme.AxisX)
	}
}
