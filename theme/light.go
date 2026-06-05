package theme

// lightTheme is the default theme. Refreshed in the v2 theme phase:
// categorical defaults to tableau10 (more saturated, widely
// recognised, better print contrast); sequential defaults to
// viridis (perceptually uniform, colorblind-safe); diverging
// defaults to rdbu (Brewer 9-class).
func lightTheme() *Theme {
	f := func(v float64) *float64 { return &v }
	return &Theme{
		AxisColor:         "#6b7280",
		GridColor:         "#e5e7eb",
		TextColor:         "#111827",
		BackgroundColor:   "transparent",
		FontSans:          "Inter, system-ui, sans-serif",
		FontMono:          "ui-monospace, SF Mono, monospace",
		FontSizeLabel:     11,
		FontSizeTitle:     16,
		FontSizeAxisTitle: 12,
		ColorSchemeCategorical: []string{
			"#4c78a8", "#f58518", "#e45756", "#72b7b2", "#54a24b",
			"#eeca3b", "#b279a2", "#ff9da6", "#9d755d", "#bab0ac",
		},
		ColorSchemeSequential: []string{
			"#440154", "#482878", "#3e4a89", "#31688e", "#26828e",
			"#1f9e89", "#35b779", "#6dcd59", "#fde725",
		},
		Mark: &MarkStyle{
			Fill:        "#4c78a8",
			Opacity:     f(1),
			StrokeWidth: f(0),
		},
		Marks: map[string]*MarkStyle{
			"line":     {Stroke: "#4c78a8", StrokeWidth: f(1.5), Fill: "transparent"},
			"rule":     {Stroke: "#4c78a8", StrokeWidth: f(1)},
			"area":     {Fill: "#4c78a8", Opacity: f(0.7)},
			"point":    {Fill: "#4c78a8", StrokeWidth: f(0), Size: f(64)},
			"bar":      {Fill: "#4c78a8", CornerRadius: f(0)},
			"text":     {Fill: "#111827", FontSize: f(11)},
			"tick":     {Stroke: "#4c78a8", StrokeWidth: f(1)},
			"geoshape": {Fill: "#cbd5e1", Stroke: "#ffffff", StrokeWidth: f(0.5)},
			"geopoint": {Fill: "#4c78a8", StrokeWidth: f(0), Size: f(36)},
			"arc":      {Stroke: "#ffffff", StrokeWidth: f(1)},
		},
		Axis: &AxisStyle{
			DomainColor:   "#6b7280",
			DomainWidth:   f(1),
			TickColor:     "#6b7280",
			TickWidth:     f(1),
			TickSize:      f(5),
			GridColor:     "#e5e7eb",
			GridWidth:     f(1),
			LabelColor:    "#111827",
			LabelFontSize: f(11),
			LabelPadding:  f(4),
			TitleColor:    "#111827",
			TitleFontSize: f(12),
			TitlePadding:  f(8),
		},
		Legend: &LegendStyle{
			LabelColor:      "#111827",
			LabelFontSize:   f(11),
			TitleColor:      "#111827",
			TitleFontSize:   f(12),
			TitleFontWeight: "600",
			SymbolSize:      f(64),
			Padding:         f(8),
			RowPadding:      f(4),
		},
		Title: &TitleStyle{
			Color:      "#111827",
			FontSize:   f(16),
			FontWeight: "600",
			Anchor:     "start",
			Padding:    f(12),
		},
		View: &ViewStyle{
			Background: "transparent",
			Padding:    f(0),
		},
		Range: &Range{
			Category:  &RangeSlot{Scheme: "tableau10"},
			Ordinal:   &RangeSlot{Scheme: "blues"},
			Ramp:      &RangeSlot{Scheme: "viridis"},
			Heatmap:   &RangeSlot{Scheme: "viridis"},
			Diverging: &RangeSlot{Scheme: "rdbu"},
			Symbol:    &RangeSlot{Scheme: "tableau10"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: f(1)},
			"deselected": {Opacity: f(0.3)},
		},
	}
}
