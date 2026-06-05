package theme

// colorblindTheme: Okabe-Ito categorical palette (CUD, Nature
// Methods 8:441) + cividis sequential (designed for deuteranopia
// by Nuñez et al. 2018). Verified safe for protanopia,
// deuteranopia, tritanopia, and grayscale conversion.
func colorblindTheme() *Theme {
	f := func(v float64) *float64 { return &v }
	// Okabe-Ito 8-color set; uses the orange (#e69f00) as the
	// default mark fill to avoid the black/yellow extremes
	// reading as "missing data" in single-mark charts.
	primary := "#e69f00"
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
			"#000000", "#e69f00", "#56b4e9", "#009e73",
			"#f0e442", "#0072b2", "#d55e00", "#cc79a7",
		},
		ColorSchemeSequential: []string{
			"#00224e", "#123570", "#3b496c", "#575c6d", "#707173",
			"#8a8678", "#a59c74", "#c3b369", "#fee838",
		},
		Mark: &MarkStyle{
			Fill:        primary,
			Opacity:     f(1),
			StrokeWidth: f(0),
		},
		Marks: map[string]*MarkStyle{
			"line":     {Stroke: primary, StrokeWidth: f(1.75), Fill: "transparent"},
			"rule":     {Stroke: primary, StrokeWidth: f(1)},
			"area":     {Fill: primary, Opacity: f(0.7)},
			"point":    {Fill: primary, StrokeWidth: f(0), Size: f(72)},
			"bar":      {Fill: primary, CornerRadius: f(0)},
			"text":     {Fill: "#111827", FontSize: f(11)},
			"tick":     {Stroke: primary, StrokeWidth: f(1)},
			"geoshape": {Fill: "#cbd5e1", Stroke: "#ffffff", StrokeWidth: f(0.5)},
			"geopoint": {Fill: primary, StrokeWidth: f(0), Size: f(36)},
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
			SymbolSize:      f(72),
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
			Category:  &RangeSlot{Scheme: "okabe_ito"},
			Ordinal:   &RangeSlot{Scheme: "cividis"},
			Ramp:      &RangeSlot{Scheme: "cividis"},
			Heatmap:   &RangeSlot{Scheme: "cividis"},
			Diverging: &RangeSlot{Scheme: "puor"},
			Symbol:    &RangeSlot{Scheme: "okabe_ito"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: f(1)},
			"deselected": {Opacity: f(0.3)},
		},
	}
}
