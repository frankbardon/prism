package encode_test

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// E3-S1: `axis.tick_count`, `axis.values` and `axis.tick_min_step`
// choose the tick set — and, because grid lines are emitted at exactly
// the major tick positions, the gridline set with it. Every assertion
// below checks the coupling as well as the ticks.

// majorTickValues returns the numeric value of every major (non-minor)
// tick on an axis, in emitted order.
func majorTickValues(t *testing.T, ax scene.Axis) []float64 {
	t.Helper()
	var out []float64
	for _, tk := range ax.Ticks {
		if tk.Minor {
			continue
		}
		v, ok := tk.Value.(float64)
		if !ok {
			t.Fatalf("major tick %v is %T, want float64", tk.Value, tk.Value)
		}
		out = append(out, v)
	}
	return out
}

// majorTickLabels returns the label of every major tick.
func majorTickLabels(ax scene.Axis) []string {
	var out []string
	for _, tk := range ax.Ticks {
		if tk.Minor {
			continue
		}
		out = append(out, tk.Label)
	}
	return out
}

// wantGridFollowsMajors is the coupling assertion: one grid line per
// major tick, never one per minor.
func wantGridFollowsMajors(t *testing.T, ax scene.Axis) {
	t.Helper()
	majors := 0
	for _, tk := range ax.Ticks {
		if !tk.Minor {
			majors++
		}
	}
	if len(ax.Grid) != majors {
		t.Errorf("grid lines = %d, want %d (one per major tick)", len(ax.Grid), majors)
	}
}

func hasMinorTick(ax scene.Axis) bool {
	for _, tk := range ax.Ticks {
		if tk.Minor {
			return true
		}
	}
	return false
}

// tickAxisSpec renders three rows on a quantitative y whose axis block
// is supplied by the caller. The default domain is [0, 120] (zero
// forced in, nice rounding applied) so the tick arithmetic is stable.
func tickAxisSpec(yAxis string) string {
	return `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"k": "a", "v": 100}, {"k": "b", "v": 110}, {"k": "c", "v": 120}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "k", "type": "nominal"},
    "y": {"field": "v", "type": "quantitative"` + yAxis + `}
  }
}`
}

// TestPrismAxisDefaultTickCountUnchanged pins the pre-E3-S1 rendering:
// with no tick_count the generator still asks for DefaultTickCount.
func TestPrismAxisDefaultTickCountUnchanged(t *testing.T) {
	y := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec("")), scene.ChannelY)
	want := []float64{0, 20, 40, 60, 80, 100, 120}
	got := majorTickValues(t, y)
	if len(got) != len(want) {
		t.Fatalf("major ticks = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("major ticks = %v, want %v", got, want)
		}
	}
	if !hasMinorTick(y) {
		t.Error("linear axis lost its minor ticks")
	}
	wantGridFollowsMajors(t, y)
}

// TestPrismAxisTickCountReplacesHardcodedFive is the headline case:
// the count the author asks for reaches NiceTicks instead of the
// literal 5, and the gridlines follow.
func TestPrismAxisTickCountReplacesHardcodedFive(t *testing.T) {
	y := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec(`, "axis": {"tick_count": 3}`)), scene.ChannelY)
	got := majorTickValues(t, y)
	if len(got) != 3 || got[0] != 0 || got[1] != 50 || got[2] != 100 {
		t.Errorf("major ticks = %v, want [0 50 100] for tick_count=3", got)
	}
	wantGridFollowsMajors(t, y)

	dense := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec(`, "axis": {"tick_count": 12}`)), scene.ChannelY)
	if len(majorTickValues(t, dense)) <= len(got) {
		t.Errorf("tick_count=12 produced %d majors, want more than tick_count=3's %d",
			len(majorTickValues(t, dense)), len(got))
	}
	wantGridFollowsMajors(t, dense)
}

// TestPrismAxisTickCountZeroRemovesTicksAndGrid: 0 is a real request,
// not "unset".
func TestPrismAxisTickCountZeroRemovesTicksAndGrid(t *testing.T) {
	y := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec(`, "axis": {"tick_count": 0}`)), scene.ChannelY)
	if len(y.Ticks) != 0 {
		t.Errorf("ticks = %d, want none for tick_count=0", len(y.Ticks))
	}
	if len(y.Grid) != 0 {
		t.Errorf("grid lines = %d, want none for tick_count=0", len(y.Grid))
	}
	if y.Domain == (scene.Line{}) {
		t.Error("tick_count=0 also dropped the axis domain line; it should only drop ticks")
	}
}

