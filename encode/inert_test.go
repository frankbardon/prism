package encode

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// decodeInertSpec decodes a spec literal or fails the test.
func decodeInertSpec(t *testing.T, src string) *spec.Spec {
	t.Helper()
	s, err := spec.Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// inertCodes returns the warning codes InertFieldWarnings reports for
// src, in emission order.
func inertCodes(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, w := range InertFieldWarnings(decodeInertSpec(t, src)) {
		out = append(out, w.Code)
	}
	return out
}

// inertPaths returns the reported field paths.
func inertPaths(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, w := range InertFieldWarnings(decodeInertSpec(t, src)) {
		p, _ := w.Details["Path"].(string)
		out = append(out, p)
	}
	return out
}

func TestPrismInertMarkDefFieldNotReadByMark(t *testing.T) {
	// pad_angle is arc-family geometry; a bar never reads it.
	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 1}]},
	  "mark": {"type": "bar", "pad_angle": 0.02},
	  "encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}
	}`)
	if len(got) != 1 || got[0] != "mark.pad_angle" {
		t.Fatalf("want one mark.pad_angle warning, got %v", got)
	}
}

func TestPrismInertMarkDefHonouredFieldStaysSilent(t *testing.T) {
	// The same property on a mark that DOES read it must not warn —
	// a false positive here is worse than silence.
	got := inertCodes(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 1}]},
	  "mark": {"type": "pie", "pad_angle": 0.02},
	  "encoding": {"theta": {"field": "b", "type": "quantitative"}, "color": {"field": "a", "type": "nominal"}}
	}`)
	if len(got) != 0 {
		t.Fatalf("honoured pad_angle warned: %v", got)
	}
}

// TestPrismInertMarkDefStrokeDashIsSilent is the inverse of the check
// E7-S1 landed: stroke_dash was dead on every mark until E7-S4 wired
// applyMarkDef -> scene.Style.StrokeDash -> stroke-dasharray, so it is
// now a universal style property and must never be reported.
func TestPrismInertMarkDefStrokeDashIsSilent(t *testing.T) {
	ws := InertFieldWarnings(decodeInertSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 1}]},
	  "mark": {"type": "line", "stroke_dash": [4, 4]},
	  "encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}
	}`))
	if len(ws) != 0 {
		t.Fatalf("honoured stroke_dash warned: %+v", ws)
	}
}

func TestPrismInertChannelSizeAndShapeReported(t *testing.T) {
	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2, "c": 3}]},
	  "mark": {"type": "point"},
	  "encoding": {
	    "x": {"field": "a", "type": "quantitative"},
	    "y": {"field": "b", "type": "quantitative"},
	    "size": {"field": "c", "type": "quantitative"},
	    "shape": {"field": "c", "type": "nominal"}
	  }
	}`)
	want := map[string]bool{"encoding.size": true, "encoding.shape": true}
	if len(got) != 2 {
		t.Fatalf("want 2 warnings, got %v", got)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected path %q (all: %v)", p, got)
		}
	}
}

func TestPrismInertOpacityChannelHeatmapExempt(t *testing.T) {
	heat := `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": "y", "v": 1, "o": 2}]},
	  "mark": {"type": "heatmap"},
	  "encoding": {
	    "x": {"field": "a", "type": "nominal"},
	    "y": {"field": "b", "type": "nominal"},
	    "color": {"field": "v", "type": "nominal"},
	    "opacity": {"field": "o", "type": "quantitative"}
	  }
	}`
	if got := inertCodes(t, heat); len(got) != 0 {
		t.Fatalf("heatmap reads the opacity channel; should not warn: %v", got)
	}
	bar := strings.Replace(heat, `"type": "heatmap"`, `"type": "bar"`, 1)
	if got := inertPaths(t, bar); len(got) != 1 || got[0] != "encoding.opacity" {
		t.Fatalf("want encoding.opacity on bar, got %v", got)
	}
}

