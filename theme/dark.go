package theme

// darkTheme inverts the light palette: dark background, light text,
// brighter primary mark colors for contrast.
func darkTheme() *Theme {
	return &Theme{
		AxisColor:         "#9ca3af", // gray-400
		GridColor:         "#374151", // gray-700
		TextColor:         "#f3f4f6", // gray-100
		BackgroundColor:   "#0b1220", // slate-950-ish
		FontSans:          "Inter, system-ui, sans-serif",
		FontMono:          "ui-monospace, SF Mono, monospace",
		FontSizeLabel:     11,
		FontSizeTitle:     16,
		FontSizeAxisTitle: 12,
		ColorSchemeCategorical: []string{
			"#60a5fa", "#f87171", "#34d399", "#fbbf24",
			"#a78bfa", "#f472b6", "#2dd4bf", "#c084fc",
		},
		ColorSchemeSequential: []string{
			"#082f49", "#0c4a6e", "#075985", "#0369a1",
			"#0284c7", "#0ea5e9", "#38bdf8", "#7dd3fc",
		},
	}
}
