package theme

import (
	"fmt"
	"sort"
	"strings"
)

// CSSVariables returns the <style>...</style> block that the SVG
// renderer embeds at the top of every output document.
//
// Output shape:
//
//	<style>:root{
//	  --prism-color-axis:#...;
//	  --prism-color-grid:#...;
//	  ...
//	}
//	<dark-mode override rules, when a companion base is set>
//	.prism-axis-domain { ... }
//	.prism-grid-line   { ... }
//	...
//	</style>
//
// The `:root{` prefix is load-bearing for consumers: Arc rewrites
// that exact selector to scope the token block per chart, and
// per-organisation theming swaps the values inside it. Never emit a
// second `:root{` — the dark companion block below intentionally uses
// compound selectors so the rewrite has exactly one target.
//
// Variable categories:
//   - --prism-color-*       core palette (axis/grid/text/text-muted/bg)
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
	writeCompanionVars(&b, t)
	writeClassSelectors(&b)
	b.WriteString("</style>")
	return b.String()
}

// CompanionDark names the theme whose token values are emitted as the
// dark-mode override block alongside this theme's own.
//
// This is the coupling between a chart and its host's light/dark
// state. A rendered SVG carries BOTH token sets, so a host that flips
// theme repaints the chart through CSS alone — no re-render, no round
// trip, and a chart already streamed into a chat transcript flips with
// everything around it.
//
// The alternative — a theme hint on the render request — makes the
// host's theme part of the chart's identity, which means every cached
// SVG is cached under the wrong one half the time.
//
// Only the light-family bases carry a companion. A theme explicitly
// chosen as `dark` is an explicit choice that must not be re-flipped
// by the host, and `print` has no dark mode by definition.
func (t *Theme) CompanionDark() string {
	if t == nil {
		return ""
	}
	switch t.Name {
	case "light":
		return "dark"
	case "high_contrast":
		return "high_contrast_dark"
	case "colorblind":
		return "colorblind_dark"
	}
	return ""
}

// darkScopeSelectors are how a host says "dark" to a chart.
//
// Explicit only. An earlier draft also let `prefers-color-scheme:
// dark` flip an un-classed chart, and that is wrong in a way that is
// easy to miss until you look at one: an SVG is inlined into whatever
// page embeds it, and a light page viewed on a machine whose OS is
// set to dark would have rendered dark-theme charts on white — light
// grey labels, near-invisible grid. The OS setting says nothing about
// the background this particular chart landed on.
//
// So the host, which does know, says so: `prism-dark` on the SVG or
// any ancestor. A host that genuinely wants to follow the OS opts in
// with `prism-auto`, which is the only selector the media query below
// touches.
const darkScopeSelectors = ".prism-dark,.prism-dark :root,:root.prism-dark"

// writeCompanionVars emits the dark-mode token overrides.
//
// Two filters, both load-bearing:
//
//  1. Only tokens that actually DIFFER from the light values. A token
//     repeated at the same value is bytes the consumer pays for on
//     every chart, and it makes a diff of the two bases unreadable.
//
//  2. Only COLOUR tokens. This one is a correctness fix, not a size
//     one. The companion block wins over :root by specificity, so a
//     geometry token in it silently overrides whatever the ORGANISATION
//     set: an org that raised --prism-axis-band-padding to 0.5 got 0.5
//     on a light host and Prism's 0.28 on a dark one, because the
//     companion carried the dark base's default for a token the org had
//     already customised. Geometry does not depend on the ground and
//     must never appear here — which is also why the two bases are
//     required to share their chrome measurements exactly.
func writeCompanionVars(b *strings.Builder, t *Theme) {
	name := t.CompanionDark()
	if name == "" {
		return
	}
	dark, ok := Get(name)
	if !ok || dark == nil {
		return
	}

	var light, darkBuf strings.Builder
	writeRootVars(&light, t)
	writeRootVars(&darkBuf, dark)
	diff := diffDeclarations(light.String(), darkBuf.String())
	if diff == "" {
		return
	}

	fmt.Fprintf(b, "%s{%s}", darkScopeSelectors, diff)
	fmt.Fprintf(b, "@media (prefers-color-scheme:dark){.prism-auto,.prism-auto :root,:root.prism-auto{%s}}", diff)
}

