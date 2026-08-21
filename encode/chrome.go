package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// chrome.go bridges the resolved theme to the layout and axis
// builders. Everything the layout measures with comes from a
// --prism-* token, so an organisation that changes a token gets a
// chart laid out to it rather than a chart laid out to Prism's
// defaults with its colours swapped.

// layoutStyleFromTheme reads the token values the layout needs.
// Absent tokens fall back to DefaultLayoutStyle's value for that
// field individually — a theme that sets only a label size must not
// lose every other measurement to the zero value.
func layoutStyleFromTheme(t *theme.Theme) LayoutStyle {
	st := DefaultLayoutStyle()
	if t == nil {
		return st
	}
	if t.FontSizeLabel > 0 {
		st.LabelFontSize = t.FontSizeLabel
	}
	if t.FontSizeAxisTitle > 0 {
		st.TitleFontSize = t.FontSizeAxisTitle
	}
	if t.FontSizeTitle > 0 {
		st.ChartTitleSize = t.FontSizeTitle
	}
	if a := t.Axis; a != nil {
		if a.LabelFontSize != nil {
			st.LabelFontSize = *a.LabelFontSize
		}
		if a.TitleFontSize != nil {
			st.TitleFontSize = *a.TitleFontSize
		}
		if a.LabelPadding != nil {
			st.LabelPadding = *a.LabelPadding
		}
		if a.TitlePadding != nil {
			st.TitlePadding = *a.TitlePadding
		}
		if a.TickSize != nil {
			st.TickSize = *a.TickSize
		}
	}
	if l := t.Legend; l != nil {
		if l.Gap != nil {
			st.LegendGap = *l.Gap
		}
		if l.RowHeight != nil {
			st.LegendRowH = *l.RowHeight
		}
		if l.SymbolExtent != nil {
			st.LegendSymbol = *l.SymbolExtent
		}
		if l.LabelFontSize != nil {
			st.LegendLabelSz = *l.LabelFontSize
		}
		if l.TitleFontSize != nil {
			st.LegendTitleSz = *l.TitleFontSize
		}
	}
	if ti := t.Title; ti != nil {
		if ti.FontSize != nil {
			st.ChartTitleSize = *ti.FontSize
		}
		if ti.Padding != nil {
			st.TitleBlockPad = *ti.Padding
		}
	}
	if v := t.View; v != nil && v.Padding != nil {
		st.EdgePadding = *v.Padding
	}
	return st
}

// titleAnchorFromTheme returns the SVG text-anchor for the chart
// title. The token was already emitted and already ignored by the
// renderer; it is now read.
func titleAnchorFromTheme(t *theme.Theme) string {
	anchor := "start"
	if t != nil && t.Title != nil && t.Title.Anchor != "" {
		anchor = t.Title.Anchor
	}
	switch anchor {
	case "start", "middle", "end":
		return anchor
	case "center":
		return "middle"
	}
	return "start"
}

// tilingMarks fill their whole categorical cell rather than drawing a
// discrete object inside it.
//
// A heatmap's cells ARE the grid: a 28% gap between them turns a
// continuous surface into a scatter of squares, and a 96px width cap
// leaves the plot half empty. So tiling marks take a hairline gap —
// just enough to separate adjacent cells — and no cap at all.
var tilingMarks = map[string]bool{
	"heatmap": true, "rect": true, "image": true,
}

// bandShapeFor returns the band padding and width cap for a mark type.
func bandShapeFor(t *theme.Theme, markType string) (padding, maxWidth float64) {
	if tilingMarks[markType] {
		return 0.02, 0
	}
	return bandPaddingFromTheme(t), bandMaxWidthFromTheme(t)
}

// bandPaddingFromTheme returns the categorical step fraction left
// empty between bands.
func bandPaddingFromTheme(t *theme.Theme) float64 {
	if t != nil && t.Axis != nil && t.Axis.BandPadding != nil {
		p := *t.Axis.BandPadding
		if p >= 0 && p < 1 {
			return p
		}
	}
	return 0.28
}

// bandMaxWidthFromTheme caps one band's pixel width. Zero disables
// the cap.
func bandMaxWidthFromTheme(t *theme.Theme) float64 {
	if t != nil && t.Axis != nil && t.Axis.BandMaxWidth != nil {
		return *t.Axis.BandMaxWidth
	}
	return 96
}

