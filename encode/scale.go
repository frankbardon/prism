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

	// Scheme is `scale.scheme`: a named color scheme from the global
	// catalogue or the theme's own registry. Read by the palette
	// cascade (ResolveCategoricalPaletteWithOpts and its sequential
	// twin), never by the numeric scale resolvers.
	Scheme string

	// Range is `scale.range` in its inline-color-list form, lifted
	// only when every entry is a string. It outranks Scheme and the
	// theme's Range slots, and is honoured on **color channels
	// only** — on a position channel it is rejected at validate with
	// PRISM_SPEC_044, because a range that disagrees with the plot
	// rect encode/layout.go computes would desynchronise axes,
	// gridlines and marks.
	Range []string

	// Interpolate is `scale.interpolate`: the colorspace a
	// continuous ramp is traversed in — "rgb" (default), "hsl" or
	// "lab". Applied once, at encode time, by resampling the ramp
	// (see ResampleRamp) so nothing downstream needs colorspace math.
	Interpolate string

	// Clamp is `scale.clamp`: pin a value outside the resolved domain
	// to the nearest domain edge rather than letting it map past the
	// range. nil / false is the default — the value overflows and the
	// plot rect clips it. Continuous families only; a band / point /
	// ordinal scale has no out-of-domain image to pin (an unknown
	// category is an error, not an overflow).
	Clamp *bool

	// Reverse is `scale.reverse`: run the scale the other way round.
	// It is defined *relative to the channel's default orientation*,
	// not to an absolute screen direction — a y scale already maps the
	// domain minimum to the bottom of the plot, so reverse:true puts
	// the minimum at the top; on x it swaps left for right.
	//
	// Continuous families implement it by flipping the pixel range
	// (see pixelRange), which carries axes, ticks and gridlines along
	// automatically because all three are placed through Scale.Apply.
	// Discrete families instead hand the slots to the categories back
	// to front — d3-scale's own band behaviour — so band widths and
	// the signed step are untouched and inverted-y marks keep working.
	Reverse *bool

	// Round is `scale.round`: quantise layout to whole pixels. On a
	// band / point scale it floors the step and rounds the leading
	// offset and the band width, which is what makes band edges crisp;
	// on a continuous scale it rounds the resolved pixel. It is a
	// layout knob, independent of render/precision.go's 3-decimal
	// serialisation pinning — both can apply.
	Round *bool

	// Padding is `scale.padding`, the shorthand: on a band scale it
	// sets PaddingInner and PaddingOuter together, on a point scale it
	// sets the (only) outer padding. An explicit padding_inner /
	// padding_outer outranks it.
	Padding *float64

	// PaddingInner is `scale.padding_inner`: the gap between adjacent
	// bands, as a fraction of the step, in [0,1). Band scales only.
	PaddingInner *float64

	// PaddingOuter is `scale.padding_outer`: the gap before the first
	// and after the last band / point, as a fraction of the step.
	PaddingOuter *float64

	// Align is `scale.align` in [0,1]: where the slack left over after
	// the bands are laid out sits. 0 packs them against the range
	// start, 1 against the end, 0.5 (the default) centres them.
	Align *float64
}

// Band and point geometry defaults. The band trio matches Vega-Lite
// (inner 0.1, outer 0.05) *and* reproduces Prism's historic band
// layout byte-for-byte: the pre-split scale spent a half-inner-gap at
// each end of the range, which is exactly what outer = inner/2 with
// align = 0.5 produces. Changing any of the three moves every bar,
// tick, heatmap and boxplot golden.
const (
	defaultBandPaddingInner = 0.1
	defaultBandPaddingOuter = 0.05
	defaultPointPadding     = 0.5
	defaultScaleAlign       = 0.5
	// maxInnerPadding is the largest inner padding that still leaves
	// a band something to draw.
	maxInnerPadding = 0.99
)

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
	opts.Scheme = sc.Scheme
	opts.Range = colorRangeList(sc.Range)
	opts.Interpolate = sc.Interpolate
	opts.Clamp = copyBool(sc.Clamp)
	opts.Reverse = copyBool(sc.Reverse)
	opts.Round = copyBool(sc.Round)
	opts.Padding = copyFloat(sc.Padding)
	opts.PaddingInner = copyFloat(sc.PaddingInner)
	opts.PaddingOuter = copyFloat(sc.PaddingOuter)
	opts.Align = copyFloat(sc.Align)
	return opts
}

// copyBool detaches an optional flag from the spec block so a later
// mutation of the spec cannot reach the resolved scale.
func copyBool(v *bool) *bool {
	if v == nil {
		return nil
	}
	b := *v
	return &b
}

