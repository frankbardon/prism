package theme

// printTheme: neutral grayscale, no saturated fills, hatch-friendly.
// Targets monochrome print output without dynamic background fills.
func printTheme() *Theme {
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
	}
}
