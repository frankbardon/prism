package theme

import (
	"fmt"
	"strings"
)

// CSSVariables returns the <style>...</style> block that the SVG
// renderer embeds at the top of every output document. Format mirrors
// design/07-rendering.md § Theming via CSS variables.
//
// Output shape:
//
//	<style>:root{
//	  --prism-color-axis:#...;
//	  --prism-color-grid:#...;
//	  ...
//	}
//	.prism-axis-domain { ... }
//	.prism-grid-line   { ... }
//	...
//	</style>
//
// The CSS class set is fixed; theme values populate via var().
func (t *Theme) CSSVariables() string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("<style>")
	b.WriteString(":root{")
	if t.AxisColor != "" {
		fmt.Fprintf(&b, "--prism-color-axis:%s;", t.AxisColor)
	}
	if t.GridColor != "" {
		fmt.Fprintf(&b, "--prism-color-grid:%s;", t.GridColor)
	}
	if t.TextColor != "" {
		fmt.Fprintf(&b, "--prism-color-text:%s;", t.TextColor)
	}
	if t.BackgroundColor != "" {
		fmt.Fprintf(&b, "--prism-color-bg:%s;", t.BackgroundColor)
	}
	if t.FontSans != "" {
		fmt.Fprintf(&b, "--prism-font-sans:%s;", t.FontSans)
	}
	if t.FontMono != "" {
		fmt.Fprintf(&b, "--prism-font-mono:%s;", t.FontMono)
	}
	if t.FontSizeLabel != 0 {
		fmt.Fprintf(&b, "--prism-font-size-label:%gpx;", t.FontSizeLabel)
	}
	if t.FontSizeTitle != 0 {
		fmt.Fprintf(&b, "--prism-font-size-title:%gpx;", t.FontSizeTitle)
	}
	if t.FontSizeAxisTitle != 0 {
		fmt.Fprintf(&b, "--prism-font-size-axis-title:%gpx;", t.FontSizeAxisTitle)
	}
	b.WriteString("}")
	// Class selectors driven by the variables above.
	b.WriteString(".prism-axis-domain{stroke:var(--prism-color-axis);fill:none;}")
	b.WriteString(".prism-axis-tick{stroke:var(--prism-color-axis);}")
	b.WriteString(".prism-axis-label{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:var(--prism-font-size-label,11px);}")
	b.WriteString(".prism-axis-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:var(--prism-font-size-axis-title,12px);font-weight:600;}")
	b.WriteString(".prism-grid-line{stroke:var(--prism-color-grid);}")
	b.WriteString(".prism-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:var(--prism-font-size-title,16px);font-weight:600;}")
	b.WriteString(".prism-legend-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:12px;font-weight:600;}")
	b.WriteString(".prism-legend-label{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:11px;}")
	b.WriteString(".prism-legend-swatch{stroke:none;}")
	// Selection-driven CSS classes (D078). Theme authors override via
	// the documented CSS variables; defaults dim deselected marks.
	b.WriteString(".prism-selected{opacity:var(--prism-selected-opacity,1);}")
	b.WriteString(".prism-deselected{opacity:var(--prism-deselected-opacity,0.3);}")
	b.WriteString("</style>")
	return b.String()
}