// copyFloat is copyBool for the optional float knobs.
func copyFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	f := *v
	return &f
}

// colorRangeList lifts `scale.range` into the inline color list the
// palette cascade consumes. The wire type is `any` because Vega-Lite
// also allows a string (a named range reference) and numeric arrays
// (size / opacity ranges); Prism honours neither, so anything that is
// not a list of strings reads as "no explicit range".
func colorRangeList(v any) []string {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		s, ok := it.(string)
		if !ok {
			return nil
		}
		out = append(out, s)
	}
	return out
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

// output returns the clamp / round policy the continuous scale impls
// embed.
func (o ScaleOpts) output() scale.ContinuousOutput {
	return scale.ContinuousOutput{Clamp: o.clampEnabled(), Round: o.roundEnabled()}
}

// clampEnabled reports whether out-of-domain values are pinned to the
// domain edge. Off unless the author asks.
func (o ScaleOpts) clampEnabled() bool { return o.Clamp != nil && *o.Clamp }

// roundEnabled reports whether layout is quantised to whole pixels.
func (o ScaleOpts) roundEnabled() bool { return o.Round != nil && *o.Round }

// reverseEnabled reports whether the scale runs the other way round.
func (o ScaleOpts) reverseEnabled() bool { return o.Reverse != nil && *o.Reverse }

// pixelRange applies `scale.reverse` to a continuous scale's pixel
// range. Discrete families do NOT come through here — they reverse the
// slot assignment instead, so their signed step survives.
func (o ScaleOpts) pixelRange(rangeMin, rangeMax float64) (float64, float64) {
	if o.reverseEnabled() {
		return rangeMax, rangeMin
	}
	return rangeMin, rangeMax
}

// bandPadding resolves a band scale's (inner, outer, align) triple:
// the defaults first, then the `padding` shorthand, then the explicit
// padding_inner / padding_outer / align. Values are pinned into their
// legal ranges so a spec that skipped validation still yields drawable
// geometry.
func (o ScaleOpts) bandPadding() (inner, outer, align float64) {
	inner, outer, align = defaultBandPaddingInner, defaultBandPaddingOuter, defaultScaleAlign
	if o.Padding != nil {
		inner, outer = *o.Padding, *o.Padding
	}
	if o.PaddingInner != nil {
		inner = *o.PaddingInner
	}
	if o.PaddingOuter != nil {
		outer = *o.PaddingOuter
	}
	if o.Align != nil {
		align = *o.Align
	}
	return pinInnerPadding(inner), pinNonNegative(outer), pinUnit(align)
}

// pointPadding resolves a point scale's (outer, align) pair. A point
// scale collapses every band to a point, so it has no inner padding
// and `padding_inner` never reaches it.
func (o ScaleOpts) pointPadding() (outer, align float64) {
	outer, align = defaultPointPadding, defaultScaleAlign
	if o.Padding != nil {
		outer = *o.Padding
	}
	if o.PaddingOuter != nil {
		outer = *o.PaddingOuter
	}
	if o.Align != nil {
		align = *o.Align
	}
	return pinNonNegative(outer), pinUnit(align)
}

