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
	// Gradients/Patterns mirror theme.Theme.Gradients/Patterns,
	// pre-resolved to SVG-emission-ready shape by
	// theme.Theme.ToSceneTheme (angle-to-vector and center/radius math
	// already done for gradients; Spacing/Size defaults already
	// resolved for patterns). The renderer emits one
	// <linearGradient>/<radialGradient id="prism-gradient-<name>">
	// def per Gradients entry and one <pattern id="prism-pattern-<name>">
	// def per Patterns entry (render/svg/style.go's
	// writeGradientPatternDefs), regardless of whether a Style
	// actually references it that render pass — same "emit the whole
	// registry" convention as Filters.
	Gradients map[string]Gradient `json:"gradients,omitempty"`
	Patterns  map[string]Pattern  `json:"patterns,omitempty"`
	// ViewBackgroundRef carries the resolved theme.ViewStyle.Background
	// url(#name) reference (see theme.Theme.ResolveFillRef), already
	// prefixed as "prism-gradient-<name>" or "prism-pattern-<name>".
	// Empty means the background is unset or a plain literal color
	// (theme.ViewStyle.Background carries no rendered fill today for
	// the literal-color case — see render/svg/renderer.go's
	// renderScene). The renderer applies
	// fill="url(#<ViewBackgroundRef>)" to the view background rect
	// when non-empty.
	ViewBackgroundRef string `json:"view_background_ref,omitempty"`
	// AxisLabelLineHeight/AxisLabelLetterSpacing and
	// AxisTitleLineHeight/AxisTitleLetterSpacing carry the resolved
	// theme.AxisStyle line_height/letter_spacing tokens (E2-S2),
	// LegendLabelLineHeight/LegendLabelLetterSpacing and
	// LegendTitleLineHeight/LegendTitleLetterSpacing carry the
	// theme.LegendStyle equivalents, and TitleLineHeight/
	// TitleLetterSpacing carry theme.TitleStyle's. These structural
	// blocks don't cascade to a per-element scene.Style the way
	// Mark/Marks do (see scene.Style.LineHeight/LetterSpacing +
	// encode.applyThemeMarkStyle), so the resolved values ride on
	// scene.Theme instead — mirrors the AxisFilter/LegendFilter/
	// TitleFilter pattern above. The renderer applies LetterSpacing
	// as a `letter-spacing` presentation attribute and LineHeight as
	// a `style="line-height:…"` declaration on the corresponding
	// tick-label / axis-title / legend-label / legend-title / chart
	// title <text> element when non-nil.
	AxisLabelLineHeight      *float64 `json:"axis_label_line_height,omitempty"`
	AxisLabelLetterSpacing   *float64 `json:"axis_label_letter_spacing,omitempty"`
	AxisTitleLineHeight      *float64 `json:"axis_title_line_height,omitempty"`
	AxisTitleLetterSpacing   *float64 `json:"axis_title_letter_spacing,omitempty"`
	LegendLabelLineHeight    *float64 `json:"legend_label_line_height,omitempty"`
	LegendLabelLetterSpacing *float64 `json:"legend_label_letter_spacing,omitempty"`
	LegendTitleLineHeight    *float64 `json:"legend_title_line_height,omitempty"`
	LegendTitleLetterSpacing *float64 `json:"legend_title_letter_spacing,omitempty"`
	TitleLineHeight          *float64 `json:"title_line_height,omitempty"`
	TitleLetterSpacing       *float64 `json:"title_letter_spacing,omitempty"`
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