func TestPrismInertConditionalChannelStaysSilent(t *testing.T) {
	// A condition on the channel is evaluated by the condition pass,
	// so the binding is not inert.
	got := inertCodes(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2}]},
	  "mark": {"type": "point"},
	  "encoding": {
	    "x": {"field": "a", "type": "quantitative"},
	    "y": {"field": "b", "type": "quantitative"},
	    "size": {"type": "quantitative", "condition": {"test": {"op": "gt", "field": "a", "value": 0}, "value": 100}, "value": 10}
	  }
	}`)
	if len(got) != 0 {
		t.Fatalf("conditional size channel warned: %v", got)
	}
}

func TestPrismInertScaleKnobFamilyMismatch(t *testing.T) {
	// padding_inner is band geometry; a quantitative channel resolves
	// to linear and never reads it.
	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2}]},
	  "mark": {"type": "point"},
	  "encoding": {
	    "x": {"field": "a", "type": "quantitative", "scale": {"padding_inner": 0.2}},
	    "y": {"field": "b", "type": "quantitative"}
	  }
	}`)
	if len(got) != 1 || got[0] != "encoding.x.scale.padding_inner" {
		t.Fatalf("want encoding.x.scale.padding_inner, got %v", got)
	}
}

func TestPrismInertScaleKnobOnMatchingFamilyStaysSilent(t *testing.T) {
	got := inertCodes(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 2}]},
	  "mark": {"type": "bar"},
	  "encoding": {
	    "x": {"field": "a", "type": "nominal", "scale": {"padding_inner": 0.2, "align": 0.5}},
	    "y": {"field": "b", "type": "quantitative", "scale": {"zero": true, "nice": true}}
	  }
	}`)
	if len(got) != 0 {
		t.Fatalf("honoured scale knobs warned: %v", got)
	}
}

func TestPrismInertScaleBaseOnlyAppliesToLog(t *testing.T) {
	logSpec := `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2}]},
	  "mark": {"type": "point"},
	  "encoding": {
	    "x": {"field": "a", "type": "quantitative"},
	    "y": {"field": "b", "type": "quantitative", "scale": {"type": "log", "base": 2}}
	  }
	}`
	if got := inertCodes(t, logSpec); len(got) != 0 {
		t.Fatalf("base on a log scale warned: %v", got)
	}
	linear := strings.Replace(logSpec, `"type": "log", `, "", 1)
	if got := inertPaths(t, linear); len(got) != 1 || got[0] != "encoding.y.scale.base" {
		t.Fatalf("want encoding.y.scale.base on linear, got %v", got)
	}
}

// TestPrismInertLegendKeysAreSilent pins the E7-S3 correction: every
// legend presentation key is honoured (E3-S4 wired all five), so the
// detector must say nothing about them. Until E7-S3 it emitted
// PRISM_WARN_LEGEND_FIELD_INERT on specs the encoder obeyed.
func TestPrismInertLegendKeysAreSilent(t *testing.T) {
	if got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2, "c": "x"}]},
	  "mark": {"type": "point"},
	  "encoding": {
	    "x": {"field": "a", "type": "quantitative"},
	    "y": {"field": "b", "type": "quantitative"},
	    "color": {"field": "c", "type": "nominal", "legend": {"title": "Cohort", "symbol_type": "square", "direction": "horizontal", "symbol_size": 80, "tick_count": 3}}
	  }
	}`); len(got) != 0 {
		t.Fatalf("legend presentation keys warned: %v", got)
	}
}

func TestPrismInertFacetChildFeaturesDropped(t *testing.T) {
	ws := InertFieldWarnings(decodeInertSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 1, "g": "p", "f": "r1"}]},
	  "facet": {"column": {"field": "f", "type": "nominal"}},
	  "spec": {
	    "mark": {"type": "bar"},
	    "encoding": {
	      "x": {"field": "a", "type": "nominal"},
	      "y": {"aggregate": "sum", "field": "b", "type": "quantitative", "stack": "zero"},
	      "color": {"field": "g", "type": "nominal"},
	      "order": {"field": "b", "type": "quantitative"}
	    }
	  }
	}`))
	var features []string
	for _, w := range ws {
		if w.Code != scene.WarnFacetChildSkipped {
			t.Fatalf("unexpected code %s", w.Code)
		}
		f, _ := w.Details["Feature"].(string)
		features = append(features, f)
	}
	if len(features) != 3 {
		t.Fatalf("want aggregate + stack + order reported, got %v", features)
	}
}

func TestPrismInertReportedOncePerCompositionChild(t *testing.T) {
	// One reporter: a layer child's inert key is reported exactly
	// once, from the root walk, and carries its layer path.
	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 1}]},
	  "layer": [
	    {"mark": {"type": "bar"}, "encoding": {"x": {"field": "a", "type": "nominal"}, "y": {"field": "b", "type": "quantitative"}}},
	    {"mark": {"type": "rule", "tooltip": true}, "encoding": {"y": {"field": "b", "type": "quantitative"}}}
	  ]
	}`)
	if len(got) != 1 || got[0] != "layer[1].mark.tooltip" {
		t.Fatalf("want a single layer[1].mark.tooltip warning, got %v", got)
	}
}

