package theme

import (
	"fmt"
	"sort"
	"strings"
)

// CSSVariables returns the <style>...</style> block that the SVG
// renderer embeds at the top of every output document. Format
// mirrors design/07-rendering.md § Theming via CSS variables.
//
// Output shape:
//
//	<style>:root{
//	  --prism-color-axis:#...;
//	  --prism-color-grid:#...;
//	  --prism-mark-bar-fill:#...;
//	  --prism-axis-tick-size:5px;
//	  ...
//	}
//	.prism-axis-domain { ... }
//	.prism-grid-line   { ... }
//	.prism-mark-bar    { ... }
//	...
//	</style>
//
// Variable categories:
//   - --prism-color-*       core palette (axis/grid/text/bg)
//   - --prism-font-*        typography
//   - --prism-axis-*        axis tokens
//   - --prism-grid-*        grid tokens
//   - --prism-legend-*      legend tokens
//   - --prism-title-*       title tokens
//   - --prism-view-*        chart-rect tokens
//   - --prism-mark-<type>-* per-mark defaults
//   - --prism-selected-*    selection state
//   - --prism-deselected-*  selection state
//
// The class set is fixed; theme values populate via var().
func (t *Theme) CSSVariables() string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("<style>")
	b.WriteString(":root{")
	writeRootVars(&b, t)
	b.WriteString("}")
	writeClassSelectors(&b)
	b.WriteString("</style>")
	return b.String()
}

