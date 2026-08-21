package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// AxisOpts carries per-axis overrides resolved from the spec's
// channel.axis block plus the layout decisions Compute made.
type AxisOpts struct {
	Title        string
	Grid         bool
	LabelAngle   float64
	LabelOverlap string // "parity" (default) | "auto" | "none"
	MinorTicks   bool
	Format       string // d3-format spec for tick labels

	// Style carries the resolved token measurements so tick and title
	// offsets are computed once, at encode time, from the same numbers
	// the layout reserved space with.
	Style LayoutStyle
	// LabelMaxWidth truncates labels wider than this. 0 disables.
	LabelMaxWidth float64
	// TickCount is the target major-tick count for continuous scales.
	// 0 lets the builder derive it from the plot extent.
	TickCount int
	// TickStep pins the spacing between major ticks on a continuous
	// scale, overriding the count. Set by the layout pass, which
	// already chose a step when it rounded the domain out to one — see
	// niceLinearDomain.
	TickStep float64
	// HideDomain and HideTicks let the caller drop chrome a grid line
	// already carries.
	HideDomain bool
	HideTicks  bool
}

// DefaultAxisOpts returns the defaults for a bare BuildAxis call.
func DefaultAxisOpts(title string) AxisOpts {
	return AxisOpts{
		Title:        title,
		Grid:         true,
		LabelAngle:   0,
		LabelOverlap: "parity",
		MinorTicks:   false,
		Style:        DefaultLayoutStyle(),
	}
}

// BuildAxis converts a resolved Scale into a populated scene.Axis.
func BuildAxis(scale Scale, channel scene.Channel, position scene.AxisPosition, plot scene.Rect, title string) scene.Axis {
	return BuildAxisWithOpts(scale, channel, position, plot, DefaultAxisOpts(title))
}

// tickTargetFor returns how many major ticks an axis of the given
// pixel extent should carry.
//
// The old code asked for five on every axis regardless of size, which
// crowds a 180px facet cell and strands a 700px chart with four
// gridlines and a lot of white. Density is what actually matters:
// roughly one label per 150px vertically and one per 190px
// horizontally, where labels are wide rather than tall.
//
// The count is a REQUEST, not a promise: NiceTicks follows D3, which
// rounds the step down to a nice value and therefore returns at least
// count intervals and often half again as many. Asking for 6 on a
// 0-80 domain returns nine ticks. The divisors above are calibrated
// against that overshoot rather than against the tick count directly —
// a 500px axis asks for 4 and gets 5, which is the density intended.
func tickTargetFor(extent float64, horizontal bool) int {
	per := 150.0
	if horizontal {
		per = 190.0
	}
	n := int(math.Round(extent / per))
	if n < 2 {
		n = 2
	}
	if n > 6 {
		n = 6
	}
	return n
}