// TestPrismAxisValuesPinTickSet is the direct answer to "which
// gridlines show".
func TestPrismAxisValuesPinTickSet(t *testing.T) {
	y := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec(`, "axis": {"values": [0, 60, 120]}`)), scene.ChannelY)
	got := majorTickValues(t, y)
	if len(got) != 3 || got[0] != 0 || got[1] != 60 || got[2] != 120 {
		t.Errorf("major ticks = %v, want [0 60 120]", got)
	}
	if hasMinorTick(y) {
		t.Error("pinned tick set still injected minor ticks; pinning must suppress them")
	}
	if len(y.Grid) != 3 {
		t.Errorf("grid lines = %d, want 3 — one per pinned tick", len(y.Grid))
	}
}

// TestPrismAxisValuesOverrideTickCount: values wins outright.
func TestPrismAxisValuesOverrideTickCount(t *testing.T) {
	y := axisOnChannel(t,
		encodeFlatJSON(t, tickAxisSpec(`, "axis": {"tick_count": 9, "tick_min_step": 100, "values": [20, 40]}`)),
		scene.ChannelY)
	got := majorTickValues(t, y)
	if len(got) != 2 || got[0] != 20 || got[1] != 40 {
		t.Errorf("major ticks = %v, want [20 40]; values must beat tick_count and tick_min_step", got)
	}
}

// TestPrismAxisValuesOutsideDomainDropWithWarning: an off-plot tick is
// dropped and reported, never drawn.
func TestPrismAxisValuesOutsideDomainDropWithWarning(t *testing.T) {
	doc, err := encodeInlineDoc(t, tickAxisSpec(`, "axis": {"values": [0, 60, 500, "nope"]}`))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	y := axisOnChannel(t, doc, scene.ChannelY)
	got := majorTickValues(t, y)
	if len(got) != 2 || got[0] != 0 || got[1] != 60 {
		t.Errorf("major ticks = %v, want [0 60] — 500 is off-domain and \"nope\" is unreadable", got)
	}
	if len(y.Grid) != 2 {
		t.Errorf("grid lines = %d, want 2", len(y.Grid))
	}
	var warn *scene.Warning
	for i := range doc.Warnings {
		if doc.Warnings[i].Code == scene.WarnAxisValuesDropped {
			warn = &doc.Warnings[i]
		}
	}
	if warn == nil {
		t.Fatalf("no %s warning; got %+v", scene.WarnAxisValuesDropped, doc.Warnings)
	}
	if warn.Details["Count"] != 2 {
		t.Errorf("warning Count = %v, want 2", warn.Details["Count"])
	}
	if warn.Details["Channel"] != string(scene.ChannelY) {
		t.Errorf("warning Channel = %v, want y", warn.Details["Channel"])
	}
}

// TestPrismAxisValuesAllDroppedLeavesBareAxis: a pin nothing survives
// is a warning, not an error, and leaves no stray gridlines.
func TestPrismAxisValuesAllDroppedLeavesBareAxis(t *testing.T) {
	doc, err := encodeInlineDoc(t, tickAxisSpec(`, "axis": {"values": [900, 1000]}`))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	y := axisOnChannel(t, doc, scene.ChannelY)
	if len(y.Ticks) != 0 || len(y.Grid) != 0 {
		t.Errorf("ticks = %d, grid = %d; want both empty", len(y.Ticks), len(y.Grid))
	}
}

// TestPrismAxisTickMinStepWidensSpacing: the gap constraint is met by
// asking for fewer ticks, so the survivors stay round numbers.
func TestPrismAxisTickMinStepWidensSpacing(t *testing.T) {
	y := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec(`, "axis": {"tick_min_step": 30}`)), scene.ChannelY)
	got := majorTickValues(t, y)
	if len(got) < 2 {
		t.Fatalf("major ticks = %v, want at least 2", got)
	}
	for i := 1; i < len(got); i++ {
		if gap := math.Abs(got[i] - got[i-1]); gap < 30 {
			t.Errorf("major ticks = %v: gap %g violates tick_min_step 30", got, gap)
		}
	}
	// Fewer ticks than the unconstrained default.
	base := axisOnChannel(t, encodeFlatJSON(t, tickAxisSpec("")), scene.ChannelY)
	if len(got) >= len(majorTickValues(t, base)) {
		t.Errorf("tick_min_step did not thin the tick set: %v vs default %v", got, majorTickValues(t, base))
	}
	wantGridFollowsMajors(t, y)
}

