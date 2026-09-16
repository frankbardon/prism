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
	// TickSize / LabelPadding are the spec-level pixel overrides. Nil
	// defers to the theme's --prism-axis-tick-size /
	// --prism-axis-label-padding tokens, and then to the renderer's
	// built-in metrics.
	TickSize     *float64
	LabelPadding *float64
	// LabelLimit is the maximum label width in pixels before the label
	// is truncated with an ellipsis. Nil or non-positive means no
	// limit. Truncation happens here, at encode time, so every
	// renderer consuming the Scene IR agrees on the shortened text.
	LabelLimit *float64
	// Zindex draws the axis behind the marks at 0 (the default) and in
	// front of them at any positive value.
	Zindex int
}

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
		Zindex:       opts.Zindex,
	}

	switch s := scale.(type) {
	case *LinearScale:
		ticks := NiceTicks(s.DomainMin, s.DomainMax, 5)
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
		axis.Ticks = TimeTicks(s, 5)
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
		axis.Ticks = PowTicks(s, 5)
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScalePow,
			Domain: []any{s.DomainMin, s.DomainMax},
			Range:  [2]float64{s.RangeMin, s.RangeMax},
			Exp:    s.Exp,
		}
	case *SqrtScale:
		axis.Ticks = SqrtTicks(s, 5)
		axis.Scale = scene.ScaleSpec{
			Type:   scene.ScaleSqrt,
			Domain: []any{s.Inner.DomainMin, s.Inner.DomainMax},
			Range:  [2]float64{s.Inner.RangeMin, s.Inner.RangeMax},
			Exp:    0.5,
		}
	case *BandScale:
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
// metrics.
const axisLabelCharWidth = 6.0

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
	const lineH = 12.0
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
