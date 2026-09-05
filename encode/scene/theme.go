package scene

// Theme is the placeholder theme struct shipped in P05. P06 expands
// this into a registry with light / dark / print variants + sparse
// spec-level overrides (D009). The CSS-variable manifest emitted by
// render/svg/style.go is derived from this struct so the P06 swap is
// interface-clean.
type Theme struct {
	Name       string `json:"name,omitempty"`
	ColorAxis  *Color `json:"color_axis,omitempty"`
	ColorGrid  *Color `json:"color_grid,omitempty"`
	ColorText  *Color `json:"color_text,omitempty"`
	Background string `json:"background,omitempty"`
	FontSans   string `json:"font_sans,omitempty"`
	FontMono   string `json:"font_mono,omitempty"`
	// CSS carries the pre-rendered <style> block produced by the
	// theme package. The renderer emits this verbatim when set, and
	// falls back to a hardcoded block when empty (back-compat with
	// scene.Default()). Serialised to JSON so the JS port (prism.mjs)
	// receives the same theme bytes the Go renderer emits — required
	// for cross-impl parity (D075 + D076).
	CSS string `json:"css,omitempty"`
	// Filters mirrors theme.Theme.Filters — raw SVG <filter>
	// inner-content bodies keyed by theme-author-chosen name. The SVG
	// renderer emits one <filter id="prism-filter-<name>"> element
	// per entry (render/svg/style.go's writeFilterDefs). Populated by
	// theme.Theme.ToSceneTheme.
	Filters map[string]string `json:"filters,omitempty"`
	// AxisFilter/LegendFilter/TitleFilter/ViewFilter carry the
	// resolved theme.Axis/Legend/Title/View block's Filter name (see
	// theme.AxisStyle.Filter etc.). The renderer applies
	// filter="url(#prism-filter-<name>)" to the corresponding
	// structural group/element (the <g class="prism-axes"> wrapper,
	// the <g class="prism-legends"> wrapper, the title <text>, and
	// the view background <rect>) when non-empty.
	AxisFilter   string `json:"axis_filter,omitempty"`
	LegendFilter string `json:"legend_filter,omitempty"`
	TitleFilter  string `json:"title_filter,omitempty"`
	ViewFilter   string `json:"view_filter,omitempty"`
}

// Default returns the hard-coded P05 theme:
//   - axis: #6b7280 (gray-500)
//   - grid: #e5e7eb (gray-200)
//   - text: #111827 (gray-900)
//   - sans: Inter, system-ui, sans-serif
//   - mono: ui-monospace, SF Mono, monospace
//   - background: transparent
//
// Hex strings are guaranteed-valid by tests; the function never
// returns nil for any color pointer.
func Default() *Theme {
	axis, _ := ColorFromHex("#6b7280")
	grid, _ := ColorFromHex("#e5e7eb")
	text, _ := ColorFromHex("#111827")
	return &Theme{
		ColorAxis:  axis,
		ColorGrid:  grid,
		ColorText:  text,
		Background: "transparent",
		FontSans:   "Inter, system-ui, sans-serif",
		FontMono:   "ui-monospace, SF Mono, monospace",
	}
}
