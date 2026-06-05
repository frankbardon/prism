package theme

// printTheme: neutral grayscale, no saturated fills, hatch-friendly.
// Targets monochrome print output without dynamic background fills.
func printTheme() *Theme {
	f := func(v float64) *float64 { return &v }
	return &Theme{
		AxisColor:         "#000000",
		GridColor:         "#cccccc",
		TextColor:         "#000000",
		BackgroundColor:   "#ffffff",
		FontSans:          "Georgia, Times, serif",
		FontMono:          "Courier, monospace",
		FontSizeLabel:     10,
		FontSizeTitle:     14,
		FontSizeAxisTitle: 11,
		ColorSchemeCategorical: []string{
			"#000000", "#555555", "#888888", "#aaaaaa",
			"#222222", "#666666", "#999999", "#bbbbbb",
		},
		ColorSchemeSequential: []string{
			"#f5f5f5", "#e0e0e0", "#bdbdbd", "#9e9e9e",
			"#757575", "#616161", "#424242", "#212121",
		},
		Mark: &MarkStyle{
			Fill:        "#000000",
			Opacity:     f(1),
			StrokeWidth: f(0),
		},
		Marks: map[string]*MarkStyle{
			"line":     {Stroke: "#000000", StrokeWidth: f(1.25), Fill: "transparent"},
			"rule":     {Stroke: "#000000", StrokeWidth: f(1)},
			"area":     {Fill: "#9e9e9e", Opacity: f(0.7)},
			"point":    {Fill: "#000000", StrokeWidth: f(0), Size: f(36)},
			"bar":      {Fill: "#555555", CornerRadius: f(0)},
			"text":     {Fill: "#000000", FontSize: f(10)},
			"tick":     {Stroke: "#000000", StrokeWidth: f(1)},
			"geoshape": {Fill: "#e0e0e0", Stroke: "#000000", StrokeWidth: f(0.5)},
			"geopoint": {Fill: "#000000", StrokeWidth: f(0), Size: f(36)},
			"arc":      {Stroke: "#ffffff", StrokeWidth: f(0.75)},
		},
		Axis: &AxisStyle{
			DomainColor:   "#000000",
			DomainWidth:   f(1),
			TickColor:     "#000000",
			TickWidth:     f(1),
			TickSize:      f(5),
			GridColor:     "#cccccc",
			GridWidth:     f(0.5),
			LabelColor:    "#000000",
			LabelFontSize: f(10),
			LabelPadding:  f(4),
			TitleColor:    "#000000",
			TitleFontSize: f(11),
			TitlePadding:  f(8),
		},
		Legend: &LegendStyle{
			LabelColor:      "#000000",
			LabelFontSize:   f(10),
			TitleColor:      "#000000",
			TitleFontSize:   f(11),
			TitleFontWeight: "600",
			SymbolSize:      f(48),
			Padding:         f(8),
			RowPadding:      f(4),
		},
		Title: &TitleStyle{
			Color:      "#000000",
			FontSize:   f(14),
			FontWeight: "600",
			Anchor:     "start",
			Padding:    f(12),
		},
		View: &ViewStyle{
			Background: "#ffffff",
			Padding:    f(0),
		},
		Range: &Range{
			Ordinal:   &RangeSlot{Scheme: "greys"},
			Ramp:      &RangeSlot{Scheme: "greys"},
			Heatmap:   &RangeSlot{Scheme: "greys"},
			Diverging: &RangeSlot{Scheme: "rdgy"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: f(1)},
			"deselected": {Opacity: f(0.4)},
		},
	}
}
