package encode

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
)

// DefaultTickCount is the tick-count hint the continuous generators
// use when the channel's `axis` block sets no `tick_count`. It is the
// literal BuildAxisWithOpts hard-coded before E3-S1 made the count
// author-addressable, so the default rendering is unchanged.
//
// Grid lines are emitted at exactly the *major* tick positions, so
// whatever shapes the tick set shapes the grid with it. That coupling
// is intentional — see docs/src/concepts/encoding.md.
const DefaultTickCount = 5

// toFloatValue coerces one `axis.values` entry to a domain number.
// Split out so the pinned-tick helpers can take it as a function
// value alongside the temporal coercion.
func toFloatValue(v any) (float64, bool) { return scale.ToFloat(v) }

// generatedLinearTicks produces the nice tick values for a linear
// domain, honouring both `tick_count` and `tick_min_step`.
//
// tick_min_step is enforced by *asking for fewer ticks* rather than by
// thinning the result, so the surviving values stay nice (0/5/10, not
// 0/4/8). Only when the generator cannot open the gap even at a single
// tick — a domain narrower than the requested step — does a greedy
// stride filter take over.
func generatedLinearTicks(lo, hi float64, opts AxisOpts) []float64 {
	count := opts.tickCount()
	if count <= 0 {
		return nil
	}
	ticks := NiceTicks(lo, hi, count)
	minStep := opts.TickMinStep
	if minStep <= 0 {
		return ticks
	}
	for count > 1 && tickGap(ticks) < minStep {
		count--
		ticks = NiceTicks(lo, hi, count)
	}
	if tickGap(ticks) < minStep {
		ticks = strideFloatsMinStep(ticks, minStep)
	}
	return ticks
}

// tickGap returns the spacing between the first two ticks, or +Inf
// when there are fewer than two (nothing can violate a minimum step).
func tickGap(ticks []float64) float64 {
	if len(ticks) < 2 {
		return math.Inf(1)
	}
	return math.Abs(ticks[1] - ticks[0])
}

// strideFloatsMinStep keeps the first value then every later value at
// least minStep from the last one kept.
func strideFloatsMinStep(ticks []float64, minStep float64) []float64 {
	if len(ticks) == 0 || minStep <= 0 {
		return ticks
	}
	out := []float64{ticks[0]}
	last := ticks[0]
	for _, v := range ticks[1:] {
		if math.Abs(v-last) < minStep {
			continue
		}
		out = append(out, v)
		last = v
	}
	return out
}

// filterTickMinStep is strideFloatsMinStep over already-built scene
// ticks, for the generators that do not take a count (log) or that
// generate in a transformed space (pow / sqrt / time). Minor ticks
// ride along with the major that precedes them rather than competing
// for the spacing budget.
func filterTickMinStep(ticks []scene.Tick, minStep float64) []scene.Tick {
	if len(ticks) == 0 || minStep <= 0 {
		return ticks
	}
	out := make([]scene.Tick, 0, len(ticks))
	last := math.NaN()
	for _, t := range ticks {
		if t.Minor {
			if !math.IsNaN(last) {
				out = append(out, t)
			}
			continue
		}
		v, ok := scale.ToFloat(t.Value)
		if !ok {
			out = append(out, t)
			continue
		}
		if !math.IsNaN(last) && math.Abs(v-last) < minStep {
			continue
		}
		out = append(out, t)
		last = v
	}
	return out
}

// thinLogTicks honours `tick_count` on a log axis. LogTicks is
// power-driven rather than count-driven, so the count acts as a
// ceiling: when the decades outnumber it, every k-th major survives
// and the mantissa minors are dropped wholesale (a minor between two
// non-adjacent majors reads as noise, not as a subdivision).
func thinLogTicks(ticks []scene.Tick, count int) []scene.Tick {
	if count <= 0 {
		return nil
	}
	majors := 0
	for _, t := range ticks {
		if !t.Minor {
			majors++
		}
	}
	if majors <= count {
		return ticks
	}
	stride := (majors + count - 1) / count
	out := make([]scene.Tick, 0, count)
	seen := 0
	for _, t := range ticks {
		if t.Minor {
			continue
		}
		if seen%stride == 0 {
			out = append(out, t)
		}
		seen++
	}
	return out
}

// pinnedNumericTicks coerces the author's `axis.values` into domain
// numbers, dropping every entry the axis cannot place: one the scale
// family cannot read, and one outside the resolved domain (which
// would otherwise be drawn off-plot). Survivors are sorted ascending
// so the downstream label-overlap pass still sees ticks in order.
// Casualties raise a single PRISM_WARN_AXIS_VALUES_DROPPED naming all
// of them.
func pinnedNumericTicks(opts AxisOpts, channel scene.Channel, lo, hi float64, coerce func(any) (float64, bool)) []float64 {
	if lo > hi {
		lo, hi = hi, lo
	}
	out := make([]float64, 0, len(opts.Values))
	var dropped []any
	for _, raw := range opts.Values {
		v, ok := coerce(raw)
		if !ok || v < lo || v > hi {
			dropped = append(dropped, raw)
			continue
		}
		out = append(out, v)
	}
	sort.Float64s(out)
	if len(dropped) > 0 {
		opts.warn(axisValuesDroppedWarning(channel, dropped, lo, hi))
	}
	return out
}

