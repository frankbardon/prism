package encode

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Scale is the interface satisfied by every per-type scale impl. Re-
// exported from encode/scale so existing call sites compile unchanged.
type Scale = scale.Scale

// LinearScale is re-exported from encode/scale for back-compat.
type LinearScale = scale.LinearScale

// BandScale is re-exported from encode/scale for back-compat.
type BandScale = scale.BandScale

// OrdinalScale is re-exported from encode/scale for back-compat.
type OrdinalScale = scale.OrdinalScale

// TimeScale is re-exported from encode/scale for back-compat.
type TimeScale = scale.TimeScale

// LogScale is re-exported from encode/scale.
type LogScale = scale.LogScale

// PowScale is re-exported from encode/scale.
type PowScale = scale.PowScale

// SqrtScale is re-exported from encode/scale.
type SqrtScale = scale.SqrtScale

// PointScale is re-exported from encode/scale.
type PointScale = scale.PointScale

// defaultNiceCount is the tick-count hint used when rounding a domain
// to nice bounds. It matches the tick count BuildAxisWithOpts asks for
// so the niced bounds coincide with the outermost tick.
const defaultNiceCount = DefaultTickCount

// ResolveScale picks the right Scale impl for a channel + column
// kind, computes its domain from the values, and returns the
// resulting Scale + an optional warning. The rangeMin / rangeMax span
// the plot region for the channel's orientation; for y-axes the
// renderer is responsible for the "invert-y" flip via passing
// (rangeMax, rangeMin).
//
// Domain shaping (explicit scale.domain, scale.zero, scale.nice) falls
// back to the defaults; callers holding a spec scale block should use
// ResolveScaleWithOpts instead.
func ResolveScale(channelType string, kind table.Kind, values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	return ResolveScaleWithOpts(channelType, kind, values, rangeMin, rangeMax, ScaleOpts{})
}

// ResolveScaleWithOpts is ResolveScale with the spec's scale block
// folded in (explicit domain, zero inclusion, nice rounding, log base,
// pow exponent). It is the inference path: the scale family still
// comes from the channel type / column kind.
func ResolveScaleWithOpts(channelType string, kind table.Kind, values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	if channelType == "" {
		switch kind {
		case table.KindString:
			return resolveBand(values, rangeMin, rangeMax, opts)
		case table.KindDate:
			return resolveTime(values, rangeMin, rangeMax, opts)
		default:
			return resolveLinear(values, rangeMin, rangeMax, opts)
		}
	}
	switch channelType {
	case "quantitative":
		return resolveLinear(values, rangeMin, rangeMax, opts)
	case "nominal", "ordinal":
		return resolveBand(values, rangeMin, rangeMax, opts)
	case "temporal":
		return resolveTime(values, rangeMin, rangeMax, opts)
	}
	return nil, nil, fmt.Errorf("ResolveScale: unknown channel type %q", channelType)
}

// ResolveScaleTyped accepts an explicit scene.ScaleType (from the
// spec's `scale.type` field) and resolves the appropriate impl. Used
// by the encoder once the spec carries a scale.type override.
func ResolveScaleTyped(scaleType scene.ScaleType, values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	switch scaleType {
	case scene.ScaleLinear:
		return resolveLinear(values, rangeMin, rangeMax, opts)
	case scene.ScaleLog:
		return resolveLog(values, rangeMin, rangeMax, opts)
	case scene.ScalePow:
		return resolvePow(values, rangeMin, rangeMax, opts.Exp, opts)
	case scene.ScaleSqrt:
		return resolvePow(values, rangeMin, rangeMax, 0.5, opts)
	case scene.ScaleTime:
		return resolveTime(values, rangeMin, rangeMax, opts)
	case scene.ScaleBand:
		return resolveBand(values, rangeMin, rangeMax, opts)
	case scene.ScalePoint:
		return resolvePoint(values, rangeMin, rangeMax, opts)
	case scene.ScaleOrdinal:
		return resolveOrdinal(values, rangeMin, rangeMax, opts)
	}
	return nil, nil, fmt.Errorf("ResolveScaleTyped: unknown scale type %q", scaleType)
}