// gridPlan decides which axis carries the grid and which chrome each
// axis can therefore drop.
//
// The rule is that ONE axis carries the reference lines, and it is
// the measure axis — the one whose values a reader interpolates.
// Categorical positions are read off the label directly, so a grid
// line through them adds a stroke and no information.
//
// Concretely:
//
//   - vertical bars / lines / scatter → horizontal grid off the y axis
//   - horizontal bars (categorical y) → vertical grid off the x axis
//   - both categorical (heatmap)      → no grid at all
//
// A scatter with two continuous axes takes the y grid only, not both.
// A full mesh is the densest possible chrome and it is exactly what
// "fewer gridlines" rules out; the x positions stay readable from the
// labels and the reader's own sense of the axis.
//
// Whichever axis carries the grid then drops BOTH its domain line and
// its tick marks: a grid line already lands on those pixels, and a
// domain line under a grid line is two strokes of different weight on
// one edge.
type gridPlan struct {
	XGrid, YGrid             bool
	XHideDomain, YHideDomain bool
	XHideTicks, YHideTicks   bool
}

func planGrid(xScale, yScale Scale) gridPlan {
	xCat := isCategoricalScale(xScale)
	yCat := isCategoricalScale(yScale)

	switch {
	case xScale == nil || yScale == nil:
		// A single bound axis keeps its own chrome; there is no second
		// axis to hand the reference lines to.
		return gridPlan{XGrid: xScale != nil && !xCat, YGrid: yScale != nil && !yCat}
	case xCat && yCat:
		// Both categorical: no grid, and both domain lines stay — they
		// are the frame the cells sit in.
		return gridPlan{}
	case yCat:
		// Horizontal bars: the x axis is the measure axis.
		return gridPlan{XGrid: true, XHideDomain: true, XHideTicks: true, YHideTicks: true}
	default:
		return gridPlan{YGrid: true, YHideDomain: true, YHideTicks: true, XHideTicks: xCat}
	}
}

// isCategoricalScale reports whether s positions by category rather
// than by magnitude.
func isCategoricalScale(s Scale) bool {
	switch s.(type) {
	case *BandScale, *PointScale, *OrdinalScale:
		return true
	}
	return false
}

// tickLabelsOf extracts the visible labels from a built axis, for the
// layout pass to measure.
func tickLabelsOf(a scene.Axis) []string {
	out := make([]string, 0, len(a.Ticks))
	for _, t := range a.Ticks {
		if t.Minor || t.LabelHidden || t.Label == "" {
			continue
		}
		out = append(out, t.Label)
	}
	return out
}

// applyBandShape stamps the theme's band padding and width cap onto a
// resolved band scale.
//
// Applied after resolution rather than threaded through every
// resolve* signature: padding is a presentation decision and the
// resolvers' job is the domain. It is also where the single-category
// case stops looking broken — with no cap, one category means one bar
// as wide as the plot, which reads as a rendering failure rather than
// as one measurement. The cap centres the bands in the range instead
// of stretching them.
func applyBandShape(s Scale, padding, maxWidth float64) {
	b, ok := s.(*BandScale)
	if !ok || len(b.Categories) == 0 {
		return
	}
	b.Padding = padding
	if maxWidth <= 0 {
		return
	}
	span := b.RangeMax - b.RangeMin
	step := span / float64(len(b.Categories))
	if step*(1-padding) <= maxWidth {
		return
	}
	// Widen the padding until the band hits the cap, which keeps every
	// band centred on the tick its label sits under. Shrinking the
	// RANGE instead would move the bands off their own axis labels.
	b.Padding = 1 - maxWidth/step
	if b.Padding > 0.94 {
		b.Padding = 0.94
	}
}

// niceLinearDomain rounds a continuous domain out to the tick step, so
// the topmost grid line lands on the plot's edge instead of somewhere
// inside it.
//
// Without it a 0-71 domain draws grid lines at 0/20/40/60 and leaves
// an unexplained 11-unit strip above the last one, and the tallest bar
// touches the frame. With it the domain is 0-80, the top grid line IS
// the top edge, and the chart has a frame rather than a ragged top.
//
// The zero baseline is never crossed: a domain that starts at 0 stays
// at 0 rather than being rounded to a negative, and one that starts
// above 0 keeps its floor at 0 so bar lengths stay proportional to
// their values.
//
// Returns the step it snapped to, which the axis builder then reuses.
// Re-deriving the step from the SNAPPED domain gives a different
// answer — a 0-71 domain rounds out to 0-80, and 80 divided by the
// same requested count lands on 10 rather than the 20 that produced
// it, so the axis would draw nine lines where five were intended.
func niceLinearDomain(s Scale, count int) float64 {
	l, ok := s.(*LinearScale)
	if !ok || l.DomainMin == l.DomainMax {
		return 0
	}
	ticks := NiceTicks(l.DomainMin, l.DomainMax, count)
	if len(ticks) < 2 {
		return 0
	}
	step := ticks[1] - ticks[0]
	if step <= 0 {
		return 0
	}
	lo := floorTo(l.DomainMin, step)
	hi := ceilTo(l.DomainMax, step)
	if l.DomainMin >= 0 && lo < 0 {
		lo = 0
	}
	if l.DomainMax <= 0 && hi > 0 {
		hi = 0
	}
	// Snapping outward adds ticks the caller did not ask for: a domain
	// of 3.196-3.604 at a step of 0.1 becomes 3.1-3.7, which is six
	// intervals where three were requested. Coarsen the step until the
	// snapped domain is back near the target.
	//
	// The loop is what makes this function IDEMPOTENT, which matters
	// because it runs twice — once against the provisional plot rect
	// and once against the final one. A version that widened the domain
	// on every call turned 20-80 into 0-100 on the second pass.
	for i := 0; i < 6 && (hi-lo)/step > float64(count)*1.7; i++ {
		step = coarserNiceStep(step)
		lo = floorTo(l.DomainMin, step)
		hi = ceilTo(l.DomainMax, step)
		if l.DomainMin >= 0 && lo < 0 {
			lo = 0
		}
		if l.DomainMax <= 0 && hi > 0 {
			hi = 0
		}
	}
	if hi > lo {
		l.DomainMin, l.DomainMax = lo, hi
	}
	return step
}