// tickTemporalSpec plots a date field so the temporal branch runs.
func tickTemporalSpec(xAxis string) string {
	return `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"d": "2021-01-01", "v": 1}, {"d": "2021-06-01", "v": 4},
    {"d": "2021-12-01", "v": 2}, {"d": "2022-06-01", "v": 6}
  ]},
  "mark": "line",
  "encoding": {
    "x": {"field": "d", "type": "temporal"` + xAxis + `},
    "y": {"field": "v", "type": "quantitative"}
  }
}`
}

// TestPrismAxisTemporalValuesHonourDates: a temporal axis accepts
// date-typed entries and labels them the way the generator would.
func TestPrismAxisTemporalValuesHonourDates(t *testing.T) {
	x := axisOnChannel(t,
		encodeFlatJSON(t, tickTemporalSpec(`, "axis": {"values": ["2021-06-01", "2022-01-01"]}`)),
		scene.ChannelX)
	if len(x.Ticks) != 2 {
		t.Fatalf("temporal ticks = %d (%v), want 2", len(x.Ticks), majorTickLabels(x))
	}
	if len(x.Grid) != 2 {
		t.Errorf("temporal grid lines = %d, want 2", len(x.Grid))
	}
	for _, lbl := range majorTickLabels(x) {
		if lbl == "" {
			t.Errorf("temporal pinned tick has an empty label: %v", majorTickLabels(x))
		}
	}
	// A date outside the domain is dropped like any other.
	doc, err := encodeInlineDoc(t, tickTemporalSpec(`, "axis": {"values": ["2021-06-01", "2030-01-01"]}`))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := len(axisOnChannel(t, doc, scene.ChannelX).Ticks); got != 1 {
		t.Errorf("temporal ticks = %d, want 1 after dropping the off-domain date", got)
	}
}

// TestPrismAxisTemporalTickCount thins the calendar-aligned set.
func TestPrismAxisTemporalTickCount(t *testing.T) {
	few := axisOnChannel(t, encodeFlatJSON(t, tickTemporalSpec(`, "axis": {"tick_count": 2}`)), scene.ChannelX)
	many := axisOnChannel(t, encodeFlatJSON(t, tickTemporalSpec(`, "axis": {"tick_count": 12}`)), scene.ChannelX)
	if len(few.Ticks) > len(many.Ticks) {
		t.Errorf("tick_count=2 produced %d ticks, tick_count=12 produced %d", len(few.Ticks), len(many.Ticks))
	}
	wantGridFollowsMajors(t, few)
}

// TestPrismAxisDiscreteValuesSelectCategories: pinning works on a band
// axis too, and an unknown category warns rather than vanishing
// silently.
func TestPrismAxisDiscreteValuesSelectCategories(t *testing.T) {
	doc, err := encodeInlineDoc(t, tickAxisSpec("")) // sanity: default band axis ticks every category.
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := len(axisOnChannel(t, doc, scene.ChannelX).Ticks); got != 3 {
		t.Fatalf("default band ticks = %d, want 3", got)
	}

	body := `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"k": "a", "v": 1}, {"k": "b", "v": 2}, {"k": "c", "v": 3}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "k", "type": "nominal", "axis": {"values": ["a", "c", "zz"]}},
    "y": {"field": "v", "type": "quantitative"}
  }
}`
	doc, err = encodeInlineDoc(t, body)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	x := axisOnChannel(t, doc, scene.ChannelX)
	if len(x.Ticks) != 2 {
		t.Fatalf("band ticks = %d, want 2 (a and c)", len(x.Ticks))
	}
	if x.Ticks[0].Value != "a" || x.Ticks[1].Value != "c" {
		t.Errorf("band ticks = %v / %v, want a / c", x.Ticks[0].Value, x.Ticks[1].Value)
	}
	if len(x.Grid) != 2 {
		t.Errorf("band grid lines = %d, want 2", len(x.Grid))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Code == scene.WarnAxisValuesDropped {
			found = true
		}
	}
	if !found {
		t.Errorf("no %s warning for the unknown category: %+v", scene.WarnAxisValuesDropped, doc.Warnings)
	}
}

