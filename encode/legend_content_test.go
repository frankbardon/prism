package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// legendLabels flattens a legend's entry labels for comparison.
func legendLabels(lg scene.Legend) []string {
	out := make([]string, len(lg.Entries))
	for i, e := range lg.Entries {
		out[i] = e.Label
	}
	return out
}

// legendSwatches flattens a legend's swatch colours for comparison.
func legendSwatches(lg scene.Legend) []string {
	out := make([]string, len(lg.Entries))
	for i, e := range lg.Entries {
		if e.Swatch.Color != nil {
			out[i] = e.Swatch.Color.CSS()
		}
	}
	return out
}

// TestPrismLegendTitleOverride covers the three states of
// legend.title: absent (the bound field's name), a string override,
// and `false`, which suppresses the title the way axis.title does.
func TestPrismLegendTitleOverride(t *testing.T) {
	cases := []struct {
		name  string
		block string
		want  string
	}{
		{"absent", "", "g"},
		{"string", `{"title": "Region"}`, "Region"},
		{"false", `{"title": false}`, ""},
		{"empty string", `{"title": ""}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lg := onlyLegend(t, encodeInline(t, legendSpec(tc.block)))
			if lg.Title != tc.want {
				t.Errorf("Title = %q, want %q", lg.Title, tc.want)
			}
		})
	}
}

// TestPrismLegendValuesFilterKeepsSwatchAlignment is the load-bearing
// one: legend.values selects and reorders entries, and each surviving
// entry must keep the palette slot of its *original* category index
// so the swatch still matches the marks.
func TestPrismLegendValuesFilterKeepsSwatchAlignment(t *testing.T) {
	full := onlyLegend(t, encodeInline(t, legendSpec("")))
	if got := legendLabels(full); len(got) != 3 {
		t.Fatalf("unfiltered labels = %v, want 3 entries", got)
	}
	fullColors := legendSwatches(full)

	// Reverse-ordered subset: the author's order drives the rows, the
	// original index drives the colour.
	lg := onlyLegend(t, encodeInline(t, legendSpec(`{"values": ["east", "north"]}`)))
	if got, want := legendLabels(lg), []string{"east", "north"}; !equalStrings(got, want) {
		t.Fatalf("labels = %v, want %v", got, want)
	}
	got := legendSwatches(lg)
	want := []string{fullColors[2], fullColors[0]}
	if !equalStrings(got, want) {
		t.Errorf("swatches = %v, want %v (east then north, original palette slots)", got, want)
	}
}

// TestPrismLegendValuesUnknownDropped pins that a value naming no
// category is dropped rather than rendered as an empty row.
func TestPrismLegendValuesUnknownDropped(t *testing.T) {
	lg := onlyLegend(t, encodeInline(t, legendSpec(`{"values": ["north", "atlantis"]}`)))
	if got, want := legendLabels(lg), []string{"north"}; !equalStrings(got, want) {
		t.Errorf("labels = %v, want %v", got, want)
	}
}

// TestPrismLegendValuesFilterShrinksFrame covers the frame half of
// the story: fewer shown entries means a shorter box.
func TestPrismLegendValuesFilterShrinksFrame(t *testing.T) {
	full := onlyLegend(t, encodeInline(t, legendSpec("")))
	one := onlyLegend(t, encodeInline(t, legendSpec(`{"values": ["north"]}`)))
	if one.Frame.H >= full.Frame.H {
		t.Errorf("filtered frame H = %v, want < unfiltered %v", one.Frame.H, full.Frame.H)
	}
}

// TestPrismLegendValuesFilterShrinksReservation pins that a side
// placement's reserved band follows the *shown* entry count, so the
// plot rect grows back when legend.values narrows the legend.
func TestPrismLegendValuesFilterShrinksReservation(t *testing.T) {
	full := encodeInline(t, legendSpec(`{"orient": "bottom"}`))
	one := encodeInline(t, legendSpec(`{"orient": "bottom", "values": ["north"]}`))
	if one.Plot.H <= full.Plot.H {
		t.Errorf("filtered plot H = %v, want > unfiltered %v", one.Plot.H, full.Plot.H)
	}
}

// TestPrismLegendLabelLimitTruncates covers label_limit: labels are
// ellipsised and the box narrows to the budget.
func TestPrismLegendLabelLimitTruncates(t *testing.T) {
	full := onlyLegend(t, encodeInline(t, legendSpec("")))
	lg := onlyLegend(t, encodeInline(t, legendSpec(`{"label_limit": 24}`)))
	// 24 px / 6 px-per-char = 4 chars: "east" already fits, the two
	// five-letter labels lose a character to the ellipsis.
	if got, want := legendLabels(lg), []string{"nor…", "sou…", "east"}; !equalStrings(got, want) {
		t.Fatalf("labels = %v, want %v", got, want)
	}
	if lg.Frame.W >= full.Frame.W {
		t.Errorf("limited frame W = %v, want < default %v", lg.Frame.W, full.Frame.W)
	}
}

// TestPrismLegendFormatAppliesToNumericCategories pins that
// legend.format goes through the encode/format d3 subset — the same
// one PRISM_SPEC_011 validates the string against — and not through
// fmt verbs, which would render ".0%" as %!(NOVERB).
func TestPrismLegendFormatAppliesToNumericCategories(t *testing.T) {
	body := `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"k": "a", "v": 3, "g": "0.25"},
    {"k": "b", "v": 5, "g": "0.5"},
    {"k": "c", "v": 7, "g": "0.75"}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "k", "type": "nominal"},
    "y": {"field": "v", "type": "quantitative"},
    "color": {"field": "g", "type": "nominal", "legend": {"format": ".0%"}}
  }
}`
	lg := onlyLegend(t, encodeInline(t, body))
	if got, want := legendLabels(lg), []string{"25%", "50%", "75%"}; !equalStrings(got, want) {
		t.Errorf("labels = %v, want %v", got, want)
	}
}

// TestPrismLegendFormatLeavesNonNumericCategories pins the
// pass-through half: a nominal category a number format cannot
// describe is rendered verbatim rather than mangled.
func TestPrismLegendFormatLeavesNonNumericCategories(t *testing.T) {
	lg := onlyLegend(t, encodeInline(t, legendSpec(`{"format": ".2f"}`)))
	if got, want := legendLabels(lg), []string{"north", "south", "east"}; !equalStrings(got, want) {
		t.Errorf("labels = %v, want %v", got, want)
	}
}

// gradientInputs builds a 0..100 gradient legend carrying the content
// resolved from the supplied spec block.
func gradientInputs(lg *spec.Legend) encode.LegendInputs {
	return encode.LegendInputs{
		Channel: scene.ChannelColor,
		Title:   "score",
		Content: encode.ResolveLegendContent(lg),
		Gradient: &encode.GradientLegend{
			ID:        "grad-0",
			DomainMin: 0,
			DomainMax: 100,
		},
	}
}

// gradientTickLabels flattens the labelled stops of a gradient legend.
func gradientTickLabels(lg *scene.Legend) []string {
	if lg == nil || len(lg.Entries) == 0 {
		return nil
	}
	out := make([]string, len(lg.Entries[0].Ticks))
	for i, tk := range lg.Entries[0].Ticks {
		out[i] = tk.Label
	}
	return out
}

// TestPrismGradientLegendTickCount covers legend.tick_count on the
// gradient builder: the default spread, an explicit count, the
// degenerate 1, and 0 (a bare bar).
func TestPrismGradientLegendTickCount(t *testing.T) {
	plot := scene.Rect{X: 40, Y: 20, W: 700, H: 500}
	n3, n1, n0 := 3, 1, 0
	cases := []struct {
		name string
		lg   *spec.Legend
		want []string
	}{
		{"default", nil, []string{"0", "25", "50", "75", "100"}},
		{"three", &spec.Legend{TickCount: &n3}, []string{"0", "50", "100"}},
		{"one", &spec.Legend{TickCount: &n1}, []string{"0"}},
		{"zero", &spec.Legend{TickCount: &n0}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gradientTickLabels(encode.BuildGradientLegend(gradientInputs(tc.lg), plot))
			if !equalStrings(got, tc.want) {
				t.Errorf("tick labels = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPrismGradientLegendTickOffsets pins that stops span the bar end
// to end, so the renderer can place them proportionally.
func TestPrismGradientLegendTickOffsets(t *testing.T) {
	n := 3
	lg := encode.BuildGradientLegend(gradientInputs(&spec.Legend{TickCount: &n}), scene.Rect{W: 700, H: 500})
	ticks := lg.Entries[0].Ticks
	for i, want := range []float64{0, 0.5, 1} {
		if ticks[i].Offset != want {
			t.Errorf("tick %d offset = %v, want %v", i, ticks[i].Offset, want)
		}
	}
}

// TestPrismGradientLegendFormatAndValues covers the other two content
// knobs on a gradient: format renders the stop labels, and values
// pins the stops outright (dropping anything off-domain).
func TestPrismGradientLegendFormatAndValues(t *testing.T) {
	plot := scene.Rect{W: 700, H: 500}
	lg := encode.BuildGradientLegend(gradientInputs(&spec.Legend{
		Format: ".1f",
		Values: []any{0.0, 50.0, 250.0},
	}), plot)
	if got, want := gradientTickLabels(lg), []string{"0.0", "50.0"}; !equalStrings(got, want) {
		t.Errorf("tick labels = %v, want %v", got, want)
	}
	if got, want := lg.Entries[0].Label, "0.0–100.0"; got != want {
		t.Errorf("summary label = %q, want %q", got, want)
	}
}

// TestPrismGradientLegendTitleOverride pins that the title override
// reaches the gradient builder too.
func TestPrismGradientLegendTitleOverride(t *testing.T) {
	plot := scene.Rect{W: 700, H: 500}
	if got := encode.BuildGradientLegend(gradientInputs(nil), plot).Title; got != "score" {
		t.Errorf("default title = %q, want %q", got, "score")
	}
	lg := encode.BuildGradientLegend(gradientInputs(&spec.Legend{Title: false}), plot)
	if lg.Title != "" {
		t.Errorf("suppressed title = %q, want empty", lg.Title)
	}
}

// TestPrismResolveLegendContentDefaults pins that an absent legend
// block resolves to the zero content — the state every committed
// golden was rendered under.
func TestPrismResolveLegendContentDefaults(t *testing.T) {
	c := encode.ResolveLegendContent(nil)
	if c.TitleSet || c.Format != nil || c.TickCount != nil || c.LabelLimit != nil || len(c.Values) != 0 {
		t.Errorf("nil block resolved to %+v, want the zero value", c)
	}
	if got := c.MaxChars(); got != 14 {
		t.Errorf("MaxChars = %v, want the default 14", got)
	}
	if got := c.Label("north"); got != "north" {
		t.Errorf("Label = %q, want passthrough", got)
	}
	if got := c.Truncate("a-very-long-category-label"); got != "a-very-long-category-label" {
		t.Errorf("Truncate = %q, want untouched", got)
	}
}

// TestPrismLegendValuesMatchNumerically pins the coercion: a JSON
// number in legend.values names a numeric-looking category.
func TestPrismLegendValuesMatchNumerically(t *testing.T) {
	c := encode.ResolveLegendContent(&spec.Legend{Values: []any{1.0, "3"}})
	got := c.SelectCategories([]string{"1.0", "2", "3"})
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("SelectCategories = %v, want [0 2]", got)
	}
}

// TestPrismLegendBadFormatIgnored pins that an unparseable
// legend.format degrades to the default rather than failing the
// encode — PRISM_SPEC_011 is where a bad specifier is reported.
func TestPrismLegendBadFormatIgnored(t *testing.T) {
	if c := encode.ResolveLegendContent(&spec.Legend{Format: "%%not-a-spec"}); c.Format != nil {
		t.Errorf("Format = %v, want nil for an unparseable specifier", c.Format)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
