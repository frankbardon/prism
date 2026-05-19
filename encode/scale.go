// Package encode bridges from materialised tables (output of the
// plan-execute stage) to a renderable Scene IR (consumed by
// render/svg, render/pdf, render/canvas). It owns scale resolution,
// nice-tick computation, layout math, and per-mark pixel resolution.
// Renderers below stay dumb — they map coordinates already in pixel
// space, not data space.
//
// See design/02-architecture.md § Stage 5 (Encode) and design/06-
// scene-ir.md for the type catalog this package emits.
package encode

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/table"
)

// Scale resolves a single data value into a pixel coordinate. The
// concrete impls are LinearScale (quantitative), BandScale (nominal
// with bar geometry), OrdinalScale (nominal with explicit positions),
// and TimeScale (a P05 stub: parses ISO-8601 → epoch ms → linear).
type Scale interface {
	// Apply resolves a data value to its pixel coordinate. Returns
	// PRISM_ENCODE_001 on type / category mismatches.
	Apply(value any) (float64, error)
	// Domain returns the resolved input domain ([]float64 for linear /
	// time; []string for band / ordinal). Cast on read.
	Domain() []any
	// Range returns the [min,max] pixel range the scale maps into.
	Range() [2]float64
	// Type returns the canonical scene.ScaleType for this scale.
	Type() scene.ScaleType
}

// LinearScale is the canonical quantitative scale: linear
// interpolation from [domainMin, domainMax] to [rangeMin, rangeMax].
type LinearScale struct {
	DomainMin float64
	DomainMax float64
	RangeMin  float64
	RangeMax  float64
}

// Apply implements Scale.
func (s *LinearScale) Apply(value any) (float64, error) {
	v, ok := toFloat(value)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("LinearScale.Apply: value %v (type %T) is not numeric.", value, value),
			map[string]any{"Field": "<linear>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	if s.DomainMax == s.DomainMin {
		return (s.RangeMin + s.RangeMax) / 2, nil
	}
	t := (v - s.DomainMin) / (s.DomainMax - s.DomainMin)
	return s.RangeMin + t*(s.RangeMax-s.RangeMin), nil
}

// Domain implements Scale.
func (s *LinearScale) Domain() []any { return []any{s.DomainMin, s.DomainMax} }

// Range implements Scale.
func (s *LinearScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *LinearScale) Type() scene.ScaleType { return scene.ScaleLinear }

// BandScale is the categorical scale used by bar marks. Each
// category gets a band of equal width; padding leaves an inner gap.
type BandScale struct {
	Categories []string
	RangeMin   float64
	RangeMax   float64
	Padding    float64 // [0,1) inner padding (fraction of step)
}

// step returns the full step width per category (band + gap).
func (s *BandScale) step() float64 {
	if len(s.Categories) == 0 {
		return 0
	}
	return (s.RangeMax - s.RangeMin) / float64(len(s.Categories))
}

// BandWidth returns the pixel width of one band (post-padding).
// Bar marks ask for this to set rect width.
func (s *BandScale) BandWidth() float64 {
	step := s.step()
	return step * (1 - s.Padding)
}

// Apply implements Scale. Returns the left edge of the band for the
// given category.
func (s *BandScale) Apply(value any) (float64, error) {
	cat, ok := value.(string)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("BandScale.Apply: value %v (type %T) is not a string category.", value, value),
			map[string]any{"Field": "<band>", "Source": "<scale>", "Available": "string"},
		)
	}
	for i, c := range s.Categories {
		if c == cat {
			step := s.step()
			pad := step * s.Padding / 2
			return s.RangeMin + float64(i)*step + pad, nil
		}
	}
	return 0, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("BandScale.Apply: category %q not in domain.", cat),
		map[string]any{"Field": "<band>", "Source": "<scale>", "Available": joinCommas(s.Categories)},
	)
}

// BandCenter returns the center x of the band for category cat.
// Used by axis tick placement so ticks sit under the band middle.
func (s *BandScale) BandCenter(cat string) (float64, error) {
	left, err := s.Apply(cat)
	if err != nil {
		return 0, err
	}
	return left + s.BandWidth()/2, nil
}

// Domain implements Scale.
func (s *BandScale) Domain() []any {
	out := make([]any, len(s.Categories))
	for i, c := range s.Categories {
		out[i] = c
	}
	return out
}

// Range implements Scale.
func (s *BandScale) Range() [2]float64 { return [2]float64{s.RangeMin, s.RangeMax} }

// Type implements Scale.
func (s *BandScale) Type() scene.ScaleType { return scene.ScaleBand }

// OrdinalScale maps categories to explicit positions (no band
// arithmetic). Used for discrete numeric axes or fixed-position
// labels.
type OrdinalScale struct {
	Categories []string
	Positions  []float64 // same length as Categories
}

// Apply implements Scale.
func (s *OrdinalScale) Apply(value any) (float64, error) {
	cat, ok := value.(string)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("OrdinalScale.Apply: value %v (type %T) is not a string category.", value, value),
			map[string]any{"Field": "<ordinal>", "Source": "<scale>", "Available": "string"},
		)
	}
	for i, c := range s.Categories {
		if c == cat {
			return s.Positions[i], nil
		}
	}
	return 0, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("OrdinalScale.Apply: category %q not in domain.", cat),
		map[string]any{"Field": "<ordinal>", "Source": "<scale>", "Available": joinCommas(s.Categories)},
	)
}

// Domain implements Scale.
func (s *OrdinalScale) Domain() []any {
	out := make([]any, len(s.Categories))
	for i, c := range s.Categories {
		out[i] = c
	}
	return out
}