// pinnedScaleTicks is pinnedNumericTicks plus placement: it applies
// the scale to each surviving value and labels it with the supplied
// formatter. Used by the continuous families whose generators do not
// go through TicksWithLabels (log, pow, sqrt).
func pinnedScaleTicks(
	s Scale, opts AxisOpts, channel scene.Channel,
	lo, hi float64, coerce func(any) (float64, bool), label func(float64) string,
) []scene.Tick {
	values := pinnedNumericTicks(opts, channel, lo, hi, coerce)
	out := make([]scene.Tick, 0, len(values))
	for _, v := range values {
		pix, err := s.Apply(v)
		if err != nil {
			continue
		}
		out = append(out, scene.Tick{Value: v, Pixel: pix, Label: label(v)})
	}
	return out
}

// pinnedTimeTicks places the author's `axis.values` on a temporal
// axis. Entries may be ISO-8601 date strings, time.Time values, or
// numeric epoch milliseconds — whatever TimeScale.Apply accepts. The
// label layout is the one the automatic generator would have chosen
// for this domain's span, so pinning changes which ticks appear, not
// how they read.
func pinnedTimeTicks(s *TimeScale, opts AxisOpts, channel scene.Channel) []scene.Tick {
	if s == nil || s.Linear == nil {
		return nil
	}
	lo, hi := s.Linear.DomainMin, s.Linear.DomainMax
	values := pinnedNumericTicks(opts, channel, lo, hi, scale.ToEpochMs)
	_, layout := pickTimeLevel(time.UnixMilli(int64(hi)).Sub(time.UnixMilli(int64(lo))))
	out := make([]scene.Tick, 0, len(values))
	for _, epoch := range values {
		pix, err := s.Linear.Apply(epoch)
		if err != nil {
			continue
		}
		out = append(out, scene.Tick{
			Value: epoch,
			Pixel: pix,
			Label: time.UnixMilli(int64(epoch)).UTC().Format(layout),
		})
	}
	return out
}

// pinnedCategoryTicks narrows a discrete axis's ticks to the
// categories named by `axis.values`, preserving the scale's own
// category order. Names the domain does not contain are dropped with
// the same warning the continuous families use. With no `values` the
// ticks pass through untouched.
func pinnedCategoryTicks(ticks []scene.Tick, opts AxisOpts, channel scene.Channel) []scene.Tick {
	if !opts.pinned() {
		return ticks
	}
	want := make(map[string]bool, len(opts.Values))
	for _, raw := range opts.Values {
		want[categoryKey(raw)] = true
	}
	out := make([]scene.Tick, 0, len(opts.Values))
	matched := make(map[string]bool, len(opts.Values))
	for _, t := range ticks {
		key := categoryKey(t.Value)
		if !want[key] {
			continue
		}
		out = append(out, t)
		matched[key] = true
	}
	var dropped []any
	for _, raw := range opts.Values {
		if !matched[categoryKey(raw)] {
			dropped = append(dropped, raw)
		}
	}
	if len(dropped) > 0 {
		opts.warn(axisValuesDroppedWarning(channel, dropped, nil, nil))
	}
	return out
}

// categoryKey renders a discrete tick value (or a pinned entry) as the
// string the two are matched on.
func categoryKey(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// powTickLabel returns the labeller PowTicks uses: significant-figure
// rounding applied to the label only, so an inverted pow value reads
// as 63.25 rather than 63.245553203367585. The tick's Value and Pixel
// stay exact.
func powTickLabel(format string) func(float64) string {
	return func(v float64) string { return formatTick(roundSigFigs(v, 4), format) }
}

// axisValuesDroppedWarning builds PRISM_WARN_AXIS_VALUES_DROPPED. lo /
// hi are the domain bounds the values were measured against, or nil
// for a discrete axis whose domain is a category list.
func axisValuesDroppedWarning(channel scene.Channel, dropped []any, lo, hi any) scene.Warning {
	details := map[string]any{
		"Channel": string(channel),
		"Dropped": dropped,
		"Count":   len(dropped),
	}
	if lo != nil && hi != nil {
		details["DomainMin"] = lo
		details["DomainMax"] = hi
	}
	return scene.Warning{
		Code: scene.WarnAxisValuesDropped,
		Message: fmt.Sprintf(
			"%s axis: %d of the tick values pinned by axis.values cannot be placed on the resolved domain and were dropped (%v); their grid lines go with them.",
			channel, len(dropped), dropped),
		Details: details,
	}
}
