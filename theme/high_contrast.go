package theme

// highContrastColors: maximum contrast for low-vision readability and
// projector / presentation use.
//
// This base intentionally breaks the muted-chrome hierarchy the other
// bases follow. Everywhere else the axis chrome steps back so the data
// comes forward; here a reader may not be able to resolve a mid-grey
// at all, so text and chrome are pure black and the separation comes
// from WEIGHT and SIZE instead of value. The grid is off for the same
// reason: at this contrast a grid competes with the marks.
var highContrastColors = chromeColors{
	Text:       "#000000",
	TextMuted:  "#000000",
	Axis:       "#000000",
	Grid:       "#000000",
	Domain:     "#000000",
	Zero:       "#000000",
	Background: "#ffffff",
	Surface:    "#ffffff",
}

func highContrastTheme() *Theme {
	sc := ChromeScale{LabelSize: 13, TitleSize: 20}
	c := highContrastColors
	t := &Theme{
		AxisColor:         c.Axis,
		GridColor:         c.Grid,
		TextColor:         c.Text,
		TextMutedColor:    c.TextMuted,
		BackgroundColor:   c.Background,
		FontSans:          "Inter, system-ui, sans-serif",
		FontMono:          "ui-monospace, SF Mono, monospace",
		FontSizeLabel:     sc.LabelSize,
		FontSizeTitle:     sc.TitleSize,
		FontSizeAxisTitle: sc.LabelSize,
		ColorSchemeCategorical: []string{
			"#000000", "#1a73e8", "#d32f2f", "#388e3c",
			"#f57c00", "#7b1fa2", "#00838f", "#5d4037",
		},
		ColorSchemeSequential: []string{
			"#ffffff", "#e0e0e0", "#bdbdbd", "#9e9e9e",
			"#757575", "#616161", "#424242", "#212121", "#000000",
		},
		Mark: &MarkStyle{
			Fill:        "#000000",
			Stroke:      "#000000",
			Opacity:     ptr(1),
			StrokeWidth: ptr(0),
		},
		Marks:  highContrastMarks(c),
		Axis:   chromeAxis(sc, c),
		Legend: chromeLegend(sc, c),
		Title:  chromeTitle(sc, c),
		View: &ViewStyle{
			Background:  c.Background,
			Stroke:      "#000000",
			StrokeWidth: ptr(2),
			Padding:     ptr(12),
		},
		Range: &Range{
			Category:  &RangeSlot{Scheme: "dark2"},
			Ordinal:   &RangeSlot{Scheme: "greys"},
			Ramp:      &RangeSlot{Scheme: "greys"},
			Heatmap:   &RangeSlot{Scheme: "greys"},
			Diverging: &RangeSlot{Scheme: "puor"},
			Symbol:    &RangeSlot{Scheme: "dark2"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: ptr(1), StrokeWidth: ptr(3)},
			"deselected": {Opacity: ptr(0.2)},
		},
	}
	// Heavier chrome than the shared scale gives, and no grid at all.
	t.Axis.DomainWidth = ptr(2)
	t.Axis.TickWidth = ptr(2)
	t.Axis.GridWidth = ptr(0)
	t.Axis.ZeroWidth = ptr(2.5)
	t.Axis.LabelFontWeight = "600"
	t.Axis.TitleFontWeight = "700"
	t.Axis.BandPadding = ptr(0.32)
	t.Legend.TitleFontWeight = "700"
	t.Legend.StrokeColor = "#000000"
	t.Legend.StrokeWidth = ptr(1)
	t.Legend.SymbolCornerRadius = ptr(0)
	t.Title.FontWeight = "700"
	return t
}

func highContrastMarks(c chromeColors) map[string]*MarkStyle {
	m := chromeMarks("#000000", c)
	m["line"] = &MarkStyle{Stroke: "#000000", StrokeWidth: ptr(3), Fill: "transparent"}
	m["rule"] = &MarkStyle{Stroke: "#000000", StrokeWidth: ptr(1.5)}
	m["area"] = &MarkStyle{Fill: "#000000", FillOpacity: ptr(0.6), Stroke: "#000000", StrokeWidth: ptr(2.5)}
	m["point"] = &MarkStyle{Fill: "#000000", Stroke: "#ffffff", StrokeWidth: ptr(1.5), Size: ptr(120)}
	m["bar"] = &MarkStyle{Fill: "#000000", Stroke: "#000000", StrokeWidth: ptr(0), CornerRadius: ptr(0)}
	m["text"] = &MarkStyle{Fill: "#000000", FontSize: ptr(13), FontWeight: "600"}
	m["tick"] = &MarkStyle{Stroke: "#000000", StrokeWidth: ptr(1.5)}
	m["geoshape"] = &MarkStyle{Fill: "#ffffff", Stroke: "#000000", StrokeWidth: ptr(1)}
	m["geopoint"] = &MarkStyle{Fill: "#000000", Stroke: "#ffffff", StrokeWidth: ptr(1.5), Size: ptr(80)}
	m["arc"] = &MarkStyle{Stroke: "#ffffff", StrokeWidth: ptr(2)}
	return m
}