// ScaleOpts carries the per-scale knobs read off a spec scale block.
//
// The three domain-shaping knobs compose in a fixed precedence: an
// explicit Domain wins outright (Zero and Nice are not applied on top
// of it); otherwise the data extent is widened to include zero when
// Zero is on, and the result is rounded outward when Nice is on.
type ScaleOpts struct {
	Base float64 // log base (default 10 if zero)
	Exp  float64 // pow exponent (default 1 if zero)

	// Domain pins the scale domain outright. Continuous scales expect
	// exactly two bounds (numbers, or ISO-8601 / epoch-ms values on a
	// time scale); band / point / ordinal scales expect the explicit
	// category order. A malformed domain is rejected with
	// PRISM_SPEC_041 — the same code validate reports statically.
	Domain []any

	// Zero forces zero into a continuous domain. nil means the
	// default: on for linear / pow / sqrt, and never for log (a log
	// domain cannot contain zero) or time.
	Zero *bool

	// Nice rounds the resolved domain outward to human bounds. nil
	// means the default: on for linear / pow / sqrt / time, off for
	// log (whose ticks are already base-aligned).
	Nice *bool

	// NiceCount is the tick-count hint used when rounding. Zero means
	// defaultNiceCount, matching the axis builder's tick count.
	NiceCount int
}

// ScaleOptsFromSpec lifts a spec scale block into ScaleOpts. A nil
// block yields the zero value, which reads as "all defaults".
func ScaleOptsFromSpec(sc *spec.Scale) ScaleOpts {
	var opts ScaleOpts
	if sc == nil {
		return opts
	}
	if sc.Base != nil {
		opts.Base = *sc.Base
	}
	if sc.Exponent != nil {
		opts.Exp = *sc.Exponent
	}
	if sc.Zero != nil {
		z := *sc.Zero
		opts.Zero = &z
	}
	// `nice` is boolean-only on the wire (the schema rejects the
	// Vega-Lite numeric tick-count form); use axis.tick_count for
	// tick density.
	if n, ok := sc.Nice.(bool); ok {
		opts.Nice = &n
	}
	if dom, ok := sc.Domain.([]any); ok {
		opts.Domain = dom
	}
	return opts
}

// zeroEnabled reports whether zero-forcing runs, given the per-family
// default.
func (o ScaleOpts) zeroEnabled(def bool) bool {
	if o.Zero != nil {
		return *o.Zero
	}
	return def
}

// niceEnabled reports whether domain rounding runs, given the
// per-family default.
func (o ScaleOpts) niceEnabled(def bool) bool {
	if o.Nice != nil {
		return *o.Nice
	}
	return def
}

func (o ScaleOpts) niceCount() int {
	if o.NiceCount > 0 {
		return o.NiceCount
	}
	return defaultNiceCount
}

// shapeContinuous applies zero-forcing then nice rounding to a
// data-derived extent.
func (o ScaleOpts) shapeContinuous(mn, mx float64, zeroDefault, niceDefault bool) (float64, float64) {
	if o.zeroEnabled(zeroDefault) {
		if mn > 0 {
			mn = 0
		}
		if mx < 0 {
			mx = 0
		}
	}
	if o.niceEnabled(niceDefault) {
		mn, mx = niceDomain(mn, mx, o.niceCount())
	}
	return mn, mx
}

