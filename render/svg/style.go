package svg

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// writeStyleBlock emits the <style>...</style> block at the top of
// the SVG. Prefers the theme's pre-rendered CSS string (populated by
// theme.Theme.CSSVariables in the encoder); falls back to the
// hardcoded block when the theme carries no CSS (back-compat with
// scene.Default()).
func writeStyleBlock(w *Writer, theme *scene.Theme) {
	if theme == nil {
		theme = scene.Default()
	}
	if theme.CSS != "" {
		// Indent + emit the prebuilt CSS verbatim. The encoder ensures
		// the string already includes the <style>...</style> wrapper.
		w.Raw("  ")
		w.Raw(theme.CSS)
		w.Newline()
		return
	}
	w.Raw("  <style>")
	w.Raw(":root{")
	if theme.ColorAxis != nil {
		fmt.Fprintf(w.buf, "--prism-color-axis:%s;", theme.ColorAxis.CSS())
	}
	if theme.ColorGrid != nil {
		fmt.Fprintf(w.buf, "--prism-color-grid:%s;", theme.ColorGrid.CSS())
	}
	if theme.ColorText != nil {
		fmt.Fprintf(w.buf, "--prism-color-text:%s;", theme.ColorText.CSS())
	}
	if theme.FontSans != "" {
		fmt.Fprintf(w.buf, "--prism-font-sans:%s;", theme.FontSans)
	}
	if theme.FontMono != "" {
		fmt.Fprintf(w.buf, "--prism-font-mono:%s;", theme.FontMono)
	}
	w.Raw("}")
	w.Raw(".prism-axis-domain{stroke:var(--prism-color-axis);fill:none;}")
	w.Raw(".prism-axis-tick{stroke:var(--prism-color-axis);}")
	w.Raw(".prism-axis-label{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:11px;}")
	w.Raw(".prism-axis-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:12px;font-weight:600;}")
	w.Raw(".prism-grid-line{stroke:var(--prism-color-grid);}")
	w.Raw(".prism-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:16px;font-weight:600;}")
	w.Raw(".prism-legend-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:12px;font-weight:600;}")
	w.Raw(".prism-legend-label{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:11px;}")
	w.Raw(".prism-legend-swatch{stroke:none;}")
	// Selection defaults (D078) — kept in lock-step with theme/css.go.
	w.Raw(".prism-selected{opacity:var(--prism-selected-opacity,1);}")
	w.Raw(".prism-deselected{opacity:var(--prism-deselected-opacity,0.3);}")
	w.Raw("</style>")
	w.Newline()
}

// writeStyleAttrs renders the per-mark Style on an element. Caller
// has already opened the tag; writeStyleAttrs appends fill, stroke,
// stroke-width, opacity attributes (omitting unset / default values).
func writeStyleAttrs(w *Writer, s scene.Style) {
	if s.Fill != nil {
		w.Attr("fill", s.Fill.CSS())
	}
	if s.Stroke != nil {
		w.Attr("stroke", s.Stroke.CSS())
	}
	if s.StrokeWidth > 0 {
		w.AttrFloat("stroke-width", s.StrokeWidth)
	}
	if s.Opacity > 0 && s.Opacity < 1 {
		w.AttrFloat("opacity", s.Opacity)
	}
}