// Range implements Scale.
func (s *OrdinalScale) Range() [2]float64 {
	if len(s.Positions) == 0 {
		return [2]float64{0, 0}
	}
	mn, mx := s.Positions[0], s.Positions[0]
	for _, p := range s.Positions {
		if p < mn {
			mn = p
		}
		if p > mx {
			mx = p
		}
	}
	return [2]float64{mn, mx}
}

// Type implements Scale.
func (s *OrdinalScale) Type() scene.ScaleType { return scene.ScaleOrdinal }

// TimeScale is the P05 stub for temporal scales. It parses ISO-8601
// strings (date or datetime) to epoch ms, then falls through to a
// linear scale over the resulting numeric domain. Real temporal
// support (month/day bucketing, nice ticks over calendar intervals,
// format strings) lands in P06.
type TimeScale struct {
	Linear *LinearScale
}

// Apply implements Scale. Accepts time.Time, ISO-8601 strings, and
// numeric epoch ms.
func (s *TimeScale) Apply(value any) (float64, error) {
	v, ok := toEpochMs(value)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("TimeScale.Apply: value %v (type %T) is not a recognised time form (want ISO-8601 string, time.Time, or numeric epoch ms).", value, value),
			map[string]any{"Field": "<time>", "Source": "<scale>", "Available": "iso-8601 | time.Time | float epoch_ms"},
		)
	}
	return s.Linear.Apply(v)
}

// Domain implements Scale.
func (s *TimeScale) Domain() []any { return s.Linear.Domain() }

// Range implements Scale.
func (s *TimeScale) Range() [2]float64 { return s.Linear.Range() }

// Type implements Scale.
func (s *TimeScale) Type() scene.ScaleType { return scene.ScaleTime }

// ResolveScale picks the right Scale impl for a channel + column
// kind, computes its domain from the values, and returns the
// resulting Scale + an optional warning (e.g. time-scale stub). The
// rangeMin / rangeMax span the plot region for the channel's
// orientation; for y-axes the renderer is responsible for the
// "invert-y" flip via passing (rangeMax, rangeMin).
func ResolveScale(channelType string, kind table.Kind, values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	// Empty channelType infers from the column kind: string → band,
	// date → time, anything else → linear.
	if channelType == "" {
		switch kind {
		case table.KindString:
			return resolveBand(values, rangeMin, rangeMax)
		case table.KindDate:
			return resolveTime(values, rangeMin, rangeMax)
		default:
			return resolveLinear(values, rangeMin, rangeMax)
		}
	}
	switch channelType {
	case "quantitative":
		return resolveLinear(values, rangeMin, rangeMax)
	case "nominal", "ordinal":
		return resolveBand(values, rangeMin, rangeMax)
	case "temporal":
		return resolveTime(values, rangeMin, rangeMax)
	}
	return nil, nil, fmt.Errorf("ResolveScale: unknown channel type %q", channelType)
}

func resolveLinear(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	if len(values) == 0 {
		return &LinearScale{DomainMin: 0, DomainMax: 1, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
	}
	var mn, mx float64
	first := true
	for _, v := range values {
		f, ok := toFloat(v)
		if !ok {
			continue
		}
		if first {
			mn = f
			mx = f
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
			"resolveLinear: no numeric values in domain.",
			map[string]any{"Field": "<linear>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	// Pad domain so 0 is included when all values are positive — this
	// matches Vega-Lite's "zero" default for quantitative scales and
	// makes bar charts read sensibly.
	if mn > 0 {
		mn = 0
	}
	if mx < 0 {
		mx = 0
	}
	return &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
}

func resolveBand(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	seen := map[string]bool{}
	cats := []string{}
	for _, v := range values {
		s, ok := v.(string)
		if !ok {
			continue
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		cats = append(cats, s)
	}
	if len(cats) == 0 {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolveBand: no string categories in domain.",
			map[string]any{"Field": "<band>", "Source": "<scale>", "Available": "string"},
		)
	}
	return &BandScale{
		Categories: cats,
		RangeMin:   rangeMin,
		RangeMax:   rangeMax,
		Padding:    0.1,
	}, nil, nil
}

func resolveTime(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	var mn, mx float64
	first := true
	for _, v := range values {
		e, ok := toEpochMs(v)
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
	// Do NOT zero-extend — calendar epochs are far from zero and
	// padding the domain with 0 would compress every chart to one
	// pixel on the right edge.
	lin := &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}
	warn := &scene.Warning{
		Code:    scene.WarnTimeScaleStubbed,
		Message: "temporal channel rendered as linear over epoch ms; real calendar-aware ticks land in P06.",
	}
	return &TimeScale{Linear: lin}, warn, nil
}

// toFloat coerces common Go numeric types to float64.
func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, !math.IsNaN(x) && !math.IsInf(x, 0)
	case float32:
		f := float64(x)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// toEpochMs converts a time-shaped value (time.Time, ISO-8601
// string, numeric epoch ms) to float64 epoch ms.
func toEpochMs(v any) (float64, bool) {
	switch x := v.(type) {
	case time.Time:
		return float64(x.UnixMilli()), true
	case string:
		// Try RFC3339 first, then date-only.
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return float64(t.UnixMilli()), true
		}
		if t, err := time.Parse("2006-01-02", x); err == nil {
			return float64(t.UnixMilli()), true
		}
		return 0, false
	}
	return toFloat(v)
}

// joinCommas is a tiny helper that turns a slice into a
// comma-separated string for error contexts.
func joinCommas(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	cp := append([]string(nil), xs...)
	sort.Strings(cp)
	out := cp[0]
	for _, s := range cp[1:] {
		out += ", " + s
	}
	return out
}
