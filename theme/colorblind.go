package theme

// colorblindTheme: Okabe-Ito categorical palette (CUD, Nature
// Methods 8:441) + cividis sequential (designed for deuteranopia by
// Nuñez et al. 2018). Verified safe for protanopia, deuteranopia,
// tritanopia, and grayscale conversion.
//
// It keeps Okabe-Ito rather than adopting the new default palette:
// the default optimises for how a chart LOOKS while staying broadly
// safe, and this base optimises for safety alone, from a published
// set with two decades of use behind it. A reader who selects this
// base is asking for the conservative answer.
//
// The ordering is Okabe-Ito's own with black moved off slot 1: a
// single-series chart drawn in pure black reads as "unstyled" rather
// than as a colour choice, so the orange leads and black — the most
// distinguishable entry of all — is held for slot 5.
func colorblindTheme() *Theme {
	sc := ChromeScale{LabelSize: 11, TitleSize: 15}
	c := lightColors
	primary := "#e69f00"
	return &Theme{
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
			"#e69f00", "#0072b2", "#009e73", "#cc79a7",
			"#000000", "#56b4e9", "#d55e00", "#f0e442",
		},
		ColorSchemeSequential: []string{
			"#00224e", "#123570", "#3b496c", "#575c6d", "#707173",
			"#8a8678", "#a59c74", "#c3b369", "#fee838",
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
			Category:  &RangeSlot{Scheme: "okabe_ito"},
			Ordinal:   &RangeSlot{Scheme: "cividis"},
			Ramp:      &RangeSlot{Scheme: "cividis"},
			Heatmap:   &RangeSlot{Scheme: "cividis"},
			Diverging: &RangeSlot{Scheme: "puor"},
			Symbol:    &RangeSlot{Scheme: "okabe_ito"},
		},
		States: map[string]*StateStyle{
			"selected":   {Opacity: ptr(1)},
			"deselected": {Opacity: ptr(0.25)},
		},
	}
}

// colorblindDarkTheme is the dark companion CSSVariables emits
// alongside the colorblind base. Same Okabe-Ito hues — they are
// chosen for hue separation, which a ground change does not affect —
// lifted where an entry would otherwise vanish into near-black. Black
// becomes white; it is the "maximum separation from everything else"
// slot, and on a dark ground that is white.
func colorblindDarkTheme() *Theme {
	t := colorblindTheme()
	c := darkColors
	t.AxisColor = c.Axis
	t.GridColor = c.Grid
	t.TextColor = c.Text
	t.TextMutedColor = c.TextMuted
	t.BackgroundColor = c.Background
	t.ColorSchemeCategorical = []string{
		"#f0b73a", "#4d9bd6", "#2fbb92", "#e094bd",
		"#ffffff", "#7fcbf2", "#f07a33", "#f5ea6a",
	}
	primary := t.ColorSchemeCategorical[0]
	t.Mark = &MarkStyle{Fill: primary, Opacity: ptr(1), StrokeWidth: ptr(0)}
	t.Marks = darkMarks(primary, c)
	sc := ChromeScale{LabelSize: 11, TitleSize: 15}
	t.Axis = chromeAxis(sc, c)
	t.Legend = chromeLegend(sc, c)
	t.Title = chromeTitle(sc, c)
	t.View = chromeView(c)
	t.Range = &Range{
		Category:  &RangeSlot{Colors: append([]string(nil), t.ColorSchemeCategorical...)},
		Ordinal:   &RangeSlot{Scheme: "cividis"},
		Ramp:      &RangeSlot{Scheme: "cividis"},
		Heatmap:   &RangeSlot{Scheme: "cividis"},
		Diverging: &RangeSlot{Scheme: "puor"},
		Symbol:    &RangeSlot{Colors: append([]string(nil), t.ColorSchemeCategorical...)},
	}
	return t
}

// highContrastDarkColors inverts the high-contrast base rather than
// muting it: the whole point of that base is that nothing is a
// mid-tone, so its dark companion is pure white on pure black.
var highContrastDarkColors = chromeColors{
	Text:       "#ffffff",
	TextMuted:  "#ffffff",
	Axis:       "#ffffff",
	Grid:       "#ffffff",
	Domain:     "#ffffff",
	Zero:       "#ffffff",
	Background: "#000000",
	Surface:    "#000000",
}

// highContrastDarkTheme is the dark companion for high_contrast.
func highContrastDarkTheme() *Theme {
	t := highContrastTheme()
	c := highContrastDarkColors
	sc := ChromeScale{LabelSize: 13, TitleSize: 20}
	t.AxisColor = c.Axis
	t.GridColor = c.Grid
	t.TextColor = c.Text
	t.TextMutedColor = c.TextMuted
	t.BackgroundColor = c.Background
	t.ColorSchemeCategorical = []string{
		"#ffffff", "#7ab8ff", "#ff7b7b", "#6ede84",
		"#ffb14e", "#d9a3ff", "#5fd8e6", "#d3a48c",
	}
	t.Mark = &MarkStyle{Fill: "#ffffff", Stroke: "#ffffff", Opacity: ptr(1), StrokeWidth: ptr(0)}
	t.Marks = highContrastMarks(c)
	for _, m := range t.Marks {
		if m.Fill == "#000000" {
			m.Fill = "#ffffff"
		}
		if m.Stroke == "#000000" {
			m.Stroke = "#ffffff"
		} else if m.Stroke == "#ffffff" {
			m.Stroke = "#000000"
		}
	}
	t.Axis = chromeAxis(sc, c)
	t.Axis.DomainWidth = ptr(2)
	t.Axis.TickWidth = ptr(2)
	t.Axis.GridWidth = ptr(0)
	t.Axis.ZeroWidth = ptr(2.5)
	t.Axis.LabelFontWeight = "600"
	t.Axis.TitleFontWeight = "700"
	t.Axis.BandPadding = ptr(0.32)
	t.Legend = chromeLegend(sc, c)
	t.Legend.TitleFontWeight = "700"
	t.Legend.StrokeColor = "#ffffff"
	t.Legend.StrokeWidth = ptr(1)
	t.Legend.SymbolCornerRadius = ptr(0)
	t.Title = chromeTitle(sc, c)
	t.Title.FontWeight = "700"
	t.View = &ViewStyle{Background: c.Background, Stroke: "#ffffff", StrokeWidth: ptr(2), Padding: ptr(12)}
	return t
}
