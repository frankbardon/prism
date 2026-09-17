package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

func progressTable(t *testing.T) *table.Table {
	t.Helper()
	tbl, _, err := table.FromInline("metrics", []map[string]any{
		{"metric": "Familiarity", "score": 92.4, "cap": 100.0},
		{"metric": "Advocacy", "score": 33.2, "cap": 120.0},
	}, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	return tbl
}

// A literal total widens the measure domain so the track — which runs
// to that total — cannot land past the plot edge. This is the bug
// bullet still has when a band bound sits above the data range.
func TestPrismProgressMeasureExtrasLiteralTotal(t *testing.T) {
	got := progressMeasureExtras(&spec.MarkDef{Type: "progress", Total: 100.0}, progressTable(t))
	if len(got) != 1 {
		t.Fatalf("extras = %v, want exactly one value", got)
	}
	if f, ok := got[0].(float64); !ok || f != 100 {
		t.Errorf("extras[0] = %v, want 100", got[0])
	}
}

// A field-name total is read per row, not collapsed to row 0 — that
// row-0 collapse is exactly the bullet limitation this mark exists to
// remove, so every row's ceiling has to reach the domain.
func TestPrismProgressMeasureExtrasFieldTotalIsPerRow(t *testing.T) {
	got := progressMeasureExtras(&spec.MarkDef{Type: "progress", Total: "cap"}, progressTable(t))
	if len(got) != 2 {
		t.Fatalf("extras = %v, want one value per row", got)
	}
	if got[1] != 120.0 {
		t.Errorf("extras[1] = %v, want the second row's cap (120)", got[1])
	}
}

func TestPrismProgressMeasureExtrasAbsentTotal(t *testing.T) {
	if got := progressMeasureExtras(&spec.MarkDef{Type: "progress"}, progressTable(t)); got != nil {
		t.Errorf("extras = %v, want nil when total is unset", got)
	}
}

// The measure axis follows mark.orient when declared and is inferred
// from which channel is discrete otherwise — the same rule
// marks.MarkOrientation applies to the resolved scales.
func TestPrismProgressMeasureAxisFollowsMarkOrient(t *testing.T) {
	quant := &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}}
	nominal := &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "metric", Type: "nominal"}}

	cases := []struct {
		name   string
		def    *spec.MarkDef
		enc    *spec.Encoding
		wantX  bool
		reason string
	}{
		{"inferred horizontal", &spec.MarkDef{Type: "progress"},
			&spec.Encoding{X: quant, Y: nominal}, true, "nominal y, quantitative x"},
		{"inferred vertical", &spec.MarkDef{Type: "progress"},
			&spec.Encoding{X: nominal, Y: quant}, false, "nominal x, quantitative y"},
		{"explicit vertical wins", &spec.MarkDef{Type: "progress", Orient: "vertical"},
			&spec.Encoding{X: quant, Y: nominal}, false, "orient overrides the inference"},
		{"explicit horizontal wins", &spec.MarkDef{Type: "progress", Orient: "horizontal"},
			&spec.Encoding{X: nominal, Y: quant}, true, "orient overrides the inference"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := progressMeasureIsX(tc.def, tc.enc); got != tc.wantX {
				t.Errorf("progressMeasureIsX = %v, want %v (%s)", got, tc.wantX, tc.reason)
			}
		})
	}
}

// Without a progress_track token the track still comes from the
// theme rather than a constant baked into the mark encoder — the grid
// colour, because the track is chrome rather than a second series.
// This is the fallback path a custom theme that never heard of
// progress takes.
func TestPrismProgressTrackStyleFallsBackToGridColour(t *testing.T) {
	custom := &theme.Theme{Axis: &theme.AxisStyle{GridColor: "#123456"}}
	got := progressTrackStyle(custom)
	if got.Fill == nil {
		t.Fatal("track style has no fill")
	}
	if got.Fill.R != 0x12 || got.Fill.G != 0x34 || got.Fill.B != 0x56 {
		t.Errorf("track fill = %+v, want the theme's grid colour #123456", got.Fill)
	}

	// The legacy flat token is the fallback when no nested axis block
	// sets one.
	legacy := progressTrackStyle(&theme.Theme{GridColor: "#abcdef"})
	if legacy.Fill == nil || legacy.Fill.R != 0xab {
		t.Errorf("legacy grid fill = %+v, want #abcdef", legacy.Fill)
	}
}