// BuildAxisWithOpts is the full-control axis builder.
func BuildAxisWithOpts(scale Scale, channel scene.Channel, position scene.AxisPosition, plot scene.Rect, opts AxisOpts) scene.Axis {
	st := opts.Style
	if st.LabelFontSize == 0 {
		st = DefaultLayoutStyle()
	}
	horizontal := position == scene.AxisPositionBottom || position == scene.AxisPositionTop

	axis := scene.Axis{
		ID:         string(channel) + "-axis",
		Channel:    channel,
		Position:   position,
		Title:      opts.Title,
		LabelAngle: opts.LabelAngle,
		HideDomain: opts.HideDomain,
		HideTicks:  opts.HideTicks,
	}

	extent := plot.H
	if horizontal {
		extent = plot.W
	}
	count := opts.TickCount
	if count == 0 {
		count = tickTargetFor(extent, horizontal)
	}

	categorical := false

	switch s := scale.(type) {
	case *LinearScale:
		ticks := NiceTicks(s.DomainMin, s.DomainMax, count)
		if opts.TickStep > 0 {
			ticks = ticksByStep(s.DomainMin, s.DomainMax, opts.TickStep)
		}
		labelled, err := TicksWithLabels(ticks, s, opts.Format)
		if err == nil {
			axis.Ticks = labelled
		}
		if opts.MinorTicks {
			axis.Ticks = injectLinearMinorTicks(axis.Ticks, s)
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleLinear,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
		}
	case *TimeScale:
		axis.Ticks = TimeTicks(s, count)
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleTime,
			Domain: []any{s.Linear.DomainMin, s.Linear.DomainMax},
			Range:  [2]float64{s.Linear.RangeMin, s.Linear.RangeMax},
		}
	case *LogScale:
		axis.Ticks = LogTicks(s)
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleLog,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
			Base:   s.Base,
		}
	case *PowScale:
		axis.Ticks = PowTicks(s, count)
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScalePow,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
			Exp:    s.Exp,
		}
	case *SqrtScale:
		axis.Ticks = SqrtTicks(s, count)
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleSqrt,
			Domain: []any{s.Inner.DomainMin, s.Inner.DomainMax},
			Range:  [2]float64{s.Inner.RangeMin, s.Inner.RangeMax},
			Exp:    0.5,
		}
	case *BandScale:
		categorical = true
		axis.Ticks = BandTicks(s)
		dom := make([]any, len(s.Categories))
		for i, c := range s.Categories {
			dom[i] = c
		}
		axis.Scale = scene.ScaleSpec{
			Type:    scene.ScaleBand,
			Domain:  dom,
			Range:   [2]float64{s.RangeMin, s.RangeMax},
			Padding: s.Padding,
		}
	case *PointScale:
		categorical = true
		ticks := make([]scene.Tick, 0, len(s.Categories))
		for _, c := range s.Categories {
			pix, err := s.Apply(c)
			if err != nil {
				continue
			}
			ticks = append(ticks, scene.Tick{Value: c, Pixel: pix, Label: c})
		}
		axis.Ticks = ticks
		dom := make([]any, len(s.Categories))
		for i, c := range s.Categories {
			dom[i] = c
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScalePoint,
			Domain: dom,
			Range:  [2]float64{s.RangeMin, s.RangeMax},
		}
	case *OrdinalScale:
		categorical = true
		ticks := make([]scene.Tick, len(s.Categories))
		for i, c := range s.Categories {
			ticks[i] = scene.Tick{Value: c, Pixel: s.Positions[i], Label: c}
		}
		axis.Ticks = ticks
		dom := make([]any, len(s.Categories))
		for i, c := range s.Categories {
			dom[i] = c
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleOrdinal,
			Domain: dom,
			Range:  s.Range(),
		}
	}

	// Truncate over-wide labels, keeping the full text for the
	// renderer to emit as a <title>.
	if opts.LabelMaxWidth > 0 {
		m := TextMetrics{FontSize: st.LabelFontSize, Tabular: !categorical}
		for i := range axis.Ticks {
			short, cut := m.Truncate(axis.Ticks[i].Label, opts.LabelMaxWidth)
			if cut {
				axis.Ticks[i].Full = axis.Ticks[i].Label
				axis.Ticks[i].Label = short
			}
		}
	}

	// Overlap handling. Categorical axes are exempt when the layout
	// already rotated or truncated them: dropping a category label
	// loses information the reader cannot interpolate back, unlike a
	// numeric one.
	if opts.LabelOverlap != "none" && !(categorical && opts.LabelAngle != 0) {
		axis.Ticks = applyLabelOverlap(axis.Ticks, opts.LabelOverlap, position, st)
	}

	// Domain line + (optional) grid lines.
	switch position {
	case scene.AxisPositionBottom:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Bottom(), X2: plot.Right(), Y2: plot.Bottom()}
		if opts.Grid {
			axis.Grid = gridLines(axis.Ticks, plot, true)
		}
	case scene.AxisPositionTop:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Y, X2: plot.Right(), Y2: plot.Y}
		if opts.Grid {
			axis.Grid = gridLines(axis.Ticks, plot, true)
		}
	case scene.AxisPositionLeft:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Y, X2: plot.X, Y2: plot.Bottom()}
		if opts.Grid {
			axis.Grid = gridLines(axis.Ticks, plot, false)
		}
	case scene.AxisPositionRight:
		axis.Domain = scene.Line{X1: plot.Right(), Y1: plot.Y, X2: plot.Right(), Y2: plot.Bottom()}
		if opts.Grid {
			axis.Grid = gridLines(axis.Ticks, plot, false)
		}
	}

	if opts.HideTicks {
		axis.Ticks = clearTickMarks(axis.Ticks)
	}
	axis.ZeroLine = zeroLineFor(scale, plot, horizontal)
	axis.LabelOffset, axis.TitleOffset = axisOffsets(axis, st, opts, categorical)

	return axis
}