// pinUnit pins v into [0,1].
func pinUnit(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// pinInnerPadding pins v into [0,1): an inner padding of exactly 1
// would collapse every band to zero width.
func pinInnerPadding(v float64) float64 {
	if v = pinUnit(v); v >= 1 {
		return maxInnerPadding
	}
	return v
}

// pinNonNegative pins v into [0, +inf).
func pinNonNegative(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return v
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
	rangeMin, rangeMax = opts.pixelRange(rangeMin, rangeMax)
	lo, hi, pinned, err := opts.numericDomain("linear")
	if err != nil {
		return nil, nil, err
	}
	if pinned {
		return newLinearScale(lo, hi, rangeMin, rangeMax, opts), nil, nil
	}
	if len(values) == 0 {
		return newLinearScale(0, 1, rangeMin, rangeMax, opts), nil, nil
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
	return newLinearScale(mn, mx, rangeMin, rangeMax, opts), nil, nil
}

// newLinearScale is the single LinearScale constructor that folds in
// the clamp / round output policy. The pixel range arrives already
// flipped by pixelRange when `scale.reverse` is on.
func newLinearScale(domainMin, domainMax, rangeMin, rangeMax float64, opts ScaleOpts) *LinearScale {
	return &LinearScale{
		DomainMin:        domainMin,
		DomainMax:        domainMax,
		RangeMin:         rangeMin,
		RangeMax:         rangeMax,
		ContinuousOutput: opts.output(),
	}
}

// NewBandScale builds a band scale over cats, folding in the spec's
// padding / align / round / reverse knobs. It is the one band
// constructor: the flat encoder and the shared-scale path in
// encode_composite.go both come through here so a spec knob can never
// apply to one and not the other.
func NewBandScale(cats []string, rangeMin, rangeMax float64, opts ScaleOpts) *BandScale {
	inner, outer, align := opts.bandPadding()
	return &BandScale{
		Categories:   cats,
		RangeMin:     rangeMin,
		RangeMax:     rangeMax,
		PaddingInner: inner,
		PaddingOuter: outer,
		Align:        align,
		Round:        opts.roundEnabled(),
		Reverse:      opts.reverseEnabled(),
	}
}

// NewPointScale is NewBandScale's point-family twin.
func NewPointScale(cats []string, rangeMin, rangeMax float64, opts ScaleOpts) *PointScale {
	outer, align := opts.pointPadding()
	return &PointScale{
		Categories: cats,
		RangeMin:   rangeMin,
		RangeMax:   rangeMax,
		Padding:    outer,
		Align:      align,
		Round:      opts.roundEnabled(),
		Reverse:    opts.reverseEnabled(),
	}
}

func resolveBand(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	cats, err := bandCategories(values, opts, "resolveBand", "<band>")
	if err != nil {
		return nil, nil, err
	}
	return NewBandScale(cats, rangeMin, rangeMax, opts), nil, nil
}

func resolvePoint(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	cats, err := bandCategories(values, opts, "resolvePoint", "<point>")
	if err != nil {
		return nil, nil, err
	}
	return NewPointScale(cats, rangeMin, rangeMax, opts), nil, nil
}

func resolveOrdinal(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	cats, err := bandCategories(values, opts, "resolveOrdinal", "<ordinal>")
	if err != nil {
		return nil, nil, err
	}
	return newOrdinalScale(cats, rangeMin, rangeMax, opts), nil, nil
}

// newOrdinalScale evenly distributes positions across the range. An
// ordinal scale pins positions up front rather than deriving them per
// lookup, so `reverse` and `round` are baked in here.
func newOrdinalScale(cats []string, rangeMin, rangeMax float64, opts ScaleOpts) *OrdinalScale {
	positions := make([]float64, len(cats))
	if len(cats) == 1 {
		positions[0] = (rangeMin + rangeMax) / 2
	} else {
		step := (rangeMax - rangeMin) / float64(len(cats)-1)
		for i := range cats {
			positions[i] = rangeMin + step*float64(i)
		}
	}
	if opts.reverseEnabled() {
		for i, j := 0, len(positions)-1; i < j; i, j = i+1, j-1 {
			positions[i], positions[j] = positions[j], positions[i]
		}
	}
	if opts.roundEnabled() {
		for i := range positions {
			positions[i] = math.Round(positions[i])
		}
	}
	return &OrdinalScale{Categories: cats, Positions: positions}
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
	rangeMin, rangeMax = opts.pixelRange(rangeMin, rangeMax)
	lo, hi, pinned, err := opts.temporalDomain()
	if err != nil {
		return nil, nil, err
	}
	if pinned {
		return &TimeScale{Linear: newLinearScale(lo, hi, rangeMin, rangeMax, opts)}, nil, nil
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
	lin := newLinearScale(mn, mx, rangeMin, rangeMax, opts)
	// T06.04: calendar-aware ticks live in encode/ticks_time.go;
	// drop the stub warning that P05 emitted.
	return &TimeScale{Linear: lin}, nil, nil
}

func resolveLog(values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	rangeMin, rangeMax = opts.pixelRange(rangeMin, rangeMax)
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
		return &LogScale{
			Base:             base,
			DomainMin:        lo,
			DomainMax:        hi,
			RangeMin:         rangeMin,
			RangeMax:         rangeMax,
			ContinuousOutput: opts.output(),
		}, nil, nil
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
		Base:             base,
		DomainMin:        mn,
		DomainMax:        mx,
		RangeMin:         rangeMin,
		RangeMax:         rangeMax,
		ContinuousOutput: opts.output(),
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
	rangeMin, rangeMax = opts.pixelRange(rangeMin, rangeMax)
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
		return powScale(exp, lo, hi, rangeMin, rangeMax, opts), nil, nil
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
	return powScale(exp, mn, mx, rangeMin, rangeMax, opts), nil, nil
}

func powScale(exp, mn, mx, rangeMin, rangeMax float64, opts ScaleOpts) Scale {
	inner := PowScale{
		Exp:              exp,
		DomainMin:        mn,
		DomainMax:        mx,
		RangeMin:         rangeMin,
		RangeMax:         rangeMax,
		ContinuousOutput: opts.output(),
	}
	if exp == 0.5 {
		return &SqrtScale{Inner: inner}
	}
	return &inner
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
