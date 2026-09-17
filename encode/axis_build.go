package encode

import (
	"github.com/frankbardon/prism/encode/scene"
)

// AxisOpts carries per-axis overrides resolved from the spec's
// channel.axis block. Defaults match the P05 behaviour (grid on,
// 0-degree labels, parity-skip overlap mode).
type AxisOpts struct {
	Title        string
	Grid         bool
	LabelAngle   float64
	LabelOverlap string // "parity" (default) | "auto" | "none"
	MinorTicks   bool   // default true for linear
	Format       string // d3-format spec for tick labels
	// Orient is the spec's `axis.orient` — the side the axis sits on
	// ("top" / "bottom" for x, "left" / "right" for y). Empty means
	// the channel's default side. It is never consumed by
	// BuildAxisWithOpts directly: AxisPositionFor turns it into the
	// scene.AxisPosition that drives BOTH the layout reservation
	// (AxisPlacement) and the position stamped on the built axis, so
	// the padding can never sit on one side while the axis renders on
	// another. A value invalid for the channel is rejected upstream by
	// PRISM_SPEC_044 and falls back to the default side here.
	Orient string
	// Labels / Ticks / Domain are the per-component visibility
	// switches from `axis.labels`, `axis.ticks` and `axis.domain`
	// (E3-S2). All three default to true and compose independently:
	// suppressing the labels leaves the tick marks and the domain
	// line, and so on. Whole-axis suppression is `"axis": null`
	// (E1-S4), handled before the axis is ever built.
	Labels bool
	Ticks  bool
	Domain bool
	// TickSize / LabelPadding / TitlePadding are the spec-level pixel
	// overrides. Nil defers to the theme's --prism-axis-tick-size /
	// --prism-axis-label-padding / --prism-axis-title-padding tokens,
	// and then to the renderer's built-in metrics.
	TickSize     *float64
	LabelPadding *float64
	TitlePadding *float64
	// LabelLimit is the maximum label width in pixels before the label
	// is truncated with an ellipsis. Nil or non-positive means no
	// limit. Truncation happens here, at encode time, so every
	// renderer consuming the Scene IR agrees on the shortened text.
	LabelLimit *float64
	// Zindex draws the axis behind the marks at 0 (the default) and in
	// front of them at any positive value.
	Zindex int

	// TickCount is the author's `axis.tick_count`: how many major
	// ticks the continuous generators should aim for. Nil means the
	// spec said nothing, so DefaultTickCount applies; an explicit 0
	// means "no ticks" (and therefore no grid lines). It is a hint,
	// not a guarantee — the nice-tick generator rounds to a readable
	// step and may land either side of the request.
	TickCount *int

	// Values is the author's `axis.values`: an explicit tick set that
	// replaces the generated one outright, overriding both TickCount
	// and TickMinStep. Entries the axis cannot place are dropped with
	// scene.WarnAxisValuesDropped rather than drawn off-plot.
	Values []any

	// TickMinStep is the author's `axis.tick_min_step`: the smallest
	// gap, in domain units, allowed between adjacent generated ticks.
	// Zero means unset. It only shapes *generated* ticks — Values
	// pins exactly what it names.
	TickMinStep float64

	// Warnings is an optional sink for the warnings the axis builder
	// raises (today: dropped `axis.values` entries). BuildAxisWithOpts
	// cannot return warnings without breaking its pinned five-argument
	// shape, which encode/axis_call_sites_test.go gates, so callers
	// that collect warnings wire their slice in via withWarnings.
	Warnings *[]scene.Warning
}

// withWarnings returns a copy of opts with sink wired as its warning
// destination. Call sites read `axisOptsFor(enc.X).withWarnings(&warnings)`.
func (o AxisOpts) withWarnings(sink *[]scene.Warning) AxisOpts {
	o.Warnings = sink
	return o
}

