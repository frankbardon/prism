package svg

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
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
	writeTypographyAttrs(w, s.LineHeight, s.LetterSpacing)
	writeFilterAttr(w, s.Filter)
}

// writeTypographyAttrs applies the E2-S2 line-height / letter-spacing
// typography tokens to a text-bearing element. letterSpacing emits as
// the `letter-spacing` SVG presentation attribute (unitless numbers
// are valid user-unit lengths, same convention as the existing
// font-size attr). lineHeight has no SVG presentation-attribute
// equivalent, so it emits as a `style="line-height:…"` CSS
// declaration instead — a unitless multiplier, matching CSS
// line-height semantics. Both are no-ops when nil (unset), which
// keeps every existing element byte-identical until a theme actually
// sets one of these tokens. Shared by writeStyleAttrs (mark/text-mark
// glyphs) and the structural axis/legend/title text emitters, which
// call it directly since those elements carry no scene.Style of
// their own — see scene.Theme's AxisLabelLineHeight family.
func writeTypographyAttrs(w *Writer, lineHeight, letterSpacing *float64) {
	if letterSpacing != nil {
		w.AttrFloat("letter-spacing", *letterSpacing)
	}
	if lineHeight != nil {
		w.Attr("style", "line-height:"+render.FormatFloat(*lineHeight))
	}
}

// writeFilterAttr emits filter="url(#prism-filter-<name>)" when name
// is non-empty. Shared by mark elements (writeStyleAttrs) and the
// structural axis/legend/title/view elements (renderer.go), keeping
// the id-naming convention (E1-S2 / prism-filter-<name>) in one
// place.
func writeFilterAttr(w *Writer, name string) {
	if name == "" {
		return
	}
	w.Attr("filter", "url(#prism-filter-"+name+")")
}

// writeFilterDefs emits one <filter id="prism-filter-<name>"> element
// per entry in theme.Filters, wrapping the raw body verbatim (the
// theme package validates every Filter reference resolves at load
// time — see theme/validate.go — so by encode time this map already
// contains every name any style block could reference). Names are
// sorted for deterministic golden bytes across runs. Emits nothing
// when the theme carries no filters.
func writeFilterDefs(w *Writer, theme *scene.Theme) {
	if theme == nil || len(theme.Filters) == 0 {
		return
	}
	names := make([]string, 0, len(theme.Filters))
	for name := range theme.Filters {
		names = append(names, name)
	}
	sort.Strings(names)
	w.Raw("  <defs>")
	for _, name := range names {
		w.OpenTag("filter")
		w.Attr("id", "prism-filter-"+name)
		w.CloseTagOpen()
		w.Raw(theme.Filters[name])
		w.EndTag("filter")
	}
	w.Raw("</defs>")
	w.Newline()
}