func TestPrismInertHonouredSpecIsSilent(t *testing.T) {
	// A spec built entirely from honoured surface must emit nothing —
	// the property that keeps the warning worth reading.
	got := inertCodes(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"m": "Jan", "v": 1, "g": "a"}]},
	  "mark": {"type": "bar", "corner_radius": 2, "fill_opacity": 0.8},
	  "encoding": {
	    "x": {"field": "m", "type": "nominal", "axis": {"label_angle": -45, "format": ".0f"}, "scale": {"padding": 0.2}},
	    "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "scale": {"zero": true}, "stack": "zero"},
	    "color": {"field": "g", "type": "nominal", "legend": {"title": "Group", "orient": "right"}}
	  }
	}`)
	if len(got) != 0 {
		t.Fatalf("honoured spec warned: %v", got)
	}
}

func TestPrismFormatTickUsesD3Subset(t *testing.T) {
	// E7-S1: axis.format ran through fmt.Sprintf, so a documented
	// d3 specifier rendered "%!(NOVERB)".
	if got := formatTick(0.4, ".0%"); got != "40%" {
		t.Fatalf("formatTick(0.4, \".0%%\") = %q, want 40%%", got)
	}
	if got := formatTick(1234, ",.0f"); got != "1,234" {
		t.Fatalf("formatTick(1234, \"$,.0f\") = %q", got)
	}
	// Default and unparseable paths keep the historic %g rendering,
	// which is what holds every committed golden.
	if got := formatTick(12.5, ""); got != "12.5" {
		t.Fatalf("default formatTick = %q", got)
	}
	if got := formatTick(12.5, "!!not-a-format"); got != "12.5" {
		t.Fatalf("unparseable formatTick = %q", got)
	}
}

// TestPrismInertTableColumnFormatOnlyDeadUnderSubMark pins the
// narrowed columns[].format check (E7-S4): a text column's format is
// applied by buildTable, so it must be silent; the same key on a
// column bound to a sub-mark still has no text to shape and is
// reported.
func TestPrismInertTableColumnFormatOnlyDeadUnderSubMark(t *testing.T) {
	silent := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "b": 1}]},
	  "mark": {"type": "table"},
	  "encoding": {"columns": [{"field": "b", "type": "quantitative", "format": ",.0f"}]}
	}`)
	if len(silent) != 0 {
		t.Fatalf("honoured columns[].format warned: %v", silent)
	}

	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": "x", "trend": "[1,2,3]"}]},
	  "mark": {"type": "table"},
	  "encoding": {"columns": [
	    {"field": "a", "type": "nominal"},
	    {"field": "trend", "type": "quantitative", "mark": "sparkline", "format": ",.0f"}
	  ]}
	}`)
	if len(got) != 1 || got[0] != "encoding.columns[1].format" {
		t.Fatalf("want a single sub-mark column warning, got %v", got)
	}
}

// TestPrismInertOffsetChannelReported pins E1-S1's honesty clause: the
// x_offset / y_offset wire surface landed ahead of the bar geometry
// that draws it, so binding one today says so instead of silently
// dodging no bars. Both deadChannels entries come out when the encoder
// lands (E1-S3 / E1-S4) and this test inverts with them.
func TestPrismInertOffsetChannelReported(t *testing.T) {
	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2, "c": 3}]},
	  "mark": {"type": "bar"},
	  "encoding": {
	    "x": {"field": "a", "type": "nominal"},
	    "y": {"field": "b", "type": "quantitative"},
	    "x_offset": {"field": "c", "type": "nominal"}
	  }
	}`)
	if len(got) != 1 || got[0] != "encoding.x_offset" {
		t.Fatalf("want [encoding.x_offset], got %v", got)
	}
}

// TestPrismInertOffsetChannelUnboundIsSilent keeps every pre-E1-S1
// spec warning-free: an absent offset channel must report nothing.
func TestPrismInertOffsetChannelUnboundIsSilent(t *testing.T) {
	got := inertPaths(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"a": 1, "b": 2}]},
	  "mark": {"type": "bar"},
	  "encoding": {
	    "x": {"field": "a", "type": "nominal"},
	    "y": {"field": "b", "type": "quantitative"}
	  }
	}`)
	if len(got) != 0 {
		t.Fatalf("want no warnings, got %v", got)
	}
}
