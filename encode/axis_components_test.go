package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// E3-S2 — the seven axis fields governing what an axis draws, how big
// it is, and where it sits in the stack: labels / ticks / domain,
// tick_size / label_padding / label_limit, and zindex.

// componentSpec renders a two-point bar chart whose x channel carries
// the supplied axis block.
func componentSpec(xAxis string) string {
	return `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"day": "alpha", "score": 1}, {"day": "omega", "score": 4}]},
  "mark": {"type": "bar"},
  "encoding": {
    "x": {"field": "day", "type": "nominal"` + xAxis + `},
    "y": {"field": "score", "type": "quantitative"}
  }
}`
}

func TestPrismAxisComponentsDefaultToVisible(t *testing.T) {
	x := axisOnChannel(t, encodeFlatJSON(t, componentSpec(``)), scene.ChannelX)
	if x.HideLabels || x.HideTicks || x.HideDomain {
		t.Errorf("default axis hides components: labels=%v ticks=%v domain=%v",
			x.HideLabels, x.HideTicks, x.HideDomain)
	}
	if x.TickSize != nil || x.LabelPadding != nil {
		t.Errorf("default axis states geometry: tick_size=%v label_padding=%v",
			x.TickSize, x.LabelPadding)
	}
	if x.Zindex != 0 {
		t.Errorf("default axis zindex = %d, want 0", x.Zindex)
	}
}

// The three visibility switches must compose independently: each one
// suppresses its own component and leaves the other two alone.
func TestPrismAxisComponentsSuppressIndependently(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		block                     string
		labels, ticks, domainGone bool
	}{
		{"labels", `, "axis": {"labels": false}`, true, false, false},
		{"ticks", `, "axis": {"ticks": false}`, false, true, false},
		{"domain", `, "axis": {"domain": false}`, false, false, true},
		{"all three", `, "axis": {"labels": false, "ticks": false, "domain": false}`, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := axisOnChannel(t, encodeFlatJSON(t, componentSpec(tc.block)), scene.ChannelX)
			if x.HideLabels != tc.labels {
				t.Errorf("HideLabels = %v, want %v", x.HideLabels, tc.labels)
			}
			if x.HideTicks != tc.ticks {
				t.Errorf("HideTicks = %v, want %v", x.HideTicks, tc.ticks)
			}
			if x.HideDomain != tc.domainGone {
				t.Errorf("HideDomain = %v, want %v", x.HideDomain, tc.domainGone)
			}
			// Suppressing a component never drops the ticks themselves:
			// the labels and the grid lines are derived from the same
			// list.
			if len(x.Ticks) == 0 {
				t.Error("ticks list emptied by component suppression")
			}
		})
	}
}

// A suppressed component hands its share of the side reservation back
// to the plot rect (E1-S1's layout).
func TestPrismAxisComponentSuppressionReleasesPadding(t *testing.T) {
	plotOf := func(t *testing.T, block string) scene.Rect {
		t.Helper()
		doc := encodeFlatJSON(t, componentSpec(block))
		return doc.Grid.Cells[0].Scene.Plot
	}
	base := plotOf(t, ``)
	noLabels := plotOf(t, `, "axis": {"labels": false}`)
	noTicks := plotOf(t, `, "axis": {"ticks": false}`)
	bare := plotOf(t, `, "axis": {"labels": false, "ticks": false}`)

	if !(noLabels.H > base.H) {
		t.Errorf("labels:false plot height = %v, want more than the default %v", noLabels.H, base.H)
	}
	if !(noTicks.H > base.H) {
		t.Errorf("ticks:false plot height = %v, want more than the default %v", noTicks.H, base.H)
	}
	if !(bare.H > noLabels.H) {
		t.Errorf("both-suppressed plot height = %v, want more than labels-only %v", bare.H, noLabels.H)
	}
	// Suppressing the x axis's components must not move the y side.
	if bare.X != base.X || bare.W != base.W {
		t.Errorf("x-axis suppression moved the y side: X %v→%v, W %v→%v",
			base.X, bare.X, base.W, bare.W)
	}
	// The domain line sits on the plot edge and reserves nothing, so
	// hiding it must not resize anything.
	if noDomain := plotOf(t, `, "axis": {"domain": false}`); noDomain != base {
		t.Errorf("domain:false plot = %+v, want the default %+v", noDomain, base)
	}
}

// E1-S4's `"axis": null` is whole-axis suppression; the component
// switches are finer-grained. The two compose: turning every component
// off reserves exactly what null reserves (nothing), but the axis is
// still emitted — its grid lines and title survive, which is the whole
// point of the finer-grained switches.
func TestPrismAxisNullVersusComponentSuppression(t *testing.T) {
	cellPlot := func(t *testing.T, block string) scene.Rect {
		t.Helper()
		return encodeFlatJSON(t, componentSpec(block)).Grid.Cells[0].Scene.Plot
	}
	bare := cellPlot(t, `, "axis": {"labels": false, "ticks": false}`)
	hidden := cellPlot(t, `, "axis": null`)
	if bare != hidden {
		t.Errorf("components-all-off plot = %+v, axis:null plot = %+v; "+
			"with every component off the two should reserve the same space",
			bare, hidden)
	}
	// They agree on padding but not on structure: null emits no axis
	// at all, component suppression still emits one carrying its grid
	// lines and title.
	for _, ax := range encodeFlatJSON(t, componentSpec(`, "axis": null`)).Grid.Cells[0].Scene.Axes {
		if ax.Channel == scene.ChannelX {
			t.Fatal(`"axis": null still emitted an x axis`)
		}
	}
	x := axisOnChannel(t, encodeFlatJSON(t,
		componentSpec(`, "axis": {"labels": false, "ticks": false, "domain": false}`)), scene.ChannelX)
	if len(x.Grid) == 0 {
		t.Error("component suppression also dropped the grid lines; only `axis: null` should")
	}
	if x.Title == "" {
		t.Error("component suppression also dropped the axis title; only `axis: null` should")
	}
}

