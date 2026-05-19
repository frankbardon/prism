package theme

// lightTheme matches the P05 hardcoded palette so the golden diff
// when T06.11 swaps the renderer's CSS source from scene.Default to
// theme.Light is limited to additive class entries.
func lightTheme() *Theme {
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
			"#3b82f6", "#ef4444", "#10b981", "#f59e0b",
			"#8b5cf6", "#ec4899", "#14b8a6", "#a855f7",
		},
		ColorSchemeSequential: []string{
			"#f0f9ff", "#bae6fd", "#7dd3fc", "#38bdf8",
			"#0ea5e9", "#0284c7", "#0369a1", "#075985",
		},
	}
}