// The token is the point of E10-S2: a theme that states
// marks.progress_track gets that value, not the grid-derived guess.
func TestPrismProgressTrackTokenWinsOverGridColour(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	custom := &theme.Theme{
		Axis: &theme.AxisStyle{GridColor: "#123456"},
		Marks: map[string]*theme.MarkStyle{
			theme.MarksKeyProgressTrack: {Fill: "#fedcba", Stroke: "#010203", StrokeWidth: f(2)},
		},
	}
	got := progressTrackStyle(custom)
	if got.Fill == nil || got.Fill.R != 0xfe || got.Fill.G != 0xdc || got.Fill.B != 0xba {
		t.Errorf("track fill = %+v, want the token's #fedcba", got.Fill)
	}
	// The whole MarkStyle surface reaches the track, not just a fill —
	// that is what a full theme.Marks key buys over a colour-only slot.
	if got.Stroke == nil || got.Stroke.R != 0x01 {
		t.Errorf("track stroke = %+v, want the token's #010203", got.Stroke)
	}
	if got.StrokeWidth != 2 {
		t.Errorf("track stroke_width = %v, want 2", got.StrokeWidth)
	}
}

// The global theme.Mark block carries the *data* fill. Folding it into
// the track (which t.MarkDefault would do) paints the track the same
// colour as the value bar sitting on it and erases the reading, so
// progressTrackStyle reads t.Marks directly. Guards that choice.
func TestPrismProgressTrackIgnoresGlobalMarkBlock(t *testing.T) {
	custom := &theme.Theme{
		Mark: &theme.MarkStyle{Fill: "#4c78a8"},
		Axis: &theme.AxisStyle{GridColor: "#123456"},
	}
	got := progressTrackStyle(custom)
	if got.Fill == nil {
		t.Fatal("track style has no fill")
	}
	if got.Fill.R == 0x4c && got.Fill.G == 0x78 && got.Fill.B == 0xa8 {
		t.Error("track inherited theme.Mark's data fill; it must not fold the global mark block")
	}
}

// Every bundled theme resolves a track, and the bundled values differ
// across themes rather than all collapsing onto the last-resort
// constant.
func TestPrismProgressTrackStyleComesFromTheme(t *testing.T) {
	light := progressTrackStyle(theme.MustGet("light"))
	dark := progressTrackStyle(theme.MustGet("dark"))
	if light.Fill == nil || dark.Fill == nil || *light.Fill == *dark.Fill {
		t.Errorf("light (%+v) and dark (%+v) tracks should differ", light.Fill, dark.Fill)
	}

	// Preserved from E10-S1 on purpose: light's track is still its grid
	// colour, so promoting the derivation to a token moved no pixels.
	want, err := scene.ColorFromHex("#e5e7eb")
	if err != nil {
		t.Fatalf("ColorFromHex: %v", err)
	}
	if *light.Fill != *want {
		t.Errorf("light track fill = %+v, want the pre-token #e5e7eb", light.Fill)
	}

	// high_contrast is the theme whose grid colour (pure black) would
	// have hidden its own black value bar, so it states a different
	// track. Checks the token is actually reachable per theme.
	hc := progressTrackStyle(theme.MustGet("high_contrast"))
	if hc.Fill == nil {
		t.Fatal("high_contrast track has no fill")
	}
	if hc.Fill.R == 0 && hc.Fill.G == 0 && hc.Fill.B == 0 {
		t.Error("high_contrast track resolved to black, hiding its own value bar")
	}
}

// A nil theme still draws something rather than an unpainted rect.
func TestPrismProgressTrackStyleNilTheme(t *testing.T) {
	got := progressTrackStyle(nil)
	if got.Fill == nil {
		t.Fatal("nil theme produced an unpainted track")
	}
}