// axisOffsets resolves where the tick labels and the axis title sit,
// measured out from the plot edge. Both are derived from the same
// tokens the layout reserved space with, so the title cannot land on
// top of the label column no matter how wide the labels turned out.
func axisOffsets(axis scene.Axis, st LayoutStyle, opts AxisOpts, categorical bool) (labelOff, titleOff float64) {
	tick := st.TickSize
	if opts.HideTicks {
		tick = 0
	}
	labelOff = tick + st.LabelPadding

	m := TextMetrics{FontSize: st.LabelFontSize, Tabular: !categorical}
	labels := make([]string, 0, len(axis.Ticks))
	for _, t := range axis.Ticks {
		if !t.LabelHidden {
			labels = append(labels, t.Label)
		}
	}
	horizontal := axis.Position == scene.AxisPositionBottom || axis.Position == scene.AxisPositionTop
	var labelExtent float64
	if horizontal {
		if opts.LabelAngle != 0 {
			labelExtent = m.MaxWidth(labels) * math.Abs(math.Sin(opts.LabelAngle*math.Pi/180))
		} else {
			labelExtent = m.Height()
		}
	} else {
		labelExtent = m.MaxWidth(labels)
	}
	titleOff = labelOff + labelExtent + st.TitlePadding
	return labelOff, titleOff
}

// ticksByStep walks a domain at a fixed step, inclusive of both ends
// when they land on a multiple. Used when the layout already chose the
// step while rounding the domain, so the two cannot disagree.
func ticksByStep(min, max, step float64) []float64 {
	if step <= 0 || max <= min {
		return nil
	}
	n := int(math.Round((max-min)/step)) + 1
	if n < 2 || n > 200 {
		return nil
	}
	out := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		v := min + float64(i)*step
		// Re-snap to the step to shed accumulated float error, so a
		// 0.1 step yields 0.3 rather than 0.30000000000000004.
		out = append(out, math.Round(v/step)*step)
	}
	return out
}

// clearTickMarks flags every tick as label-only. The tick's pixel and
// label survive; only the little perpendicular stroke goes away.
func clearTickMarks(ticks []scene.Tick) []scene.Tick {
	out := make([]scene.Tick, 0, len(ticks))
	for _, t := range ticks {
		if t.Minor {
			continue
		}
		out = append(out, t)
	}
	return out
}

// zeroLineFor returns the emphasised baseline at value 0, but only
// when 0 falls strictly INSIDE the domain.
//
// When 0 is the domain's edge the axis line already is the baseline,
// and a second stroke on the same pixels is the double-line artefact
// this pass is removing elsewhere. When 0 is interior — a
// change-versus-last-quarter chart, a z-score — the crossing is the
// single most important reference on the chart and a plain grid line
// does not carry it.
func zeroLineFor(s Scale, plot scene.Rect, horizontal bool) *scene.Line {
	type domained interface {
		Apply(any) (float64, error)
	}
	var mn, mx float64
	switch v := s.(type) {
	case *LinearScale:
		mn, mx = v.DomainMin, v.DomainMax
	case *PowScale:
		mn, mx = v.DomainMin, v.DomainMax
	case *SqrtScale:
		mn, mx = v.Inner.DomainMin, v.Inner.DomainMax
	default:
		return nil
	}
	if mn >= 0 || mx <= 0 {
		return nil
	}
	var d domained = s.(domained)
	px, err := d.Apply(0.0)
	if err != nil {
		return nil
	}
	if horizontal {
		return &scene.Line{X1: px, Y1: plot.Y, X2: px, Y2: plot.Bottom()}
	}
	return &scene.Line{X1: plot.X, Y1: px, X2: plot.Right(), Y2: px}
}

