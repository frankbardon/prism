package encode_test

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// The tests in this file pin the v0.18 rendering upgrades chart
// builders lean on: spec-level frame size, channel title / sort,
// tooltip titles, color domain order and nested / sized concat.

const twoBars = `"data": {"values": [{"k": "a", "v": 1, "s": "no"}, {"k": "b", "v": 3, "s": "yes"}, {"k": "c", "v": 2, "s": "no"}]}`

func TestSpecFrameSize(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema": "urn:prism:schema:v1:spec", "width": 320, "height": 180, `+twoBars+`,
		"mark": "bar", "encoding": {"x": {"field": "k", "type": "nominal"}, "y": {"field": "v", "type": "quantitative"}}}`)
	f := doc.Grid.Cells[0].Scene.Frame
	if f.W != 320 || f.H != 180 {
		t.Fatalf("frame %vx%v, want 320x180", f.W, f.H)
	}
}

func TestChannelTitleAndSort(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema": "urn:prism:schema:v1:spec", `+twoBars+`, "mark": "bar", "encoding": {
		"x": {"field": "k", "type": "nominal", "title": "Key", "sort": "descending"},
		"y": {"field": "v", "type": "quantitative", "title": "Value", "axis": {"title": "Axis wins"}},
		"color": {"field": "s", "type": "nominal", "title": "State", "scale": {"domain": ["yes", "no"]}}}}`)
	x := axisOnChannel(t, doc, scene.ChannelX)
	if x.Title != "Key" {
		t.Errorf("x title %q, want the channel title", x.Title)
	}
	if got := tickLabels(x); strings.Join(got, ",") != "c,b,a" {
		t.Errorf("x categories %v, want descending c,b,a", got)
	}
	if y := axisOnChannel(t, doc, scene.ChannelY); y.Title != "Axis wins" {
		t.Errorf("y title %q, want axis.title to outrank the channel title", y.Title)
	}
	sc := doc.Grid.Cells[0].Scene
	if len(sc.Legends) != 1 || sc.Legends[0].Title != "State" {
		t.Fatalf("legends %+v, want one titled State", sc.Legends)
	}
	if e := sc.Legends[0].Entries; len(e) != 2 || e[0].Label != "yes" {
		t.Errorf("legend entries %+v, want the domain order yes, no", e)
	}
	for _, w := range doc.Warnings {
		if w.Code == scene.WarnChannelInert {
			t.Errorf("unexpected inert warning: %s", w.Message)
		}
	}

	list := encodeFlatJSON(t, `{"$schema": "urn:prism:schema:v1:spec", `+twoBars+`, "mark": "bar", "encoding": {
		"x": {"field": "k", "type": "nominal", "sort": ["b", "c"]}, "y": {"field": "v", "type": "quantitative", "sort": "-y"}}}`)
	if got := tickLabels(axisOnChannel(t, list, scene.ChannelX)); strings.Join(got, ",") != "b,c,a" {
		t.Errorf("x categories %v, want the listed b,c then a", got)
	}
	if !hasWarning(list.Warnings, scene.WarnChannelInert, "encoding.y.sort") {
		t.Errorf("sort by channel: want an inert warning, got %+v", list.Warnings)
	}
}

func TestTooltipTitleAndFormat(t *testing.T) {
	doc := encodeFlatJSON(t, `{"$schema": "urn:prism:schema:v1:spec", "data": {"values": [{"k": "a", "v": 1.5}, {"k": "b", "v": null}]},
		"mark": "point", "encoding": {"x": {"field": "k", "type": "nominal"}, "y": {"field": "k", "type": "nominal"},
		"tooltip": [{"field": "k", "title": "Key"}, {"field": "v", "title": "Value", "format": ".2~f"}]}}`)
	var lines []string
	for _, l := range doc.Grid.Cells[0].Scene.Layers {
		for _, m := range l.Marks {
			for _, ln := range m.Tooltip.Lines {
				lines = append(lines, ln.Label)
			}
		}
	}
	want := "Key: a|Value: 1.5|Key: b|Value: –"
	if got := strings.Join(lines, "|"); got != want {
		t.Errorf("tooltip lines %q, want %q", got, want)
	}
}

