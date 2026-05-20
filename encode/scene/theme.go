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