func writeRootVars(b *strings.Builder, t *Theme) {
	// Legacy flat tokens (preserved for back-compat with prism.mjs).
	if t.AxisColor != "" {
		fmt.Fprintf(b, "--prism-color-axis:%s;", t.AxisColor)
	}
	if t.GridColor != "" {
		fmt.Fprintf(b, "--prism-color-grid:%s;", t.GridColor)
	}
	if t.TextColor != "" {
		fmt.Fprintf(b, "--prism-color-text:%s;", t.TextColor)
	}
	if t.BackgroundColor != "" {
		fmt.Fprintf(b, "--prism-color-bg:%s;", t.BackgroundColor)
	}
	if t.FontSans != "" {
		fmt.Fprintf(b, "--prism-font-sans:%s;", t.FontSans)
	}
	if t.FontMono != "" {
		fmt.Fprintf(b, "--prism-font-mono:%s;", t.FontMono)
	}
	if t.FontSizeLabel != 0 {
		fmt.Fprintf(b, "--prism-font-size-label:%gpx;", t.FontSizeLabel)
	}
	if t.FontSizeTitle != 0 {
		fmt.Fprintf(b, "--prism-font-size-title:%gpx;", t.FontSizeTitle)
	}
	if t.FontSizeAxisTitle != 0 {
		fmt.Fprintf(b, "--prism-font-size-axis-title:%gpx;", t.FontSizeAxisTitle)
	}

	// v2 nested tokens.
	writeAxisVars(b, t.Axis)
	writeLegendVars(b, t.Legend)
	writeTitleVars(b, t.Title)
	writeViewVars(b, t.View)
	writeMarkVars(b, "mark", t.Mark)
	// Marks rendered in sorted-name order so the CSS bytes are
	// deterministic across runs.
	if t.Marks != nil {
		names := make([]string, 0, len(t.Marks))
		for k := range t.Marks {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, name := range names {
			writeMarkVars(b, "mark-"+name, t.Marks[name])
		}
	}
	writeStateVars(b, t.States)
}

func writeAxisVars(b *strings.Builder, a *AxisStyle) {
	if a == nil {
		return
	}
	if a.DomainColor != "" {
		fmt.Fprintf(b, "--prism-axis-domain-color:%s;", a.DomainColor)
	}
	if a.DomainWidth != nil {
		fmt.Fprintf(b, "--prism-axis-domain-width:%gpx;", *a.DomainWidth)
	}
	if a.TickColor != "" {
		fmt.Fprintf(b, "--prism-axis-tick-color:%s;", a.TickColor)
	}
	if a.TickWidth != nil {
		fmt.Fprintf(b, "--prism-axis-tick-width:%gpx;", *a.TickWidth)
	}
	if a.TickSize != nil {
		fmt.Fprintf(b, "--prism-axis-tick-size:%gpx;", *a.TickSize)
	}
	if a.TickOpacity != nil {
		fmt.Fprintf(b, "--prism-axis-tick-opacity:%g;", *a.TickOpacity)
	}
	if a.GridColor != "" {
		fmt.Fprintf(b, "--prism-grid-color:%s;", a.GridColor)
	}
	if a.GridWidth != nil {
		fmt.Fprintf(b, "--prism-grid-width:%gpx;", *a.GridWidth)
	}
	if a.GridOpacity != nil {
		fmt.Fprintf(b, "--prism-grid-opacity:%g;", *a.GridOpacity)
	}
	if len(a.GridDash) > 0 {
		fmt.Fprintf(b, "--prism-grid-dash:%s;", dashString(a.GridDash))
	}
	if a.LabelColor != "" {
		fmt.Fprintf(b, "--prism-axis-label-color:%s;", a.LabelColor)
	}
	if a.LabelFontSize != nil {
		fmt.Fprintf(b, "--prism-axis-label-font-size:%gpx;", *a.LabelFontSize)
	}
	if a.LabelFontWeight != "" {
		fmt.Fprintf(b, "--prism-axis-label-font-weight:%s;", a.LabelFontWeight)
	}
	if a.LabelPadding != nil {
		fmt.Fprintf(b, "--prism-axis-label-padding:%gpx;", *a.LabelPadding)
	}
	if a.TitleColor != "" {
		fmt.Fprintf(b, "--prism-axis-title-color:%s;", a.TitleColor)
	}
	if a.TitleFontSize != nil {
		fmt.Fprintf(b, "--prism-axis-title-font-size:%gpx;", *a.TitleFontSize)
	}
	if a.TitleFontWeight != "" {
		fmt.Fprintf(b, "--prism-axis-title-font-weight:%s;", a.TitleFontWeight)
	}
	if a.TitlePadding != nil {
		fmt.Fprintf(b, "--prism-axis-title-padding:%gpx;", *a.TitlePadding)
	}
}

func writeLegendVars(b *strings.Builder, l *LegendStyle) {
	if l == nil {
		return
	}
	if l.FillColor != "" {
		fmt.Fprintf(b, "--prism-legend-fill:%s;", l.FillColor)
	}
	if l.StrokeColor != "" {
		fmt.Fprintf(b, "--prism-legend-stroke:%s;", l.StrokeColor)
	}
	if l.StrokeWidth != nil {
		fmt.Fprintf(b, "--prism-legend-stroke-width:%gpx;", *l.StrokeWidth)
	}
	if l.Padding != nil {
		fmt.Fprintf(b, "--prism-legend-padding:%gpx;", *l.Padding)
	}
	if l.SymbolSize != nil {
		fmt.Fprintf(b, "--prism-legend-symbol-size:%gpx;", *l.SymbolSize)
	}
	if l.SymbolStrokeWidth != nil {
		fmt.Fprintf(b, "--prism-legend-symbol-stroke-width:%gpx;", *l.SymbolStrokeWidth)
	}
	if l.LabelColor != "" {
		fmt.Fprintf(b, "--prism-legend-label-color:%s;", l.LabelColor)
	}
	if l.LabelFontSize != nil {
		fmt.Fprintf(b, "--prism-legend-label-font-size:%gpx;", *l.LabelFontSize)
	}
	if l.TitleColor != "" {
		fmt.Fprintf(b, "--prism-legend-title-color:%s;", l.TitleColor)
	}
	if l.TitleFontSize != nil {
		fmt.Fprintf(b, "--prism-legend-title-font-size:%gpx;", *l.TitleFontSize)
	}
	if l.TitleFontWeight != "" {
		fmt.Fprintf(b, "--prism-legend-title-font-weight:%s;", l.TitleFontWeight)
	}
	if l.RowPadding != nil {
		fmt.Fprintf(b, "--prism-legend-row-padding:%gpx;", *l.RowPadding)
	}
	if l.ColumnPadding != nil {
		fmt.Fprintf(b, "--prism-legend-column-padding:%gpx;", *l.ColumnPadding)
	}
}

func writeTitleVars(b *strings.Builder, t *TitleStyle) {
	if t == nil {
		return
	}
	if t.Color != "" {
		fmt.Fprintf(b, "--prism-title-color:%s;", t.Color)
	}
	if t.FontSize != nil {
		fmt.Fprintf(b, "--prism-title-font-size:%gpx;", *t.FontSize)
	}
	if t.FontWeight != "" {
		fmt.Fprintf(b, "--prism-title-font-weight:%s;", t.FontWeight)
	}
	if t.Align != "" {
		fmt.Fprintf(b, "--prism-title-align:%s;", t.Align)
	}
	if t.Anchor != "" {
		fmt.Fprintf(b, "--prism-title-anchor:%s;", t.Anchor)
	}
	if t.Padding != nil {
		fmt.Fprintf(b, "--prism-title-padding:%gpx;", *t.Padding)
	}
}

func writeViewVars(b *strings.Builder, v *ViewStyle) {
	if v == nil {
		return
	}
	if v.Background != "" {
		fmt.Fprintf(b, "--prism-view-bg:%s;", v.Background)
	}
	if v.Stroke != "" {
		fmt.Fprintf(b, "--prism-view-stroke:%s;", v.Stroke)
	}
	if v.StrokeWidth != nil {
		fmt.Fprintf(b, "--prism-view-stroke-width:%gpx;", *v.StrokeWidth)
	}
	if v.Padding != nil {
		fmt.Fprintf(b, "--prism-view-padding:%gpx;", *v.Padding)
	}
	if v.CornerRadius != nil {
		fmt.Fprintf(b, "--prism-view-corner-radius:%gpx;", *v.CornerRadius)
	}
}

func writeMarkVars(b *strings.Builder, prefix string, m *MarkStyle) {
	if m == nil {
		return
	}
	p := "--prism-" + prefix
	if m.Fill != "" {
		fmt.Fprintf(b, "%s-fill:%s;", p, m.Fill)
	}
	if m.Stroke != "" {
		fmt.Fprintf(b, "%s-stroke:%s;", p, m.Stroke)
	}
	if m.StrokeWidth != nil {
		fmt.Fprintf(b, "%s-stroke-width:%gpx;", p, *m.StrokeWidth)
	}
	if m.Opacity != nil {
		fmt.Fprintf(b, "%s-opacity:%g;", p, *m.Opacity)
	}
	if m.FillOpacity != nil {
		fmt.Fprintf(b, "%s-fill-opacity:%g;", p, *m.FillOpacity)
	}
	if m.CornerRadius != nil {
		fmt.Fprintf(b, "%s-corner-radius:%gpx;", p, *m.CornerRadius)
	}
	if m.Size != nil {
		fmt.Fprintf(b, "%s-size:%g;", p, *m.Size)
	}
	if m.FontSize != nil {
		fmt.Fprintf(b, "%s-font-size:%gpx;", p, *m.FontSize)
	}
	if m.FontWeight != "" {
		fmt.Fprintf(b, "%s-font-weight:%s;", p, m.FontWeight)
	}
}

func writeStateVars(b *strings.Builder, states map[string]*StateStyle) {
	if states == nil {
		return
	}
	names := make([]string, 0, len(states))
	for k := range states {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		s := states[name]
		if s == nil {
			continue
		}
		p := "--prism-" + name
		if s.Opacity != nil {
			fmt.Fprintf(b, "%s-opacity:%g;", p, *s.Opacity)
		}
		if s.StrokeWidth != nil {
			fmt.Fprintf(b, "%s-stroke-width:%gpx;", p, *s.StrokeWidth)
		}
		if s.Stroke != "" {
			fmt.Fprintf(b, "%s-stroke:%s;", p, s.Stroke)
		}
		if s.Fill != "" {
			fmt.Fprintf(b, "%s-fill:%s;", p, s.Fill)
		}
	}
}

func writeClassSelectors(b *strings.Builder) {
	b.WriteString(".prism-axis-domain{stroke:var(--prism-axis-domain-color,var(--prism-color-axis));stroke-width:var(--prism-axis-domain-width,1px);fill:none;}")
	b.WriteString(".prism-axis-tick{stroke:var(--prism-axis-tick-color,var(--prism-color-axis));stroke-width:var(--prism-axis-tick-width,1px);}")
	b.WriteString(".prism-axis-label{fill:var(--prism-axis-label-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-axis-label-font-size,var(--prism-font-size-label,11px));font-weight:var(--prism-axis-label-font-weight,400);}")
	b.WriteString(".prism-axis-title{fill:var(--prism-axis-title-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-axis-title-font-size,var(--prism-font-size-axis-title,12px));font-weight:var(--prism-axis-title-font-weight,600);}")
	b.WriteString(".prism-grid-line{stroke:var(--prism-grid-color,var(--prism-color-grid));stroke-width:var(--prism-grid-width,1px);}")
	b.WriteString(".prism-title{fill:var(--prism-title-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-title-font-size,var(--prism-font-size-title,16px));font-weight:var(--prism-title-font-weight,600);}")
	b.WriteString(".prism-legend-title{fill:var(--prism-legend-title-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-legend-title-font-size,12px);font-weight:var(--prism-legend-title-font-weight,600);}")
	b.WriteString(".prism-legend-label{fill:var(--prism-legend-label-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-legend-label-font-size,11px);}")
	b.WriteString(".prism-legend-swatch{stroke:none;}")
	b.WriteString(".prism-selected{opacity:var(--prism-selected-opacity,1);}")
	b.WriteString(".prism-deselected{opacity:var(--prism-deselected-opacity,0.3);}")
}

func dashString(stops []float64) string {
	parts := make([]string, len(stops))
	for i, v := range stops {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return strings.Join(parts, ",")
}