// coarserNiceStep returns the next step up the 1-2-5-10 ladder.
func coarserNiceStep(step float64) float64 {
	if step <= 0 {
		return 1
	}
	pow := math.Pow(10, math.Floor(math.Log10(step)))
	mant := step / pow
	switch {
	case mant < 1.5:
		return 2 * pow
	case mant < 3.5:
		return 5 * pow
	default:
		return 10 * pow
	}
}

func floorTo(v, step float64) float64 {
	n := v / step
	f := float64(int(n))
	if n < 0 && n != f {
		f--
	}
	return f * step
}

func ceilTo(v, step float64) float64 {
	n := v / step
	f := float64(int(n))
	if n > 0 && n != f {
		f++
	}
	return f * step
}

// axisOptsWith layers the layout's decisions on top of whatever the
// spec's channel.axis block asked for. The spec always wins: an
// author who wrote `"grid": false` gets no grid even where the plan
// would have put one, and an explicit label_angle overrides the
// rotation the layout derived.
func axisOptsWith(ch *spec.PositionChannel, st LayoutStyle, grid, hideDomain, hideTicks bool, angle, labelMax, tickStep float64) AxisOpts {
	opts := AxisOpts{
		Grid:         grid,
		LabelOverlap: "parity",
		MinorTicks:   false,
		Style:        st,
		HideDomain:   hideDomain,
		HideTicks:    hideTicks,
		LabelAngle:   angle,
		// Zero is "no cap", so a caller that has not measured yet gets
		// untruncated labels to measure.
		LabelMaxWidth: labelMax,
		TickStep:      tickStep,
	}
	if ch == nil {
		return opts
	}
	opts.Title = ch.Field
	if ch.Axis == nil {
		return opts
	}
	if t, ok := axisTitleString(ch.Axis.Title); ok {
		opts.Title = t
	}
	if ch.Axis.Grid != nil {
		opts.Grid = *ch.Axis.Grid
		// An axis the author explicitly gridded keeps its own domain
		// line and ticks: the suppression above is a consequence of
		// Prism's default plan, not of the grid existing.
		opts.HideDomain = false
		opts.HideTicks = false
	}
	if ch.Axis.LabelAngle != nil {
		opts.LabelAngle = *ch.Axis.LabelAngle
	}
	if mode, ok := overlapMode(ch.Axis.LabelOverlap); ok {
		opts.LabelOverlap = mode
	}
	if ch.Axis.Format != "" {
		opts.Format = ch.Axis.Format
	}
	return opts
}

// rerangeScale moves a resolved scale onto a new pixel range without
// re-deriving its domain. Used by the second layout pass, where the
// plot rect moved but the data did not.
func rerangeScale(s Scale, min, max float64) {
	switch v := s.(type) {
	case *LinearScale:
		v.RangeMin, v.RangeMax = min, max
	case *LogScale:
		v.RangeMin, v.RangeMax = min, max
	case *PowScale:
		v.RangeMin, v.RangeMax = min, max
	case *SqrtScale:
		v.Inner.RangeMin, v.Inner.RangeMax = min, max
	case *TimeScale:
		v.Linear.RangeMin, v.Linear.RangeMax = min, max
	case *BandScale:
		v.RangeMin, v.RangeMax = min, max
	case *PointScale:
		v.RangeMin, v.RangeMax = min, max
	case *OrdinalScale:
		if len(v.Categories) == 1 {
			v.Positions[0] = (min + max) / 2
			return
		}
		step := (max - min) / float64(len(v.Categories)-1)
		for i := range v.Categories {
			v.Positions[i] = min + step*float64(i)
		}
	}
}