// numericDomain converts an explicit Domain into [lo, hi] floats.
// Returns ok=false when no domain is declared, and an error when one
// is declared but malformed.
func (o ScaleOpts) numericDomain(family string) (float64, float64, bool, error) {
	if len(o.Domain) == 0 {
		return 0, 0, false, nil
	}
	if len(o.Domain) != 2 {
		return 0, 0, false, domainError(family,
			fmt.Sprintf("a %s scale needs exactly 2 domain bounds, got %d", family, len(o.Domain)))
	}
	lo, ok1 := scale.ToFloat(o.Domain[0])
	hi, ok2 := scale.ToFloat(o.Domain[1])
	if !ok1 || !ok2 {
		return 0, 0, false, domainError(family,
			fmt.Sprintf("a %s scale needs numeric domain bounds, got [%v, %v]", family, o.Domain[0], o.Domain[1]))
	}
	if lo >= hi {
		return 0, 0, false, domainError(family,
			fmt.Sprintf("domain bounds must ascend, got [%v, %v]", o.Domain[0], o.Domain[1]))
	}
	return lo, hi, true, nil
}

// temporalDomain converts an explicit Domain into [lo, hi] epoch-ms.
func (o ScaleOpts) temporalDomain() (float64, float64, bool, error) {
	if len(o.Domain) == 0 {
		return 0, 0, false, nil
	}
	if len(o.Domain) != 2 {
		return 0, 0, false, domainError("time",
			fmt.Sprintf("a time scale needs exactly 2 domain bounds, got %d", len(o.Domain)))
	}
	lo, ok1 := scale.ToEpochMs(o.Domain[0])
	hi, ok2 := scale.ToEpochMs(o.Domain[1])
	if !ok1 || !ok2 {
		return 0, 0, false, domainError("time",
			fmt.Sprintf("a time scale needs ISO-8601 or epoch-ms domain bounds, got [%v, %v]", o.Domain[0], o.Domain[1]))
	}
	if lo >= hi {
		return 0, 0, false, domainError("time",
			fmt.Sprintf("domain bounds must ascend, got [%v, %v]", o.Domain[0], o.Domain[1]))
	}
	return lo, hi, true, nil
}