func TestLayerSharedTitleAndColorDomain(t *testing.T) {
	doc := encodeCompositeJSON(t, `{"$schema": "urn:prism:schema:v1:spec", `+twoBars+`, "layer": [
		{"$schema": "urn:prism:schema:v1:spec", "mark": "rule", "encoding": {"x": {"field": "v", "type": "quantitative", "title": "Effect"}, "y": {"field": "k", "type": "nominal"}}},
		{"$schema": "urn:prism:schema:v1:spec", "mark": "point", "encoding": {"x": {"field": "v", "type": "quantitative"}, "y": {"field": "k", "type": "nominal"},
			"color": {"field": "s", "type": "nominal", "title": "Significance", "scale": {"domain": ["yes", "no"]}}}}]}`)
	if doc.Grid.Shared.X == nil || doc.Grid.Shared.X.Title != "Effect" {
		t.Fatalf("shared x %+v, want title Effect", doc.Grid.Shared.X)
	}
	lg := doc.Grid.Cells[0].Scene.Legends
	if len(lg) != 1 || lg[0].Title != "Significance" || lg[0].Entries[0].Label != "yes" {
		t.Fatalf("legend %+v, want Significance with yes first", lg)
	}
}

func TestNestedSizedConcat(t *testing.T) {
	leaf := `"mark": "bar", "encoding": {"x": {"field": "k", "type": "nominal"}, "y": {"field": "v", "type": "quantitative"}}`
	doc := encodeCompositeJSON(t, `{"$schema": "urn:prism:schema:v1:spec", "width": 400, `+twoBars+`, "vconcat": [
		{"$schema": "urn:prism:schema:v1:spec", "height": 100, "title": "A", "layer": [
			{"$schema": "urn:prism:schema:v1:spec", `+leaf+`},
			{"$schema": "urn:prism:schema:v1:spec", "mark": "point", "encoding": {"x": {"field": "k", "type": "nominal"}, "y": {"field": "v", "type": "quantitative"}}}]},
		{"$schema": "urn:prism:schema:v1:spec", "height": 200, `+leaf+`}]}`)
	cells := doc.Grid.Cells
	if len(cells) != 2 {
		t.Fatalf("cells %d, want 2", len(cells))
	}
	a, b := cells[0].Scene.Frame, cells[1].Scene.Frame
	if a.H != 100 || b.H != 200 || b.Y != 120 || a.W != 400 {
		t.Errorf("frames %+v / %+v, want 400x100 at 0 and 400x200 at y=120", a, b)
	}
	if len(cells[0].Scene.Layers) != 2 {
		t.Errorf("nested layer: %d layers, want 2", len(cells[0].Scene.Layers))
	}
	if len(cells[0].Scene.Axes) != 2 || doc.Grid.Shared.X != nil {
		t.Errorf("nested layer axes: cell %d, grid shared %v; want both folded into the cell", len(cells[0].Scene.Axes), doc.Grid.Shared.X)
	}
	if cells[0].Scene.ID == cells[1].Scene.ID {
		t.Errorf("scene ids collide: %s", cells[0].Scene.ID)
	}
}

func TestConcatColumnsWrap(t *testing.T) {
	leaf := `{"$schema": "urn:prism:schema:v1:spec", "mark": "bar", "encoding": {"x": {"field": "k", "type": "nominal"}, "y": {"field": "v", "type": "quantitative"}}}`
	doc := encodeCompositeJSON(t, `{"$schema": "urn:prism:schema:v1:spec", "columns": 2, `+twoBars+`, "concat": [`+leaf+`,`+leaf+`,`+leaf+`]}`)
	c := doc.Grid.Cells
	if len(c) != 3 || c[2].Row != 1 || c[2].Col != 0 || c[2].Scene.Frame.X != 0 || c[1].Scene.Frame.Y != 0 {
		t.Fatalf("cells %+v, want a 2-wide grid with the third cell first on row 1", c)
	}
	if doc.Grid.Layout.Rows != 2 || doc.Grid.Layout.Cols != 2 {
		t.Errorf("layout %+v, want 2x2", doc.Grid.Layout)
	}
}

func tickLabels(ax scene.Axis) []string {
	var out []string
	for _, tk := range ax.Ticks {
		out = append(out, tk.Label)
	}
	return out
}

func hasWarning(ws []scene.Warning, code, pathPart string) bool {
	for _, w := range ws {
		if w.Code == code && strings.Contains(w.Message, pathPart) {
			return true
		}
	}
	return false
}
