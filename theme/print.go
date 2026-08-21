package theme

// printColors is the print base's chrome palette. Print has no
// backlight and no dark companion, so the grid can be a real grey
// rather than the near-invisible screen value — a 4% grid that reads
// on an OLED disappears entirely at 300dpi on paper.
var printColors = chromeColors{
	Text:       "#000000",
	TextMuted:  "#444444",
	Axis:       "#666666",
	Grid:       "#DDDDDD",
	Domain:     "#999999",
	Zero:       "#000000",
	Background: "#ffffff",
	Surface:    "#ffffff",
}

// printTheme: neutral grayscale, no saturated fills, hatch-friendly.
// Targets monochrome print output without dynamic background fills.
func printTheme() *Theme {
	sc := ChromeScale{LabelSize: 10, TitleSize: 14}
	c := printColors
	return &Theme{
		AxisColor:         c.Axis,
		GridColor:         c.Grid,
		TextColor:         c.Text,
		TextMutedColor:    c.TextMuted,
		BackgroundColor:   c.Background,
		FontSans:          "Georgia, Times, serif",
		FontMono:          "Courier, monospace",
		FontSizeLabel:     sc.LabelSize,
		FontSizeTitle:     sc.TitleSize,
		FontSizeAxisTitle: sc.LabelSize,
		// Ordered light-to-dark alternating rather than monotonically,
		// so ADJACENT series differ by the largest available step of
		// grey — the only channel a monochrome palette has.
		ColorSchemeCategorical: []string{
			"#222222", "#999999", "#555555", "#bbbbbb",
			"#000000", "#888888", "#444444", "#aaaaaa",
		},
		ColorSchemeSequential: []string{
			"#f5f5f5", "#e0e0e0", "#bdbdbd", "#9e9e9e",
			"#757575", "#616161", "#424242", "#212121",
		},
		Mark: &MarkStyle{
			Fill:        "#000000",
			Opacity:     ptr(1),
			StrokeWidth: ptr(0),
		},
		Marks:  printMarks(c),
		Axis:   chromeAxis(sc, c),
		Legend: chromeLegend(sc, c),
		Title:  chromeTitle(sc, c),
		View:   chromeView(c),
		Range: &Range{
			Ordinal:   &RangeSlot{Scheme: "greys"},
			Ramp:      &RangeSlot{Scheme: "greys"},
			Heatmap:   &RangeSlot{Scheme: "greys"},
			Diverging: &RangeSlot{Scheme: "rdgy"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: ptr(1)},
			"deselected": {Opacity: ptr(0.4)},
		},
	}
}

// printMarks keeps the shared geometry but drops back to ink values
// a monochrome device can hold apart, and keeps the bar's corner
// radius at 0: a rounded corner at print resolution is a stepped
// edge, not a curve.
func printMarks(c chromeColors) map[string]*MarkStyle {
	m := chromeMarks("#333333", c)
	m["bar"] = &MarkStyle{Fill: "#555555", CornerRadius: ptr(0)}
	m["line"] = &MarkStyle{Stroke: "#000000", StrokeWidth: ptr(1.5), Fill: "transparent"}
	m["area"] = &MarkStyle{Fill: "#9e9e9e", FillOpacity: ptr(0.45), Stroke: "#000000", StrokeWidth: ptr(1.25)}
	m["point"] = &MarkStyle{Fill: "#000000", Stroke: c.Surface, StrokeWidth: ptr(0.75), Size: ptr(56)}
	m["geoshape"] = &MarkStyle{Fill: "#e0e0e0", Stroke: "#000000", StrokeWidth: ptr(0.5)}
	m["arc"] = &MarkStyle{Stroke: "#ffffff", StrokeWidth: ptr(1)}
	return m
}
