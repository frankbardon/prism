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
}

// DefaultAxisOpts returns the P06 defaults.
func DefaultAxisOpts(title string) AxisOpts {
	return AxisOpts{
		Title:        title,
		Grid:         true,
		LabelAngle:   0,
		LabelOverlap: "parity",
		MinorTicks:   true,
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
		ID:         string(channel) + "-axis",
		Channel:    channel,
		Position:   position,
		Title:      opts.Title,
		LabelAngle: opts.LabelAngle,
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
	// Approximate label dimensions: 6px per character horizontally,
	// 12px tall vertically.
	const charW, lineH = 6.0, 12.0
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