// categoryDomain returns the explicit category order, with any data
// category the domain does not list appended in first-seen order so
// no row is silently dropped.
func (o ScaleOpts) categoryDomain(values []any) ([]string, error) {
	if len(o.Domain) == 0 {
		return nil, nil
	}
	cats := make([]string, 0, len(o.Domain))
	seen := map[string]bool{}
	for _, v := range o.Domain {
		s, ok := v.(string)
		if !ok {
			return nil, domainError("categorical",
				fmt.Sprintf("a band / point / ordinal domain lists string categories, got %v", v))
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		cats = append(cats, s)
	}
	for _, s := range uniqueStrings(values) {
		if !seen[s] {
			seen[s] = true
			cats = append(cats, s)
		}
	}
	return cats, nil
}

func domainError(family, reason string) error {
	return prismerrors.New(
		"PRISM_SPEC_041",
		fmt.Sprintf("Explicit scale.domain is malformed: %s.", reason),
		map[string]any{"Family": family, "Reason": reason},
	)
}

// niceDomain rounds [mn, mx] outward to the nearest multiples of the
// tick step the axis builder would pick. Mirrors D3's
// d3-scale#linear.nice (iterating until the step stabilises).
func niceDomain(mn, mx float64, count int) (float64, float64) {
	if mn == mx || math.IsNaN(mn) || math.IsNaN(mx) || math.IsInf(mn, 0) || math.IsInf(mx, 0) {
		return mn, mx
	}
	reverse := mn > mx
	if reverse {
		mn, mx = mx, mn
	}
	prestep := math.NaN()
	for i := 0; i < 10; i++ {
		step := tickIncrement(mn, mx, count)
		if step == prestep || step == 0 || math.IsNaN(step) || math.IsInf(step, 0) {
			break
		}
		if step > 0 {
			mn = math.Floor(mn/step) * step
			mx = math.Ceil(mx/step) * step
		} else {
			mn = math.Ceil(mn*step) / step
			mx = math.Floor(mx*step) / step
		}
		prestep = step
	}
	if reverse {
		mn, mx = mx, mn
	}
	return mn, mx
}

// numericExtent folds values into a [min, max] pair, skipping
// non-numeric entries. ok is false when nothing numeric was found.
func numericExtent(values []any) (mn float64, mx float64, ok bool) {
	first := true
	for _, v := range values {
		f, good := scale.ToFloat(v)
		if !good {
			continue
		}
		if first {
			mn, mx = f, f
			first = false
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

func resolveLinear(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	lo, hi, pinned, err := opts.numericDomain("linear")
	if err != nil {
		return nil, nil, err
	}
	if pinned {
		return &LinearScale{DomainMin: lo, DomainMax: hi, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
	}
	if len(values) == 0 {
		return &LinearScale{DomainMin: 0, DomainMax: 1, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
	}
	mn, mx, ok := numericExtent(values)
	if !ok {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolveLinear: no numeric values in domain.",
			map[string]any{"Field": "<linear>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	mn, mx = opts.shapeContinuous(mn, mx, true, true)
	return &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
}

func resolveBand(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	cats, err := bandCategories(values, opts, "resolveBand", "<band>")
	if err != nil {
		return nil, nil, err
	}
	return &BandScale{
		Categories: cats,
		RangeMin:   rangeMin,
		RangeMax:   rangeMax,
		Padding:    0.1,
	}, nil, nil
}

func resolvePoint(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	cats, err := bandCategories(values, opts, "resolvePoint", "<point>")
	if err != nil {
		return nil, nil, err
	}
	return &PointScale{
		Categories: cats,
		RangeMin:   rangeMin,
		RangeMax:   rangeMax,
		Padding:    0.5,
	}, nil, nil
}

func resolveOrdinal(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	cats, err := bandCategories(values, opts, "resolveOrdinal", "<ordinal>")
	if err != nil {
		return nil, nil, err
	}
	// Evenly distribute positions across [rangeMin, rangeMax].
	positions := make([]float64, len(cats))
	if len(cats) == 1 {
		positions[0] = (rangeMin + rangeMax) / 2
	} else {
		step := (rangeMax - rangeMin) / float64(len(cats)-1)
		for i := range cats {
			positions[i] = rangeMin + step*float64(i)
		}
	}
	return &OrdinalScale{Categories: cats, Positions: positions}, nil, nil
}

// bandCategories resolves the category order for a discrete scale: an
// explicit scale.domain pins the leading order, otherwise the
// first-seen data order stands.
func bandCategories(values []any, opts ScaleOpts, fn, field string) ([]string, error) {
	cats, err := opts.categoryDomain(values)
	if err != nil {
		return nil, err
	}
	if len(cats) == 0 {
		cats = uniqueStrings(values)
	}
	if len(cats) == 0 {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fn+": no string categories in domain.",
			map[string]any{"Field": field, "Source": "<scale>", "Available": "string"},
		)
	}
	return cats, nil
}

func resolveTime(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	lo, hi, pinned, err := opts.temporalDomain()
	if err != nil {
		return nil, nil, err
	}
	if pinned {
		lin := &LinearScale{DomainMin: lo, DomainMax: hi, RangeMin: rangeMin, RangeMax: rangeMax}
		return &TimeScale{Linear: lin}, nil, nil
	}
	var mn, mx float64
	first := true
	for _, v := range values {
		e, ok := scale.ToEpochMs(v)
		if !ok {
			continue
		}
		if first {
			mn, mx = e, e
			first = false
			continue
		}
		if e < mn {
			mn = e
		}
		if e > mx {
			mx = e
		}
	}
	if first {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolveTime: no parseable time values in domain.",
			map[string]any{"Field": "<time>", "Source": "<scale>", "Available": "iso-8601 | time.Time | epoch_ms"},
		)
	}
	// Zero-forcing is meaningless on a calendar axis (epoch 0 is
	// 1970-01-01), so only `nice` applies here — and it rounds to the
	// calendar boundary the tick generator would pick rather than to a
	// decimal multiple.
	if opts.niceEnabled(true) {
		mn, mx = niceTimeDomain(mn, mx)
	}
	lin := &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}
	// T06.04: calendar-aware ticks live in encode/ticks_time.go;
	// drop the stub warning that P05 emitted.
	return &TimeScale{Linear: lin}, nil, nil
}

func resolveLog(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	base := opts.Base
	if base == 0 {
		base = 10
	}
	lo, hi, pinned, err := opts.numericDomain("log")
	if err != nil {
		return nil, nil, err
	}
	if pinned {
		if lo <= 0 {
			return nil, nil, prismerrors.New(
				"PRISM_SPEC_010",
				fmt.Sprintf("Log scale requires a positive domain; got %v.", lo),
				map[string]any{"Value": lo, "ScaleType": "log"},
			)
		}
		return &LogScale{Base: base, DomainMin: lo, DomainMax: hi, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
	}
	var mn, mx float64
	first := true
	for _, v := range values {
		f, ok := scale.ToFloat(v)
		if !ok {
			continue
		}
		if f <= 0 {
			return nil, nil, prismerrors.New(
				"PRISM_SPEC_010",
				fmt.Sprintf("Log scale requires positive domain values; got %v.", f),
				map[string]any{"Value": f, "ScaleType": "log"},
			)
		}
		if first {
			mn, mx = f, f
			first = false
			continue
		}
		if f < mn {
			mn = f
		}
		if f > mx {
			mx = f
		}
	}
	if first {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolveLog: no numeric values in domain.",
			map[string]any{"Field": "<log>", "Source": "<scale>", "Available": "positive numeric"},
		)
	}
	// A log domain can never contain zero, so `zero` is not honoured
	// here — by design, and it is the reason scale.type "log" used to
	// be the only escape from a zero-based axis. `nice` defaults
	// off because log ticks are already base-aligned; an explicit
	// nice:true rounds outward to whole powers of the base.
	if opts.niceEnabled(false) {
		mn, mx = niceLogDomain(mn, mx, base)
	}
	return &LogScale{
		Base:      base,
		DomainMin: mn,
		DomainMax: mx,
		RangeMin:  rangeMin,
		RangeMax:  rangeMax,
	}, nil, nil
}

// niceLogDomain rounds a positive domain outward to whole powers of
// base.
func niceLogDomain(mn, mx, base float64) (float64, float64) {
	if mn <= 0 || mx <= 0 || base <= 1 {
		return mn, mx
	}
	lo := math.Pow(base, math.Floor(math.Log(mn)/math.Log(base)))
	hi := math.Pow(base, math.Ceil(math.Log(mx)/math.Log(base)))
	if lo <= 0 || math.IsInf(hi, 0) || math.IsNaN(lo) || math.IsNaN(hi) {
		return mn, mx
	}
	return lo, hi
}

func resolvePow(values []any, rangeMin, rangeMax, exp float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	if exp == 0 {
		exp = 1
	}
	family := "pow"
	if exp == 0.5 {
		family = "sqrt"
	}
	lo, hi, pinned, err := opts.numericDomain(family)
	if err != nil {
		return nil, nil, err
	}
	if pinned {
		return powScale(exp, lo, hi, rangeMin, rangeMax), nil, nil
	}
	mn, mx, ok := numericExtent(values)
	if !ok {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolvePow: no numeric values in domain.",
			map[string]any{"Field": "<pow>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	mn, mx = opts.shapeContinuous(mn, mx, true, true)
	return powScale(exp, mn, mx, rangeMin, rangeMax), nil, nil
}

func powScale(exp, mn, mx, rangeMin, rangeMax float64) Scale {
	if exp == 0.5 {
		return &SqrtScale{Inner: PowScale{Exp: 0.5, DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}}
	}
	return &PowScale{Exp: exp, DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}
}

func uniqueStrings(values []any) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		s, ok := v.(string)
		if !ok || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
