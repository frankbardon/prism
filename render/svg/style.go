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
	if s.FillRef != "" {
		w.Attr("fill", "url(#"+s.FillRef+")")
	} else if s.FillVar != "" {
		// Auto-dark resolved color (E4-S3): both light and dark hex
		// values are registered under this custom property in the
		// <style> block (theme.CSSVariables) — see scene.Style.FillVar.
		w.Attr("fill", "var(--"+s.FillVar+")")
	} else if s.Fill != nil {
		w.Attr("fill", s.Fill.CSS())
	}
	if s.StrokeRef != "" {
		w.Attr("stroke", "url(#"+s.StrokeRef+")")
	} else if s.StrokeVar != "" {
		w.Attr("stroke", "var(--"+s.StrokeVar+")")
	} else if s.Stroke != nil {
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

// writeGradientPatternDefs emits one <linearGradient>/<radialGradient>
// def per entry in theme.Gradients and one <pattern> def per entry in
// theme.Patterns (E3-S3). By encode time both maps already carry
// every entry the theme declared (theme.Theme.ToSceneTheme passes the
// whole registry through, same convention as Filters/writeFilterDefs)
// — Style.FillRef/StrokeRef and Theme.ViewBackgroundRef reference a
// subset of these ids. Names are sorted independently within each
// registry for deterministic golden bytes across runs. Emits nothing
// when the theme carries neither.
func writeGradientPatternDefs(w *Writer, theme *scene.Theme) {
	if theme == nil || (len(theme.Gradients) == 0 && len(theme.Patterns) == 0) {
		return
	}
	gradientNames := make([]string, 0, len(theme.Gradients))
	for name := range theme.Gradients {
		gradientNames = append(gradientNames, name)
	}
	sort.Strings(gradientNames)
	patternNames := make([]string, 0, len(theme.Patterns))
	for name := range theme.Patterns {
		patternNames = append(patternNames, name)
	}
	sort.Strings(patternNames)

	w.Raw("  <defs>")
	for _, name := range gradientNames {
		writeGradientDef(w, "prism-gradient-"+name, theme.Gradients[name])
	}
	for _, name := range patternNames {
		writePatternDef(w, "prism-pattern-"+name, theme.Patterns[name])
	}
	w.Raw("</defs>")
	w.Newline()
}

// writeGradientDef emits one <linearGradient>/<radialGradient id=id>
// element with ordered <stop> children. g.Type == "radial" reads
// X1/Y1/X2 as cx/cy/r (see scene.Gradient's doc comment); anything
// else is treated as linear (x1/y1/x2/y2). Coordinates route through
// AttrFloat (render.FormatFloat) for pinned precision per CLAUDE.md's
// "Pinned coordinate precision" rule.
func writeGradientDef(w *Writer, id string, g scene.Gradient) {
	tag := "linearGradient"
	if g.Type == "radial" {
		tag = "radialGradient"
	}
	w.OpenTag(tag)
	w.Attr("id", id)
	if g.Type == "radial" {
		w.AttrFloat("cx", g.X1)
		w.AttrFloat("cy", g.Y1)
		w.AttrFloat("r", g.X2)
	} else {
		w.AttrFloat("x1", g.X1)
		w.AttrFloat("y1", g.Y1)
		w.AttrFloat("x2", g.X2)
		w.AttrFloat("y2", g.Y2)
	}
	w.CloseTagOpen()
	for _, s := range g.Stops {
		w.OpenTag("stop")
		w.AttrFloat("offset", s.Offset)
		w.Attr("stop-color", (&s.Color).CSS())
		w.SelfClose()
	}
	w.EndTag(tag)
}

// writePatternDef emits one <pattern id=id> element, tile-sized by
// p.Spacing (patternUnits="userSpaceOnUse" — Spacing/Size are literal
// user-space pixels, not bounding-box fractions, since a pattern tile
// should stay a fixed physical size regardless of the shape it
// fills). Built-in catalogue types (theme.PatternTypes) render
// generated content tuned by p.Color/p.Spacing/p.Size via the four
// writeBuiltinPattern* helpers below; p.Type == "" (raw content)
// emits p.Content verbatim inside the wrapper instead — same trust
// tier as the theme.Filters/RawCSS escape hatches (developer-
// authored, never sanitized).
func writePatternDef(w *Writer, id string, p scene.Pattern) {
	w.OpenTag("pattern")
	w.Attr("id", id)
	w.Attr("patternUnits", "userSpaceOnUse")
	w.AttrFloat("width", p.Spacing)
	w.AttrFloat("height", p.Spacing)
	if p.Type == "diagonal-stripes" {
		// Draw vertical stripes at native orientation, then rotate the
		// whole tile 45deg so they read as diagonal.
		w.Attr("patternTransform", "rotate(45)")
	}
	w.CloseTagOpen()
	switch p.Type {
	case "diagonal-stripes":
		writeBuiltinPatternDiagonalStripes(w, p)
	case "dots":
		writeBuiltinPatternDots(w, p)
	case "cross-hatch":
		writeBuiltinPatternCrossHatch(w, p)
	case "grid":
		writeBuiltinPatternGrid(w, p)
	default:
		// Raw content — p.Type == "".
		w.Raw(p.Content)
	}
	w.EndTag("pattern")
}

// writeBuiltinPatternDiagonalStripes draws one solid stripe of width
// p.Size per tile (tile pitch p.Spacing); the enclosing <pattern>'s
// patternTransform="rotate(45)" turns it diagonal.
func writeBuiltinPatternDiagonalStripes(w *Writer, p scene.Pattern) {
	w.OpenTag("rect")
	w.AttrFloat("width", p.Size)
	w.AttrFloat("height", p.Spacing)
	w.Attr("fill", p.Color)
	w.SelfClose()
}

// writeBuiltinPatternDots draws one centered dot of diameter p.Size
// per tile.
func writeBuiltinPatternDots(w *Writer, p scene.Pattern) {
	w.OpenTag("circle")
	w.AttrFloat("cx", p.Spacing/2)
	w.AttrFloat("cy", p.Spacing/2)
	w.AttrFloat("r", p.Size/2)
	w.Attr("fill", p.Color)
	w.SelfClose()
}

// writeBuiltinPatternCrossHatch draws an X across the tile (two
// crossing diagonals), stroked at width p.Size.
func writeBuiltinPatternCrossHatch(w *Writer, p scene.Pattern) {
	w.OpenTag("path")
	s := render.FormatFloat(p.Spacing)
	w.OpenAttr("d")
	w.Raw("M0,0L" + s + "," + s + "M" + s + ",0L0," + s)
	w.CloseAttr()
	w.Attr("fill", "none")
	w.Attr("stroke", p.Color)
	w.AttrFloat("stroke-width", p.Size)
	w.SelfClose()
}

// writeBuiltinPatternGrid draws the top and left edges of the tile
// (which tile into a full lattice), stroked at width p.Size.
func writeBuiltinPatternGrid(w *Writer, p scene.Pattern) {
	w.OpenTag("path")
	s := render.FormatFloat(p.Spacing)
	w.OpenAttr("d")
	w.Raw("M" + s + ",0L0,0L0," + s)
	w.CloseAttr()
	w.Attr("fill", "none")
	w.Attr("stroke", p.Color)
	w.AttrFloat("stroke-width", p.Size)
	w.SelfClose()
}
