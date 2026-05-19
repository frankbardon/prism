package encode

import (
	"github.com/frankbardon/prism/encode/scene"
)

// BuildAxis converts a resolved Scale into a populated scene.Axis,
// including ticks, the axis domain line, and grid lines anchored to
// the plot region.
//
// Tick generation:
//   - Linear / Time scales use NiceTicks(min, max, ~5) and
//     TicksWithLabels.
//   - Band scales use BandTicks (one tick per category, centered).
//   - Ordinal scales use one tick per category at its declared
//     position.
func BuildAxis(scale Scale, channel scene.Channel, position scene.AxisPosition, plot scene.Rect, title string) scene.Axis {
	axis := scene.Axis{
		ID:       string(channel) + "-axis",
		Channel:  channel,
		Position: position,
		Title:    title,
	}

	switch s := scale.(type) {
	case *LinearScale:
		ticks := NiceTicks(s.DomainMin, s.DomainMax, 5)
		labelled, err := TicksWithLabels(ticks, s, "")
		if err == nil {
			axis.Ticks = labelled
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

	// Domain line + grid lines. Position controls geometry:
	//   - bottom / top: domain is horizontal at y=plot.Bottom/plot.Y,
	//     grid lines are vertical (one per tick).
	//   - left / right: domain is vertical, grid lines are horizontal.
	switch position {
	case scene.AxisPositionBottom:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Bottom(), X2: plot.Right(), Y2: plot.Bottom()}
		grid := make([]scene.Line, 0, len(axis.Ticks))
		for _, t := range axis.Ticks {
			grid = append(grid, scene.Line{X1: t.Pixel, Y1: plot.Y, X2: t.Pixel, Y2: plot.Bottom()})
		}
		axis.Grid = grid
	case scene.AxisPositionTop:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Y, X2: plot.Right(), Y2: plot.Y}
	case scene.AxisPositionLeft:
		axis.Domain = scene.Line{X1: plot.X, Y1: plot.Y, X2: plot.X, Y2: plot.Bottom()}
		grid := make([]scene.Line, 0, len(axis.Ticks))
		for _, t := range axis.Ticks {
			grid = append(grid, scene.Line{X1: plot.X, Y1: t.Pixel, X2: plot.Right(), Y2: t.Pixel})
		}
		axis.Grid = grid
	case scene.AxisPositionRight:
		axis.Domain = scene.Line{X1: plot.Right(), Y1: plot.Y, X2: plot.Right(), Y2: plot.Bottom()}
	}

	return axis
}