// gridLines returns grid lines anchored to the plot region.
// vertical=true emits vertical lines (one per x-axis tick); false
// emits horizontal lines (one per y-axis tick). Minor ticks never get
// one — a grid at two weights is a texture, not a reference.
func gridLines(ticks []scene.Tick, plot scene.Rect, vertical bool) []scene.Line {
	out := make([]scene.Line, 0, len(ticks))
	for _, t := range ticks {
		if t.Minor {
			continue
		}
		if vertical {
			out = append(out, scene.Line{X1: t.Pixel, Y1: plot.Y, X2: t.Pixel, Y2: plot.Bottom()})
		} else {
			out = append(out, scene.Line{X1: plot.X, Y1: t.Pixel, X2: plot.Right(), Y2: t.Pixel})
		}
	}
	return out
}

// injectLinearMinorTicks inserts a Minor=true tick at each midpoint
// between consecutive majors. Off by default now; kept for specs that
// ask for it explicitly.
func injectLinearMinorTicks(majors []scene.Tick, s *LinearScale) []scene.Tick {
	if len(majors) < 2 {
		return majors
	}
	out := make([]scene.Tick, 0, len(majors)*2-1)
	for i := 0; i < len(majors); i++ {
		out = append(out, majors[i])
		if i+1 < len(majors) {
			a, ok1 := majors[i].Value.(float64)
			b, ok2 := majors[i+1].Value.(float64)
			if !ok1 || !ok2 {
				continue
			}
			mid := (a + b) / 2
			pix, err := s.Apply(mid)
			if err != nil {
				continue
			}
			out = append(out, scene.Tick{
				Value: mid,
				Pixel: pix,
				Label: "",
				Minor: true,
			})
		}
	}
	return out
}

// applyLabelOverlap hides every other major label when adjacent boxes
// collide.
//
// The previous implementation compared each label's start against the
// PRECEDING label's end in slice order. On a y axis that order is
// ascending by value and therefore DESCENDING by pixel, so `start <
// lastEnd` was true for every pair no matter how far apart they sat,
// and half the labels on every vertical axis were dropped — the "0
// and 40, nothing else" output. Extents are now compared in absolute
// pixel order, which is direction-agnostic.
func applyLabelOverlap(ticks []scene.Tick, mode string, position scene.AxisPosition, st LayoutStyle) []scene.Tick {
	if len(ticks) < 2 {
		return ticks
	}
	out := make([]scene.Tick, len(ticks))
	copy(out, ticks)

	horizontal := position == scene.AxisPositionBottom || position == scene.AxisPositionTop
	m := TextMetrics{FontSize: st.LabelFontSize, Tabular: true}
	lineH := m.Height()

	// Walk the labelled ticks in ascending pixel order. Build the
	// index list first so the pixel direction of the underlying slice
	// is irrelevant.
	idx := make([]int, 0, len(out))
	for i := range out {
		if out[i].Minor || out[i].Label == "" {
			continue
		}
		idx = append(idx, i)
	}
	if len(idx) < 2 {
		return out
	}
	if out[idx[0]].Pixel > out[idx[len(idx)-1]].Pixel {
		for i, j := 0, len(idx)-1; i < j; i, j = i+1, j-1 {
			idx[i], idx[j] = idx[j], idx[i]
		}
	}

	// Minimum gap between two label boxes before they read as one.
	const gutter = 4.0
	extent := func(i int) (float64, float64) {
		if horizontal {
			w := m.Width(out[i].Label)
			return out[i].Pixel - w/2, out[i].Pixel + w/2
		}
		return out[i].Pixel - lineH/2, out[i].Pixel + lineH/2
	}

	_, lastEnd := extent(idx[0])
	skip := false
	for _, i := range idx[1:] {
		start, end := extent(i)
		if start < lastEnd+gutter {
			if mode == "parity" && !skip {
				skip = true
				out[i].LabelHidden = true
				continue
			}
		}
		skip = false
		lastEnd = end
	}
	return out
}