// warn appends w to the opts' warning sink, if one is wired.
func (o AxisOpts) warn(w scene.Warning) {
	if o.Warnings == nil {
		return
	}
	*o.Warnings = append(*o.Warnings, w)
}

// tickCount returns the effective tick-count hint: the author's
// `axis.tick_count` when set, DefaultTickCount otherwise. A negative
// request is clamped to 0 ("no ticks").
func (o AxisOpts) tickCount() int {
	if o.TickCount == nil {
		return DefaultTickCount
	}
	if *o.TickCount < 0 {
		return 0
	}
	return *o.TickCount
}

// pinned reports whether the author pinned an explicit tick set.
func (o AxisOpts) pinned() bool { return len(o.Values) > 0 }

// DefaultAxisOpts returns the P06 defaults.
func DefaultAxisOpts(title string) AxisOpts {
	return AxisOpts{
		Title:        title,
		Grid:         true,
		LabelAngle:   0,
		LabelOverlap: "parity",
		MinorTicks:   true,
		Labels:       true,
		Ticks:        true,
		Domain:       true,
	}
}

// BuildAxis converts a resolved Scale into a populated scene.Axis,
// including ticks, the axis domain line, and grid lines anchored to
// the plot region. Uses DefaultAxisOpts; callers needing overrides
// invoke BuildAxisWithOpts instead.
func BuildAxis(scale Scale, channel scene.Channel, position scene.AxisPosition, plot scene.Rect, title string) scene.Axis {
	return BuildAxisWithOpts(scale, channel, position, plot, DefaultAxisOpts(title))
}

