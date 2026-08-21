package theme

// prismCategorical is the default categorical palette.
//
// Replaces tableau10, which was chosen for familiarity rather than
// for separation and did not survive measurement: two of its first
// five entries (#e45756 red and #54a24b green) sit at a simulated
// ΔE of 1.6 under deuteranopia — indistinguishable — and its worst
// ADJACENT pair, the two series most likely to be compared, is 10.8.
//
// The replacement was chosen against a simulation of normal vision
// plus protanopia, deuteranopia and tritanopia (Viénot/Brettel 1999),
// scoring the minimum CIE ΔE across all four:
//
//	worst adjacent pair    18.9   (was 10.8)
//	worst pair in first 5  14.5   (was  1.6)
//	minimum contrast on white  1.96:1, on the dark ground 2.96:1
//
// Ordering is part of the design, not incidental. Slots 1-5 carry
// the strongest mutual separation because most charts never reach
// slot 6, and each neighbouring pair differs in LIGHTNESS as well as
// hue, which is the separation that survives every dichromacy.
//
// The honest limit: no ten-colour categorical palette is pairwise
// safe under all three dichromacies — slots 1 (blue) and 6 (violet)
// converge for a deuteranope, as the equivalent pair does in
// Okabe-Ito and every other published set. Past ~6 series, colour
// alone cannot carry the encoding and the chart needs faceting or
// direct labels instead.
var prismCategorical = []string{
	"#3366CC", // blue      L45
	"#DD6B0D", // orange    L58
	"#0E9E6E", // green     L58
	"#B02A72", // magenta   L41
	"#E8B117", // amber     L75
	"#6E4BC4", // violet    L42
	"#4FA8DC", // sky       L66
	"#8C5A2B", // brown     L43
	"#66798C", // slate     L50
	"#D9455C", // red       L52
}

// prismCategoricalDark is the same ten hues carried onto a dark
// ground: each entry lifted +16 in Lab lightness with chroma pulled
// back ~14%, which preserves the light set's lightness ORDER (and so
// its separation) while clearing a near-black background at 5.2:1 or
// better.
var prismCategoricalDark = []string{
	"#6294F0", // blue
	"#F7902F", // orange
	"#2CCD97", // green
	"#E750A1", // magenta
	"#F5C63C", // amber
	"#9973F6", // violet
	"#78C6F5", // sky
	"#BB824C", // brown
	"#8DA3B8", // slate
	"#F76E85", // red
}

// lightColors is the light base's chrome palette.
//
// The hierarchy is the change worth noting: text and text-muted were
// previously the same value, so a chart's axis labels carried the
// same weight as its title and competed with the data. Now the ink
// runs marks → title (#0F172A) → legend labels → axis labels and
// titles (#64748B) → domain (#D6DCE4) → grid (#EDF0F4), and a reader
// sees the data first because everything else has stepped back.
var lightColors = chromeColors{
	Text:       "#0F172A",
	TextMuted:  "#64748B",
	Axis:       "#94A3B8",
	Grid:       "#EDF0F4",
	Domain:     "#D6DCE4",
	Zero:       "#94A3B8",
	Background: "transparent",
	Surface:    "#FFFFFF",
}

// lightTheme is the default theme.
func lightTheme() *Theme {
	sc := ChromeScale{LabelSize: 11, TitleSize: 15}
	c := lightColors
	primary := prismCategorical[0]
	return &Theme{
		AxisColor:              c.Axis,
		GridColor:              c.Grid,
		TextColor:              c.Text,
		TextMutedColor:         c.TextMuted,
		BackgroundColor:        c.Background,
		FontSans:               "Inter, system-ui, sans-serif",
		FontMono:               "ui-monospace, SF Mono, monospace",
		FontSizeLabel:          sc.LabelSize,
		FontSizeTitle:          sc.TitleSize,
		FontSizeAxisTitle:      sc.LabelSize,
		ColorSchemeCategorical: append([]string(nil), prismCategorical...),
		ColorSchemeSequential: []string{
			"#440154", "#482878", "#3e4a89", "#31688e", "#26828e",
			"#1f9e89", "#35b779", "#6dcd59", "#fde725",
		},
		Mark: &MarkStyle{
			Fill:        primary,
			Opacity:     ptr(1),
			StrokeWidth: ptr(0),
		},
		Marks:  chromeMarks(primary, c),
		Axis:   chromeAxis(sc, c),
		Legend: chromeLegend(sc, c),
		Title:  chromeTitle(sc, c),
		View:   chromeView(c),
		Range: &Range{
			Category:  &RangeSlot{Colors: append([]string(nil), prismCategorical...)},
			Ordinal:   &RangeSlot{Scheme: "blues"},
			Ramp:      &RangeSlot{Scheme: "viridis"},
			Heatmap:   &RangeSlot{Scheme: "viridis"},
			Diverging: &RangeSlot{Scheme: "rdbu"},
			Symbol:    &RangeSlot{Colors: append([]string(nil), prismCategorical...)},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: ptr(1)},
			"deselected": {Opacity: ptr(0.25)},
		},
	}
}

func ptr(v float64) *float64 { return &v }
