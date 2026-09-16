package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// colorRangeSpec paints three categories with an inline scale.range
// that overlaps nothing in the default palette, so every assertion
// below fails loudly if the range is ignored.
const colorRangeSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"k": "a", "v": 3},
    {"k": "b", "v": 5},
    {"k": "c", "v": 7}
  ]},
  "mark": {"type": "bar"},
  "encoding": {
    "x": {"field": "k", "type": "nominal"},
    "y": {"field": "v", "type": "quantitative"},
    "color": {
      "field": "k", "type": "nominal",
      "scale": {"scheme": "viridis", "range": ["#ff0000", "#00ff00", "#0000ff"]}
    }
  }
}`

func TestPrismEncodeColorRangeDrivesMarksAndLegend(t *testing.T) {
	doc, err := encodeInlineDoc(t, colorRangeSpec)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sceneObj := doc.Grid.Cells[0].Scene

	want := []string{"#ff0000", "#00ff00", "#0000ff"}
	marks := sceneObj.Layers[0].Marks
	if len(marks) != len(want) {
		t.Fatalf("marks = %d, want %d", len(marks), len(want))
	}
	for i, m := range marks {
		if m.Style.Fill == nil {
			t.Fatalf("mark %d has no fill", i)
		}
		if got := m.Style.Fill.Hex(); got != want[i] {
			t.Errorf("mark %d fill = %s, want %s", i, got, want[i])
		}
	}

	if len(sceneObj.Legends) != 1 {
		t.Fatalf("legends = %d, want 1", len(sceneObj.Legends))
	}
	entries := sceneObj.Legends[0].Entries
	if len(entries) != len(want) {
		t.Fatalf("legend entries = %d, want %d", len(entries), len(want))
	}
	for i, e := range entries {
		if e.Swatch.Type != scene.SwatchSolid || e.Swatch.Color == nil {
			t.Fatalf("legend entry %d has no solid swatch", i)
		}
		if got := e.Swatch.Color.Hex(); got != want[i] {
			t.Errorf("legend swatch %d = %s, want %s", i, got, want[i])
		}
	}
}

// heatmapRangeSpec drives the continuous path: the heatmap encoder
// interpolates within the resolved ramp, so a two-stop inline range
// plus an interpolation space is observable in the cell fills.
func heatmapRangeSpec(interpolate string) string {
	block := `"range": ["#000000", "#ffffff"]`
	if interpolate != "" {
		block += `, "interpolate": "` + interpolate + `"`
	}
	return `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"r": "r0", "c": "c0", "v": 0},
    {"r": "r0", "c": "c1", "v": 50},
    {"r": "r0", "c": "c2", "v": 100}
  ]},
  "mark": {"type": "heatmap"},
  "encoding": {
    "x": {"field": "c", "type": "nominal"},
    "y": {"field": "r", "type": "nominal"},
    "color": {"field": "v", "type": "quantitative", "scale": {` + block + `}}
  }
}`
}

func heatmapFills(t *testing.T, body string) []string {
	t.Helper()
	doc, err := encodeInlineDoc(t, body)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	marks := doc.Grid.Cells[0].Scene.Layers[0].Marks
	out := make([]string, 0, len(marks))
	for _, m := range marks {
		if m.Style.Fill == nil {
			continue
		}
		out = append(out, m.Style.Fill.Hex())
	}
	return out
}

func TestPrismEncodeHeatmapHonoursColorRange(t *testing.T) {
	got := heatmapFills(t, heatmapRangeSpec(""))
	if len(got) != 3 {
		t.Fatalf("cell fills = %v, want 3", got)
	}
	if got[0] != "#000000" || got[2] != "#ffffff" {
		t.Fatalf("ramp endpoints = %s..%s, want #000000..#ffffff", got[0], got[2])
	}
	// Default rgb space: the midpoint is the component average, as
	// the heatmap encoder's own truncating lerp computes it.
	if got[1] != "#7f7f7f" {
		t.Errorf("rgb midpoint cell = %s, want #7f7f7f", got[1])
	}
}

func TestPrismEncodeHeatmapHonoursInterpolateLab(t *testing.T) {
	got := heatmapFills(t, heatmapRangeSpec("lab"))
	if len(got) != 3 {
		t.Fatalf("cell fills = %v, want 3", got)
	}
	if got[0] != "#000000" || got[2] != "#ffffff" {
		t.Fatalf("ramp endpoints = %s..%s, want #000000..#ffffff", got[0], got[2])
	}
	// The ramp was resampled in CIELAB before it reached the encoder,
	// so the midpoint cell lands on L* = 50 even though the encoder
	// itself still lerps in sRGB — that is the whole point of
	// resampling at encode time.
	if got[1] != "#777777" {
		t.Errorf("lab midpoint cell = %s, want #777777", got[1])
	}
}