// BuildAxisWithOpts is the full-control axis builder.
func BuildAxisWithOpts(scale Scale, channel scene.Channel, position scene.AxisPosition, plot scene.Rect, opts AxisOpts) scene.Axis {
	axis := scene.Axis{
		ID:           string(channel) + "-axis",
		Channel:      channel,
		Position:     position,
		Title:        opts.Title,
		LabelAngle:   opts.LabelAngle,
		HideLabels:   !opts.Labels,
		HideTicks:    !opts.Ticks,
		HideDomain:   !opts.Domain,
		TickSize:     opts.TickSize,
		LabelPadding: opts.LabelPadding,
		TitlePadding: opts.TitlePadding,
		Zindex:       opts.Zindex,
	}

	switch s := scale.(type) {
	case *LinearScale:
		var ticks []float64
		if opts.pinned() {
			ticks = pinnedNumericTicks(opts, channel, s.DomainMin, s.DomainMax, toFloatValue)
		} else {
			ticks = generatedLinearTicks(s.DomainMin, s.DomainMax, opts)
		}
		labelled, err := TicksWithLabels(ticks, s, opts.Format)
		if err == nil {
			axis.Ticks = labelled
		}
		// Minor ticks are midpoints of a *generated* nice sequence.
		// A pinned tick set has no such sequence to halve, so pinning
		// suppresses them (see docs/src/concepts/encoding.md).
		if opts.MinorTicks && !opts.pinned() {
			axis.Ticks = injectLinearMinorTicks(axis.Ticks, s)
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleLinear,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
			Clamp:  s.Clamp,
		}
	case *TimeScale:
		if opts.pinned() {
			axis.Ticks = pinnedTimeTicks(s, opts, channel)
		} else if count := opts.tickCount(); count > 0 {
			axis.Ticks = filterTickMinStep(TimeTicks(s, count), opts.TickMinStep)
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleTime,
			Domain: []any{s.Linear.DomainMin, s.Linear.DomainMax},
			Range:  [2]float64{s.Linear.RangeMin, s.Linear.RangeMax},
			Clamp:  s.Linear.Clamp,
		}
	case *LogScale:
		if opts.pinned() {
			axis.Ticks = pinnedScaleTicks(s, opts, channel,
				s.DomainMin, s.DomainMax, toFloatValue, formatLogTick)
		} else if count := opts.tickCount(); count > 0 {
			axis.Ticks = filterTickMinStep(thinLogTicks(LogTicks(s), count), opts.TickMinStep)
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleLog,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
			Base:   s.Base,
			Clamp:  s.Clamp,
		}
	case *PowScale:
		if opts.pinned() {
			axis.Ticks = pinnedScaleTicks(s, opts, channel,
				s.DomainMin, s.DomainMax, toFloatValue, powTickLabel(opts.Format))
		} else if count := opts.tickCount(); count > 0 {
			axis.Ticks = filterTickMinStep(PowTicks(s, count), opts.TickMinStep)
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScalePow,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
			Exp:    s.Exp,
			Clamp:  s.Clamp,
		}
	case *SqrtScale:
		if opts.pinned() {
			axis.Ticks = pinnedScaleTicks(s, opts, channel,
				s.Inner.DomainMin, s.Inner.DomainMax, toFloatValue, powTickLabel(opts.Format))
		} else if count := opts.tickCount(); count > 0 {
			axis.Ticks = filterTickMinStep(SqrtTicks(s, count), opts.TickMinStep)
		}
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleSqrt,
			Domain: []any{s.Inner.DomainMin, s.Inner.DomainMax},
			Range:  [2]float64{s.Inner.RangeMin, s.Inner.RangeMax},
			Exp:    0.5,
			Clamp:  s.Inner.Clamp,
		}
	case *BandScale:
		axis.Ticks = pinnedCategoryTicks(BandTicks(s), opts, channel)
		dom := make([]any, len(s.Categories))
		for i, c := range s.Categories {
			dom[i] = c
		}
		axis.Scale = scene.ScaleSpec{
			Type:    scene.ScaleBand,
			Domain:  dom,
			Range:   [2]float64{s.RangeMin, s.RangeMax},
			Padding: s.PaddingInner,
		}
	case *PointScale:
		ticks := make([]scene.Tick, 0, len(s.Categories))
		for _, c := range s.Categories {
			pix, err := s.Apply(c)
			if err != nil {
				continue
			}
			ticks = append(ticks, scene.Tick{Value: c, Pixel: pix, Label: c})
		}
		axis.Ticks = pinnedCategoryTicks(ticks, opts, channel)
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
		ticks := make([]scene.Tick, len(s.Categories))
		for i, c := range s.Categories {
			ticks[i] = scene.Tick{Value: c, Pixel: s.Positions[i], Label: c}
		}
		axis.Ticks = pinnedCategoryTicks(ticks, opts, channel)
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

	// Label truncation (E3-S2) runs before overlap detection: a
	// truncated label is narrower, so it may no longer collide with
	// its neighbour and should keep its slot.
	axis.Ticks = applyLabelLimit(axis.Ticks, opts.LabelLimit)

	// Overlap handling: parity-skip when adjacent labels collide.
	if opts.LabelOverlap != "none" {
		axis.Ticks = applyLabelOverlap(axis.Ticks, opts.LabelOverlap, position)
	}

	// Domain line + (optional) grid lines.
	switch position {
	case scene.AxisPositionBottom:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Bottom(), X2: plot.Right(), Y2: plot.Bottom()}
		if opts.Grid {
			axis.Grid = horizontalGrid(axis.Ticks, plot, true)
		}
	case scene.AxisPositionTop:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Y, X2: plot.Right(), Y2: plot.Y}
		if opts.Grid {
			axis.Grid = horizontalGrid(axis.Ticks, plot, true)
		}
	case scene.AxisPositionLeft:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Y, X2: plot.X, Y2: plot.Bottom()}
		if opts.Grid {
			axis.Grid = horizontalGrid(axis.Ticks, plot, false)
		}
	case scene.AxisPositionRight:
		axis.Domain = scene.Line{X1: plot.Right(), Y1: plot.Y, X2: plot.Right(), Y2: plot.Bottom()}
		if opts.Grid {
			axis.Grid = horizontalGrid(axis.Ticks, plot, false)
		}
	}

	return axis
}

