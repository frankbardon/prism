package theme

// darkColors is the dark base's chrome palette.
//
// Held to the same hierarchy as light rather than a straight
// inversion: an inverted light theme puts pure white on near-black,
// which glares and makes the axis chrome the brightest thing on the
// chart. Text sits at #E6EBF2 rather than #FFFFFF, muted at #93A3B8,
// and the grid at #1E293B — barely above the ground, which is what
// makes it read as a grid rather than as a cage.
var darkColors = chromeColors{
	Text:       "#E6EBF2",
	TextMuted:  "#93A3B8",
	Axis:       "#64748B",
	Grid:       "#1E293B",
	Domain:     "#334155",
	Zero:       "#64748B",
	Background: "#0F1620",
	Surface:    "#0F1620",
}

// darkTheme carries the light base's hues onto a dark ground.
//
// It is also the companion CSSVariables emits alongside the light
// token set, so a host that flips to dark repaints every chart on the
// page through CSS alone. That coupling is why the two bases must
// stay in lock-step on GEOMETRY: only colour may differ between them,
// or a theme flip would move the drawing and the two token sets would
// disagree about where the plot rect is.
func darkTheme() *Theme {
	sc := ChromeScale{LabelSize: 11, TitleSize: 15}
	c := darkColors
	primary := prismCategoricalDark[0]
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
		ColorSchemeCategorical: append([]string(nil), prismCategoricalDark...),
		ColorSchemeSequential: []string{
			"#000004", "#1c1044", "#4f127b", "#812581", "#b5367a",
			"#e55964", "#fb8861", "#fec287", "#fcfdbf",
		},
		Mark: &MarkStyle{
			Fill:        primary,
			Opacity:     ptr(1),
			StrokeWidth: ptr(0),
		},
		Marks:  darkMarks(primary, c),
		Axis:   chromeAxis(sc, c),
		Legend: chromeLegend(sc, c),
		Title:  chromeTitle(sc, c),
		View:   chromeView(c),
		Range: &Range{
			Category:  &RangeSlot{Colors: append([]string(nil), prismCategoricalDark...)},
			Ordinal:   &RangeSlot{Scheme: "purples"},
			Ramp:      &RangeSlot{Scheme: "magma"},
			Heatmap:   &RangeSlot{Scheme: "magma"},
			Diverging: &RangeSlot{Scheme: "rdbu"},
			Symbol:    &RangeSlot{Colors: append([]string(nil), prismCategoricalDark...)},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: ptr(1)},
			"deselected": {Opacity: ptr(0.25)},
		},
	}
}

// darkMarks adjusts two of the shared mark defaults for a dark
// ground: the geoshape base fill has to sit above the background
// rather than below it, and an area's fill needs more opacity to
// register at all against near-black.
func darkMarks(primary string, c chromeColors) map[string]*MarkStyle {
	m := chromeMarks(primary, c)
	m["geoshape"] = &MarkStyle{Fill: "#334155", Stroke: c.Surface, StrokeWidth: ptr(0.5)}
	m["area"] = &MarkStyle{Fill: primary, FillOpacity: ptr(0.24), Stroke: primary, StrokeWidth: ptr(2)}
	return m
}