// diffDeclarations returns the declarations in dark that are absent
// from light or carry a different value, in dark's own order.
func diffDeclarations(light, dark string) string {
	base := make(map[string]string)
	for _, d := range strings.Split(light, ";") {
		k, v, ok := strings.Cut(d, ":")
		if ok {
			base[k] = v
		}
	}
	var b strings.Builder
	for _, d := range strings.Split(dark, ";") {
		k, v, ok := strings.Cut(d, ":")
		if !ok {
			continue
		}
		if base[k] == v {
			continue
		}
		if !isColorValue(v) {
			continue
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(v)
		b.WriteByte(';')
	}
	return b.String()
}

// isColorValue reports whether a declaration's value is a colour
// rather than a measurement. Intentionally narrow: hex, the CSS colour
// functions, and the two keywords a theme can legitimately carry.
// Anything it does not recognise is treated as geometry and left out
// of the dark companion, which is the safe direction to be wrong in.
func isColorValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if v[0] == '#' {
		return true
	}
	switch v {
	case "transparent", "currentColor", "none":
		return true
	}
	for _, fn := range []string{"rgb(", "rgba(", "hsl(", "hsla(", "oklch(", "oklab(", "lab(", "lch(", "color("} {
		if strings.HasPrefix(v, fn) {
			return true
		}
	}
	return false
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
	if t.TextMutedColor != "" {
		fmt.Fprintf(b, "--prism-color-text-muted:%s;", t.TextMutedColor)
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
	if a.ZeroColor != "" {
		fmt.Fprintf(b, "--prism-axis-zero-color:%s;", a.ZeroColor)
	}
	if a.ZeroWidth != nil {
		fmt.Fprintf(b, "--prism-axis-zero-width:%gpx;", *a.ZeroWidth)
	}
	if a.BandPadding != nil {
		fmt.Fprintf(b, "--prism-axis-band-padding:%g;", *a.BandPadding)
	}
	if a.BandMaxWidth != nil {
		fmt.Fprintf(b, "--prism-axis-band-max-width:%gpx;", *a.BandMaxWidth)
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
	if l.SymbolExtent != nil {
		fmt.Fprintf(b, "--prism-legend-symbol-extent:%gpx;", *l.SymbolExtent)
	}
	if l.SymbolCornerRadius != nil {
		fmt.Fprintf(b, "--prism-legend-symbol-corner-radius:%gpx;", *l.SymbolCornerRadius)
	}
	if l.SymbolStrokeWidth != nil {
		fmt.Fprintf(b, "--prism-legend-symbol-stroke-width:%gpx;", *l.SymbolStrokeWidth)
	}
	if l.Gap != nil {
		fmt.Fprintf(b, "--prism-legend-gap:%gpx;", *l.Gap)
	}
	if l.RowHeight != nil {
		fmt.Fprintf(b, "--prism-legend-row-height:%gpx;", *l.RowHeight)
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
	// Tabular numerals keep a right-aligned numeric column aligned on
	// its digits rather than on its glyph widths, which is the
	// difference between a tick column and a ragged list. font-kerning
	// off stops a proportional fallback face from re-introducing it.
	b.WriteString(".prism-axis-label{fill:var(--prism-axis-label-color,var(--prism-color-text-muted,var(--prism-color-text)));font-family:var(--prism-font-sans);font-size:var(--prism-axis-label-font-size,var(--prism-font-size-label,11px));font-weight:var(--prism-axis-label-font-weight,400);font-variant-numeric:tabular-nums;font-feature-settings:\"tnum\" 1;}")
	b.WriteString(".prism-axis-title{fill:var(--prism-axis-title-color,var(--prism-color-text-muted,var(--prism-color-text)));font-family:var(--prism-font-sans);font-size:var(--prism-axis-title-font-size,var(--prism-font-size-axis-title,11px));font-weight:var(--prism-axis-title-font-weight,500);letter-spacing:var(--prism-axis-title-letter-spacing,0.01em);}")
	b.WriteString(".prism-grid-line{stroke:var(--prism-grid-color,var(--prism-color-grid));stroke-width:var(--prism-grid-width,1px);stroke-opacity:var(--prism-grid-opacity,1);shape-rendering:crispEdges;}")
	b.WriteString(".prism-zero-line{stroke:var(--prism-axis-zero-color,var(--prism-axis-domain-color,var(--prism-color-axis)));stroke-width:var(--prism-axis-zero-width,1px);shape-rendering:crispEdges;}")
	b.WriteString(".prism-title{fill:var(--prism-title-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-title-font-size,var(--prism-font-size-title,15px));font-weight:var(--prism-title-font-weight,600);letter-spacing:var(--prism-title-letter-spacing,-0.006em);}")
	b.WriteString(".prism-legend-title{fill:var(--prism-legend-title-color,var(--prism-color-text-muted,var(--prism-color-text)));font-family:var(--prism-font-sans);font-size:var(--prism-legend-title-font-size,11px);font-weight:var(--prism-legend-title-font-weight,500);}")
	b.WriteString(".prism-legend-label{fill:var(--prism-legend-label-color,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-legend-label-font-size,11px);}")
	b.WriteString(".prism-legend-swatch{stroke:none;}")
	// The area's fill and its upper edge are two elements; give the
	// edge the same token the fill's stroke reads so an override
	// reaches both.
	b.WriteString(".prism-mark-area-edge{fill:none;stroke-linejoin:round;stroke-linecap:round;}")
	b.WriteString(".prism-empty-note{fill:var(--prism-color-text-muted,var(--prism-color-text));font-family:var(--prism-font-sans);font-size:var(--prism-font-size-label,11px);font-style:italic;}")
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