// horizontalGrid returns grid lines anchored to the plot region.
// vertical=true emits vertical lines (one per x-axis tick); false
// emits horizontal lines (one per y-axis tick). Minor ticks emit
// shorter / no grid lines depending on caller preference (we emit
// for majors only — minor ticks add visual noise to grids).
func horizontalGrid(ticks []scene.Tick, plot scene.Rect, vertical bool) []scene.Line {
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

// axisLabelCharWidth is Prism's standing approximation of one tick
// label character's advance width in pixels. Prism runs no text
// measurement pass, so both the overlap heuristic and the label_limit
// truncation below estimate from this constant rather than from font
// metrics. It aliases scene.LabelCharWidth so the axis heuristic and
// the node-label placement in encode/marks share one number.
const axisLabelCharWidth = scene.LabelCharWidth

// axisLabelEllipsis is appended to a label shortened by label_limit.
const axisLabelEllipsis = "…"

// applyLabelLimit truncates every tick label whose estimated width
// exceeds limit pixels, appending an ellipsis. A nil or non-positive
// limit means "no limit" (Vega-Lite's convention) and returns the
// ticks untouched. A limit too small to hold even the ellipsis drops
// the label entirely rather than emitting a lone "…" the reader
// cannot decode.
func applyLabelLimit(ticks []scene.Tick, limit *float64) []scene.Tick {
	if limit == nil || *limit <= 0 || len(ticks) == 0 {
		return ticks
	}
	max := *limit
	out := make([]scene.Tick, len(ticks))
	copy(out, ticks)
	for i := range out {
		out[i].Label = truncateToWidth(out[i].Label, max)
	}
	return out
}

// truncateToWidth shortens label so its estimated pixel width fits
// within max, appending an ellipsis when characters were dropped.
// Operates on runes so a multi-byte label is never cut mid-character.
func truncateToWidth(label string, max float64) string {
	if label == "" {
		return label
	}
	runes := []rune(label)
	if float64(len(runes))*axisLabelCharWidth <= max {
		return label
	}
	// Room for the ellipsis itself, or the label cannot be shown.
	keep := int(max/axisLabelCharWidth) - 1
	if keep < 1 {
		return ""
	}
	return string(runes[:keep]) + axisLabelEllipsis
}

// injectLinearMinorTicks inserts a Minor=true tick at each midpoint
// between consecutive majors. Returns the merged + sorted slice.
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

// applyLabelOverlap inspects ticks in pixel order and marks
// LabelHidden=true on every other major tick whose estimated label
// bbox overlaps its successor. Minor ticks are ignored (already
// label-less).
func applyLabelOverlap(ticks []scene.Tick, mode string, position scene.AxisPosition) []scene.Tick {
	if len(ticks) < 2 {
		return ticks
	}
	out := make([]scene.Tick, len(ticks))
	copy(out, ticks)
	// Approximate label dimensions: axisLabelCharWidth per character
	// horizontally, 12px tall vertically.
	const lineH = scene.LabelLineHeight
	const charW = axisLabelCharWidth
	var horizontal bool
	switch position {
	case scene.AxisPositionBottom, scene.AxisPositionTop:
		horizontal = true
	}
	var lastEnd float64
	first := true
	skip := false
	for i := range out {
		if out[i].Minor || out[i].Label == "" {
			continue
		}
		var start, end float64
		if horizontal {
			w := float64(len(out[i].Label)) * charW
			start = out[i].Pixel - w/2
			end = out[i].Pixel + w/2
		} else {
			start = out[i].Pixel - lineH/2
			end = out[i].Pixel + lineH/2
		}
		if first {
			lastEnd = end
			first = false
			continue
		}
		if start < lastEnd {
			if mode == "parity" {
				if !skip {
					skip = true
					out[i].LabelHidden = true
				} else {
					skip = false
					lastEnd = end
				}
				continue
			}
		}
		skip = false
		lastEnd = end
	}
	return out
}
