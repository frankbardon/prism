package encode_test

import (
	"context"
	"strings"
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

// encodeCompositeJSON drives an inline composite spec through
// Build → Execute → EncodeComposite. Used by the E3-S5 shared-axis
// tests, which need many small spec variants rather than fixtures.
func encodeCompositeJSON(t *testing.T, body string) *scene.SceneDoc {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewMemMapFs(),
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
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	return doc
}

// encodeFlatJSON drives an inline flat spec through
// Build → Execute → Encode.
func encodeFlatJSON(t *testing.T, body string) *scene.SceneDoc {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return doc
}

// axisOnChannel returns the encoded axis for a channel on a flat
// chart's single scene.
func axisOnChannel(t *testing.T, doc *scene.SceneDoc, ch scene.Channel) scene.Axis {
	t.Helper()
	if len(doc.Grid.Cells) == 0 {
		t.Fatal("scene doc has no cells")
	}
	for _, ax := range doc.Grid.Cells[0].Scene.Axes {
		if ax.Channel == ch {
			return ax
		}
	}
	t.Fatalf("no %s axis in scene", ch)
	return scene.Axis{}
}

// histogramSpec renders a histogram whose y channel carries the
// supplied axis block.
func histogramSpec(yAxis string) string {
	return `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"score": 0.1}, {"score": 0.2}, {"score": 0.25}, {"score": 0.4},
    {"score": 0.45}, {"score": 0.6}, {"score": 0.62}, {"score": 0.75}
  ]},
  "mark": "histogram",
  "encoding": {
    "x": {"field": "score", "type": "quantitative", "bin": true},
    "y": {"aggregate": "count", "type": "quantitative"` + yAxis + `}
  }
}`
}

func TestPrismHistogramYAxisHonoursAxisConfig(t *testing.T) {
	doc := encodeFlatJSON(t, histogramSpec(`, "axis": {"grid": false, "title": "Frequency"}`))
	y := axisOnChannel(t, doc, scene.ChannelY)
	if len(y.Grid) != 0 {
		t.Errorf("histogram y grid lines = %d, want 0 (axis.grid=false)", len(y.Grid))
	}
	if y.Title != "Frequency" {
		t.Errorf("histogram y title = %q, want %q", y.Title, "Frequency")
	}
	// The x axis already honoured config; assert parity is preserved.
	x := axisOnChannel(t, doc, scene.ChannelX)
	if len(x.Grid) == 0 {
		t.Error("histogram x grid lines = 0, want the default grid")
	}
}

func TestPrismHistogramYAxisKeepsCountTitleFallback(t *testing.T) {
	doc := encodeFlatJSON(t, histogramSpec(``))
	if got := axisOnChannel(t, doc, scene.ChannelY).Title; got != "count" {
		t.Errorf("histogram y title = %q, want %q", got, "count")
	}
	// An explicit suppression must not be overwritten by the fallback.
	doc = encodeFlatJSON(t, histogramSpec(`, "axis": {"title": false}`))
	if got := axisOnChannel(t, doc, scene.ChannelY).Title; got != "" {
		t.Errorf("histogram y title = %q, want suppressed", got)
	}
}

// layerSpec renders a two-layer spec whose children carry the supplied
// axis blocks on x and y. Both layers plot the same inline rows so the
// default (shared) resolve mode produces shared x and y scales.
func layerSpec(xAxisA, yAxisA, xAxisB, yAxisB string) string {
	child := func(xAxis, yAxis string) string {
		return `{
      "$schema": "urn:prism:schema:v1:spec",
      "data": {"values": [{"day": "a", "score": 1}, {"day": "b", "score": 4}]},
      "mark": {"type": "line"},
      "encoding": {
        "x": {"field": "day", "type": "nominal"` + xAxis + `},
        "y": {"field": "score", "type": "quantitative"` + yAxis + `}
      }
    }`
	}
	return `{
  "$schema": "urn:prism:schema:v1:spec",
  "layer": [` + child(xAxisA, yAxisA) + `, ` + child(xAxisB, yAxisB) + `]
}`
}

func TestPrismSharedAxisHonoursChildAxisConfig(t *testing.T) {
	// Default resolve mode is shared for both channels.
	doc := encodeCompositeJSON(t, layerSpec(
		`, "axis": {"grid": false, "title": "Day"}`,
		`, "axis": {"grid": false}`,
		`, "axis": {"grid": false, "title": "Day"}`,
		`, "axis": {"grid": false}`,
	))
	x := doc.Grid.Shared.X
	y := doc.Grid.Shared.Y
	if x == nil || y == nil {
		t.Fatalf("shared axes missing: x=%v y=%v", x, y)
	}
	if len(x.Grid) != 0 {
		t.Errorf("shared x grid lines = %d, want 0 (axis.grid=false)", len(x.Grid))
	}
	if len(y.Grid) != 0 {
		t.Errorf("shared y grid lines = %d, want 0 (axis.grid=false)", len(y.Grid))
	}
	if x.Title != "Day" {
		t.Errorf("shared x title = %q, want %q", x.Title, "Day")
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("unexpected warnings on agreeing children: %+v", doc.Warnings)
	}
}

func TestPrismSharedAxisMatchesFlattenedSpec(t *testing.T) {
	// The reported symptom: {"grid": false, "title": ""} on layer
	// children left grid lines and a title under the default resolve
	// mode while the same spec flattened produced neither.
	doc := encodeCompositeJSON(t, layerSpec(
		`, "axis": {"grid": false, "title": false}`,
		`, "axis": {"grid": false, "title": false}`,
		`, "axis": {"grid": false, "title": false}`,
		`, "axis": {"grid": false, "title": false}`,
	))
	for _, tc := range []struct {
		name string
		ax   *scene.Axis
	}{{"x", doc.Grid.Shared.X}, {"y", doc.Grid.Shared.Y}} {
		if tc.ax == nil {
			t.Fatalf("shared %s axis missing", tc.name)
		}
		if len(tc.ax.Grid) != 0 {
			t.Errorf("shared %s grid lines = %d, want 0", tc.name, len(tc.ax.Grid))
		}
		if tc.ax.Title != "" {
			t.Errorf("shared %s title = %q, want suppressed", tc.name, tc.ax.Title)
		}
	}
}

func TestPrismSharedAxisFirstSpecifiedWins(t *testing.T) {
	// Layer 0 leaves x alone; layer 1 sets grid=false. The property is
	// unspecified until layer 1, so layer 1 wins with no conflict.
	doc := encodeCompositeJSON(t, layerSpec(
		``, ``,
		`, "axis": {"grid": false}`, ``,
	))
	if doc.Grid.Shared.X == nil {
		t.Fatal("shared x axis missing")
	}
	if len(doc.Grid.Shared.X.Grid) != 0 {
		t.Errorf("shared x grid lines = %d, want 0 (only layer-1 specified)", len(doc.Grid.Shared.X.Grid))
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("unexpected warnings when only one child specifies: %+v", doc.Warnings)
	}
}

func TestPrismSharedAxisConflictWarns(t *testing.T) {
	// Both children specify grid, and they disagree. First wins, and the
	// loser is reported rather than silently dropped.
	doc := encodeCompositeJSON(t, layerSpec(
		`, "axis": {"grid": true}`, ``,
		`, "axis": {"grid": false}`, ``,
	))
	if doc.Grid.Shared.X == nil {
		t.Fatal("shared x axis missing")
	}
	if len(doc.Grid.Shared.X.Grid) == 0 {
		t.Error("shared x grid lines = 0, want layer-0's grid=true to win")
	}
	var found *scene.Warning
	for i := range doc.Warnings {
		if doc.Warnings[i].Code == scene.WarnAxisConfigConflict {
			found = &doc.Warnings[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no %s warning; warnings=%+v", scene.WarnAxisConfigConflict, doc.Warnings)
	}
	if got := found.Details["Property"]; got != "grid" {
		t.Errorf("conflict property = %v, want %q", got, "grid")
	}
	if got := found.Details["Winner"]; got != "layer-0" {
		t.Errorf("conflict winner = %v, want %q", got, "layer-0")
	}
	if got := found.Details["Loser"]; got != "layer-1" {
		t.Errorf("conflict loser = %v, want %q", got, "layer-1")
	}
	if !strings.Contains(found.Message, "first specified wins") {
		t.Errorf("conflict message does not state the rule: %q", found.Message)
	}
}

func TestPrismSharedAxisTitleFallsBackToField(t *testing.T) {
	// No child sets axis.title, so the shared axis keeps the historical
	// field-name title.
	doc := encodeCompositeJSON(t, layerSpec(`, "axis": {"grid": false}`, ``, ``, ``))
	if doc.Grid.Shared.X == nil {
		t.Fatal("shared x axis missing")
	}
	if doc.Grid.Shared.X.Title != "day" {
		t.Errorf("shared x title = %q, want %q", doc.Grid.Shared.X.Title, "day")
	}
}

func TestPrismFacetSharedAxisHonoursChildAxisConfig(t *testing.T) {
	body := `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"region": "north", "day": "a", "score": 1},
    {"region": "north", "day": "b", "score": 4},
    {"region": "south", "day": "a", "score": 2},
    {"region": "south", "day": "b", "score": 3}
  ]},
  "facet": {"column": {"field": "region", "type": "nominal"}},
  "spec": {
    "$schema": "urn:prism:schema:v1:spec",
    "mark": {"type": "line"},
    "encoding": {
      "x": {"field": "day", "type": "nominal", "axis": {"grid": false, "title": "Day"}},
      "y": {"field": "score", "type": "quantitative", "axis": {"grid": false, "title": false}}
    }
  }
}`
	doc := encodeCompositeJSON(t, body)
	x := doc.Grid.Shared.X
	y := doc.Grid.Shared.Y
	if x == nil || y == nil {
		t.Fatalf("facet shared axes missing: x=%v y=%v", x, y)
	}
	if len(x.Grid) != 0 {
		t.Errorf("facet shared x grid lines = %d, want 0", len(x.Grid))
	}
	if len(y.Grid) != 0 {
		t.Errorf("facet shared y grid lines = %d, want 0", len(y.Grid))
	}
	if x.Title != "Day" {
		t.Errorf("facet shared x title = %q, want %q", x.Title, "Day")
	}
	if y.Title != "" {
		t.Errorf("facet shared y title = %q, want suppressed", y.Title)
	}
}