// TestPrismAxisTickCountOnNonLinearFamilies threads the count into the
// log / pow / sqrt generators too.
func TestPrismAxisTickCountOnNonLinearFamilies(t *testing.T) {
	scaleSpec := func(scaleBlock, axisBlock string) string {
		return `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"k": "a", "v": 1}, {"k": "b", "v": 100}, {"k": "c", "v": 10000}, {"k": "d", "v": 1000000}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "k", "type": "nominal"},
    "y": {"field": "v", "type": "quantitative", "scale": ` + scaleBlock + axisBlock + `}
  }
}`
	}
	for _, tc := range []struct{ name, scale string }{
		{"log", `{"type": "log"}`},
		{"pow", `{"type": "pow", "exponent": 2}`},
		{"sqrt", `{"type": "sqrt"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := axisOnChannel(t, encodeFlatJSON(t, scaleSpec(tc.scale, "")), scene.ChannelY)
			few := axisOnChannel(t, encodeFlatJSON(t, scaleSpec(tc.scale, `, "axis": {"tick_count": 2}`)), scene.ChannelY)
			if len(few.Ticks) >= len(base.Ticks) {
				t.Errorf("tick_count=2 produced %d ticks, default produced %d; the count did not reach the generator",
					len(few.Ticks), len(base.Ticks))
			}
			wantGridFollowsMajors(t, few)

			none := axisOnChannel(t, encodeFlatJSON(t, scaleSpec(tc.scale, `, "axis": {"tick_count": 0}`)), scene.ChannelY)
			if len(none.Ticks) != 0 || len(none.Grid) != 0 {
				t.Errorf("tick_count=0: ticks = %d, grid = %d, want none", len(none.Ticks), len(none.Grid))
			}
		})
	}
}

// TestPrismLayerSharedAxisHonoursTickCountAndValues is the E3-S5
// follow-through the story calls for: the new fields must survive the
// reflective shared-axis merge under the DEFAULT (shared) resolve mode.
func TestPrismLayerSharedAxisHonoursTickCountAndValues(t *testing.T) {
	doc := encodeCompositeJSON(t, layerSpec(``, `, "axis": {"tick_count": 2}`, ``, ``))
	y := doc.Grid.Shared.Y
	if y == nil {
		t.Fatal("shared y axis missing")
	}
	base := encodeCompositeJSON(t, layerSpec(``, ``, ``, ``)).Grid.Shared.Y
	if len(majorTickValues(t, *y)) >= len(majorTickValues(t, *base)) {
		t.Errorf("layer shared y majors = %v, want fewer than the default %v",
			majorTickValues(t, *y), majorTickValues(t, *base))
	}
	wantGridFollowsMajors(t, *y)

	// And the pinned form, including its warning routing.
	doc = encodeCompositeJSON(t, layerSpec(``, `, "axis": {"values": [1, 3, 99]}`, ``, ``))
	y = doc.Grid.Shared.Y
	if y == nil {
		t.Fatal("shared y axis missing")
	}
	got := majorTickValues(t, *y)
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("layer shared y majors = %v, want [1 3] (99 is off-domain)", got)
	}
	if len(y.Grid) != 2 {
		t.Errorf("layer shared y grid lines = %d, want 2", len(y.Grid))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Code == scene.WarnAxisValuesDropped {
			found = true
		}
	}
	if !found {
		t.Errorf("shared-axis path swallowed the %s warning: %+v",
			scene.WarnAxisValuesDropped, doc.Warnings)
	}
}

// TestPrismFacetSharedAxisHonoursTickValues: same, one level down.
func TestPrismFacetSharedAxisHonoursTickValues(t *testing.T) {
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
      "x": {"field": "day", "type": "nominal"},
      "y": {"field": "score", "type": "quantitative", "axis": {"values": [0, 2, 4]}}
    }
  }
}`
	y := encodeCompositeJSON(t, body).Grid.Shared.Y
	if y == nil {
		t.Fatal("facet shared y axis missing")
	}
	got := majorTickValues(t, *y)
	if len(got) != 3 || got[0] != 0 || got[1] != 2 || got[2] != 4 {
		t.Errorf("facet shared y majors = %v, want [0 2 4]", got)
	}
	if len(y.Grid) != 3 {
		t.Errorf("facet shared y grid lines = %d, want 3", len(y.Grid))
	}
}