func TestPrismAxisTickSizeAndLabelPaddingReachTheIR(t *testing.T) {
	x := axisOnChannel(t, encodeFlatJSON(t,
		componentSpec(`, "axis": {"tick_size": 12, "label_padding": 9}`)), scene.ChannelX)
	if x.TickSize == nil || *x.TickSize != 12 {
		t.Errorf("TickSize = %v, want 12", x.TickSize)
	}
	if x.LabelPadding == nil || *x.LabelPadding != 9 {
		t.Errorf("LabelPadding = %v, want 9", x.LabelPadding)
	}
}

func TestPrismAxisLabelLimitTruncatesWithEllipsis(t *testing.T) {
	// "alpha" and "omega" are 5 characters ⇒ 30 px at the 6 px/char
	// heuristic. A 24 px limit leaves room for 3 characters + ellipsis.
	x := axisOnChannel(t, encodeFlatJSON(t,
		componentSpec(`, "axis": {"label_limit": 24}`)), scene.ChannelX)
	want := map[string]string{"alpha": "alp…", "omega": "ome…"}
	seen := 0
	for _, tick := range x.Ticks {
		v, ok := tick.Value.(string)
		if !ok {
			continue
		}
		if got := want[v]; got != "" {
			seen++
			if tick.Label != got {
				t.Errorf("label for %q = %q, want %q", v, tick.Label, got)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("matched %d ticks, want 2", seen)
	}
}

func TestPrismAxisLabelLimitLeavesShortLabelsAlone(t *testing.T) {
	x := axisOnChannel(t, encodeFlatJSON(t,
		componentSpec(`, "axis": {"label_limit": 200}`)), scene.ChannelX)
	for _, tick := range x.Ticks {
		if tick.Label != "" && tick.Label != tick.Value {
			t.Errorf("label %q truncated under a 200px limit", tick.Label)
		}
	}
}

func TestPrismAxisZindexReachesTheIR(t *testing.T) {
	x := axisOnChannel(t, encodeFlatJSON(t,
		componentSpec(`, "axis": {"zindex": 1}`)), scene.ChannelX)
	if x.Zindex != 1 {
		t.Errorf("Zindex = %d, want 1", x.Zindex)
	}
}

// E3-S5 rebuilt the shared-axis path so it resolves per-channel config
// reflectively. These two cases are the acceptance E3-S5's followup
// asked for: the seven fields must work under the DEFAULT (shared)
// resolve mode for both layer and facet, not just on a flat chart.
func TestPrismAxisComponentsSurviveLayerComposition(t *testing.T) {
	doc := encodeCompositeJSON(t, layerSpec(
		`, "axis": {"labels": false, "ticks": false, "tick_size": 11, "zindex": 1}`,
		``, ``, ``))
	x := doc.Grid.Shared.X
	if x == nil {
		t.Fatal("shared x axis missing")
	}
	if !x.HideLabels || !x.HideTicks {
		t.Errorf("shared x: HideLabels=%v HideTicks=%v, want both true", x.HideLabels, x.HideTicks)
	}
	if x.HideDomain {
		t.Error("shared x hid the domain line, which no layer asked for")
	}
	if x.TickSize == nil || *x.TickSize != 11 {
		t.Errorf("shared x TickSize = %v, want 11", x.TickSize)
	}
	if x.Zindex != 1 {
		t.Errorf("shared x Zindex = %d, want 1", x.Zindex)
	}
}

func TestPrismAxisComponentsSurviveFacetComposition(t *testing.T) {
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
      "x": {"field": "day", "type": "nominal", "axis": {"labels": false, "label_padding": 7}},
      "y": {"field": "score", "type": "quantitative", "axis": {"domain": false}}
    }
  }
}`
	doc := encodeCompositeJSON(t, body)
	x, y := doc.Grid.Shared.X, doc.Grid.Shared.Y
	if x == nil || y == nil {
		t.Fatalf("facet shared axes missing: x=%v y=%v", x, y)
	}
	if !x.HideLabels {
		t.Error("facet shared x did not hide its labels")
	}
	if x.LabelPadding == nil || *x.LabelPadding != 7 {
		t.Errorf("facet shared x LabelPadding = %v, want 7", x.LabelPadding)
	}
	if !y.HideDomain {
		t.Error("facet shared y did not hide its domain line")
	}
	if y.HideLabels || y.HideTicks {
		t.Error("facet shared y suppressed components the spec left alone")
	}
}
