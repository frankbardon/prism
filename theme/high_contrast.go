package theme

// highContrastTheme: maximum contrast for low-vision readability and
// projector / presentation use. Pure black axes + text, white
// background, bold defaults, no grid (grid lines are visual noise
// at contrast extremes).
func highContrastTheme() *Theme {
	f := func(v float64) *float64 { return &v }
	return &Theme{
		AxisColor:         "#000000",
		GridColor:         "#000000",
		TextColor:         "#000000",
		BackgroundColor:   "#ffffff",
		FontSans:          "Inter, system-ui, sans-serif",
		FontMono:          "ui-monospace, SF Mono, monospace",
		FontSizeLabel:     13,
		FontSizeTitle:     20,
		FontSizeAxisTitle: 14,
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
			Opacity:     f(1),
			StrokeWidth: f(0),
		},
		Marks: map[string]*MarkStyle{
			"line":     {Stroke: "#000000", StrokeWidth: f(2.5), Fill: "transparent"},
			"rule":     {Stroke: "#000000", StrokeWidth: f(1.5)},
			"area":     {Fill: "#000000", Opacity: f(0.85)},
			"point":    {Fill: "#000000", Stroke: "#ffffff", StrokeWidth: f(1.5), Size: f(100)},
			"bar":      {Fill: "#000000", Stroke: "#000000", StrokeWidth: f(0), CornerRadius: f(0)},
			"text":     {Fill: "#000000", FontSize: f(13), FontWeight: "600"},
			"tick":     {Stroke: "#000000", StrokeWidth: f(1.5)},
			"geoshape": {Fill: "#ffffff", Stroke: "#000000", StrokeWidth: f(1)},
			"geopoint": {Fill: "#000000", Stroke: "#ffffff", StrokeWidth: f(1.5), Size: f(64)},
			"arc":      {Stroke: "#ffffff", StrokeWidth: f(2)},
		},
		Axis: &AxisStyle{
			DomainColor:     "#000000",
			DomainWidth:     f(2),
			TickColor:       "#000000",
			TickWidth:       f(2),
			TickSize:        f(6),
			GridColor:       "#000000",
			GridWidth:       f(0),
			LabelColor:      "#000000",
			LabelFontSize:   f(13),
			LabelFontWeight: "600",
			LabelPadding:    f(6),
			TitleColor:      "#000000",
			TitleFontSize:   f(14),
			TitleFontWeight: "700",
			TitlePadding:    f(10),
		},
		Legend: &LegendStyle{
			StrokeColor:     "#000000",
			StrokeWidth:     f(1),
			LabelColor:      "#000000",
			LabelFontSize:   f(13),
			TitleColor:      "#000000",
			TitleFontSize:   f(14),
			TitleFontWeight: "700",
			SymbolSize:      f(100),
			Padding:         f(12),
			RowPadding:      f(6),
		},
		Title: &TitleStyle{
			Color:      "#000000",
			FontSize:   f(20),
			FontWeight: "700",
			Anchor:     "start",
			Padding:    f(16),
		},
		View: &ViewStyle{
			Background:  "#ffffff",
			Stroke:      "#000000",
			StrokeWidth: f(2),
			Padding:     f(4),
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
			"selected":   {Opacity: f(1), StrokeWidth: f(3)},
			"deselected": {Opacity: f(0.2)},
		},
	}
}