// placeTitle positions the chart title from the theme's anchor token.
//
// The renderer previously wrote text-anchor="middle" and
// x=plot.CenterX() unconditionally, so --prism-title-anchor was
// emitted into every SVG and read by nothing. It is now honoured, and
// a start-anchored title lines up with the y axis rather than
// floating over the plot's centre — which also stops a long title
// from overrunning both edges of a narrow chart.
func placeTitle(content, anchor string, layout Layout, st LayoutStyle) *scene.TextElement {
	if content == "" {
		return nil
	}
	titleH := TextMetrics{FontSize: st.ChartTitleSize, Bold: true}.Height()
	// Baseline sits one line-height below the frame's top inset.
	y := st.EdgePadding + titleH*0.82
	x := layout.Plot.X
	switch anchor {
	case "middle":
		x = layout.Plot.CenterX()
	case "end":
		x = layout.Plot.Right()
	}
	return &scene.TextElement{Content: content, X: x, Y: y, Anchor: anchor}
}

// themeCornerRadius returns the resolved corner radius for a mark
// type, so the theme token reaches the geometry. Previously only the
// spec's mark-def could set it, which made
// --prism-mark-bar-corner-radius a token every SVG carried and no
// chart read.
func themeCornerRadius(t *theme.Theme, markType string) float64 {
	if t == nil {
		return 0
	}
	if ms := t.MarkDefault(markType); ms != nil && ms.CornerRadius != nil {
		return *ms.CornerRadius
	}
	return 0
}

// zeroBaselineMarks are the marks whose geometry MEASURES FROM ZERO,
// so their axis must include it.
//
// A bar's length is the value, and a bar chart truncated at 40 says
// something false about the ratio between 48 and 62. An area is the
// same argument with a filled region. A line and a point say only
// where a value SITS, so forcing zero onto their axis is not honesty —
// it is the opposite: it compresses the variation the chart exists to
// show into a strip at the top and leaves two thirds of the plot
// empty. Conversion rates of 3.2% to 3.6% become a flat line.
//
// So: bars and areas always include zero; lines, points, rules and
// ticks take the data's own extent. Either way the axis is LABELLED,
// so a reader can see where it starts — which is the actual
// requirement, not a fixed baseline.
//
// `"scale": {"zero": true|false}` on the channel overrides this per
// chart.
var zeroBaselineMarks = map[string]bool{
	"bar": true, "area": true, "rect": true, "histogram": true,
	"heatmap": true, "boxplot": true, "violin": true, "bullet": true,
	"funnel": true, "sparkbar": true, "sparkarea": true, "winloss": true,
}

// relaxZeroBaseline recomputes a continuous domain from the data's own
// extent when the mark does not measure from zero.
//
// Runs after resolution because resolveLinear unconditionally pulls
// zero into every positive domain; this narrows it back for the marks
// that do not need it, then pads by a fraction of the span so the
// extreme values do not sit exactly on the frame.
func relaxZeroBaseline(s Scale, ch *spec.PositionChannel, tbl *table.Table, markType string) {
	l, ok := s.(*LinearScale)
	if !ok || ch == nil || ch.Field == "" {
		return
	}
	if want, explicit := explicitZero(ch); explicit {
		if want {
			return
		}
	} else if zeroBaselineMarks[markType] {
		return
	}
	col, ok := tbl.Column(ch.Field)
	if !ok {
		return
	}
	mn, mx, ok := columnExtent(col)
	if !ok || mn == mx {
		return
	}
	// The data's own extent, with nothing added.
	//
	// Padding here and nicing afterwards fight each other, in both
	// directions, and two earlier drafts lost to it. A tenth of the
	// span pushes 21-66 out to 16.5-70.5, which rounds DOWN to a step
	// boundary at 0 — reinstating the exact zero baseline this function
	// exists to remove. Even one percent pushes a week axis of 1-4 to
	// 0.97, and floor-to-step puts its origin at 0, wasting a quarter
	// of the plot on weeks that do not exist.
	//
	// Rounding out to a step boundary is itself the breathing room, and
	// it is the only source of it, which is what keeps the result
	// stable when the pass runs twice.
	l.DomainMin, l.DomainMax = mn, mx
}

// explicitZero reads `scale.zero` off the channel, reporting whether
// the spec stated one at all.
func explicitZero(ch *spec.PositionChannel) (want, explicit bool) {
	if ch == nil || ch.Scale == nil || ch.Scale.Zero == nil {
		return false, false
	}
	return *ch.Scale.Zero, true
}

// columnExtent returns the numeric min and max of a column.
func columnExtent(col table.Column) (float64, float64, bool) {
	var mn, mx float64
	first := true
	for i := 0; i < col.Len(); i++ {
		f, ok := scale.ToFloat(col.ValueAt(i))
		if !ok {
			continue
		}
		if first {
			mn, mx, first = f, f, false
			continue
		}
		if f < mn {
			mn = f
		}
		if f > mx {
			mx = f
		}
	}
	return mn, mx, !first
}
