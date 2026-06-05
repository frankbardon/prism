package theme

// darkTheme inverts the light palette: dark background, light text,
// brighter primary mark colors for contrast. v2 refresh: observable10
// for categorical (designed for dark mode), magma for sequential.
func darkTheme() *Theme {
	f := func(v float64) *float64 { return &v }
	primary := "#4269d0"
	return &Theme{
		AxisColor:         "#9ca3af",
		GridColor:         "#374151",
		TextColor:         "#f3f4f6",
		BackgroundColor:   "#0b1220",
		FontSans:          "Inter, system-ui, sans-serif",
		FontMono:          "ui-monospace, SF Mono, monospace",
		FontSizeLabel:     11,
		FontSizeTitle:     16,
		FontSizeAxisTitle: 12,
		ColorSchemeCategorical: []string{
			"#4269d0", "#efb118", "#ff725c", "#6cc5b0", "#3ca951",
			"#ff8ab7", "#a463f2", "#97bbf5", "#9c6b4e", "#9498a0",
		},
		ColorSchemeSequential: []string{
			"#000004", "#1c1044", "#4f127b", "#812581", "#b5367a",
			"#e55964", "#fb8861", "#fec287", "#fcfdbf",
		},
		Mark: &MarkStyle{
			Fill:        primary,
			Opacity:     f(1),
			StrokeWidth: f(0),
		},
		Marks: map[string]*MarkStyle{
			"line":     {Stroke: primary, StrokeWidth: f(1.5), Fill: "transparent"},
			"rule":     {Stroke: primary, StrokeWidth: f(1)},
			"area":     {Fill: primary, Opacity: f(0.7)},
			"point":    {Fill: primary, StrokeWidth: f(0), Size: f(64)},
			"bar":      {Fill: primary, CornerRadius: f(0)},
			"text":     {Fill: "#f3f4f6", FontSize: f(11)},
			"tick":     {Stroke: primary, StrokeWidth: f(1)},
			"geoshape": {Fill: "#334155", Stroke: "#0b1220", StrokeWidth: f(0.5)},
			"geopoint": {Fill: primary, StrokeWidth: f(0), Size: f(36)},
			"arc":      {Stroke: "#0b1220", StrokeWidth: f(1)},
		},
		Axis: &AxisStyle{
			DomainColor:   "#9ca3af",
			DomainWidth:   f(1),
			TickColor:     "#9ca3af",
			TickWidth:     f(1),
			TickSize:      f(5),
			GridColor:     "#374151",
			GridWidth:     f(1),
			LabelColor:    "#f3f4f6",
			LabelFontSize: f(11),
			LabelPadding:  f(4),
			TitleColor:    "#f3f4f6",
			TitleFontSize: f(12),
			TitlePadding:  f(8),
		},
		Legend: &LegendStyle{
			LabelColor:      "#f3f4f6",
			LabelFontSize:   f(11),
			TitleColor:      "#f3f4f6",
			TitleFontSize:   f(12),
			TitleFontWeight: "600",
			SymbolSize:      f(64),
			Padding:         f(8),
			RowPadding:      f(4),
		},
		Title: &TitleStyle{
			Color:      "#f3f4f6",
			FontSize:   f(16),
			FontWeight: "600",
			Anchor:     "start",
			Padding:    f(12),
		},
		View: &ViewStyle{
			Background: "#0b1220",
			Padding:    f(0),
		},
		Range: &Range{
			Category:  &RangeSlot{Scheme: "observable10"},
			Ordinal:   &RangeSlot{Scheme: "purples"},
			Ramp:      &RangeSlot{Scheme: "magma"},
			Heatmap:   &RangeSlot{Scheme: "magma"},
			Diverging: &RangeSlot{Scheme: "rdbu"},
			Symbol:    &RangeSlot{Scheme: "observable10"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: f(1)},
			"deselected": {Opacity: f(0.25)},
		},
	}
}
