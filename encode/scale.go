package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
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

// ResolveScale picks the right Scale impl for a channel + column
// kind, computes its domain from the values, and returns the
// resulting Scale + an optional warning. The rangeMin / rangeMax span
// the plot region for the channel's orientation; for y-axes the
// renderer is responsible for the "invert-y" flip via passing
// (rangeMax, rangeMin).
func ResolveScale(channelType string, kind table.Kind, values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
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

// ResolveScaleTyped accepts an explicit scene.ScaleType (from the
// spec's `scale.type` field) and resolves the appropriate impl. Used
// by the encoder once the spec carries a scale.type override.
func ResolveScaleTyped(scaleType scene.ScaleType, values []any, rangeMin, rangeMax float64, opts ScaleOpts) (Scale, *scene.Warning, error) {
	switch scaleType {
	case scene.ScaleLinear:
		return resolveLinear(values, rangeMin, rangeMax)
	case scene.ScaleLog:
		return resolveLog(values, rangeMin, rangeMax, opts.Base)
	case scene.ScalePow:
		return resolvePow(values, rangeMin, rangeMax, opts.Exp)
	case scene.ScaleSqrt:
		return resolvePow(values, rangeMin, rangeMax, 0.5)
	case scene.ScaleTime:
		return resolveTime(values, rangeMin, rangeMax)
	case scene.ScaleBand:
		return resolveBand(values, rangeMin, rangeMax)
	case scene.ScalePoint:
		return resolvePoint(values, rangeMin, rangeMax)
	case scene.ScaleOrdinal:
		return resolveOrdinal(values, rangeMin, rangeMax)
	}
	return nil, nil, fmt.Errorf("ResolveScaleTyped: unknown scale type %q", scaleType)
}

// ScaleOpts carries per-scale knobs (log base, pow exponent, etc.).
type ScaleOpts struct {
	Base float64 // log base (default 10 if zero)
	Exp  float64 // pow exponent (default 1 if zero)
}

func resolveLinear(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	if len(values) == 0 {
		return &LinearScale{DomainMin: 0, DomainMax: 1, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
	}
	var mn, mx float64
	first := true
	for _, v := range values {
		f, ok := scale.ToFloat(v)
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
	// Pad domain so 0 is included when all values are positive.
	if mn > 0 {
		mn = 0
	}
	if mx < 0 {
		mx = 0
	}
	return &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
}

func resolveBand(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	cats := uniqueStrings(values)
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

func resolvePoint(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	cats := uniqueStrings(values)
	if len(cats) == 0 {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolvePoint: no string categories in domain.",
			map[string]any{"Field": "<point>", "Source": "<scale>", "Available": "string"},
		)
	}
	return &PointScale{
		Categories: cats,
		RangeMin:   rangeMin,
		RangeMax:   rangeMax,
		Padding:    0.5,
	}, nil, nil
}

func resolveOrdinal(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
	cats := uniqueStrings(values)
	if len(cats) == 0 {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolveOrdinal: no string categories in domain.",
			map[string]any{"Field": "<ordinal>", "Source": "<scale>", "Available": "string"},
		)
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

func resolveTime(values []any, rangeMin, rangeMax float64) (Scale, *scene.Warning, error) {
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
	lin := &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}
	// T06.04: calendar-aware ticks live in encode/ticks_time.go;
	// drop the stub warning that P05 emitted.
	return &TimeScale{Linear: lin}, nil, nil
}

func resolveLog(values []any, rangeMin, rangeMax, base float64) (Scale, *scene.Warning, error) {
	if base == 0 {
		base = 10
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
	return &LogScale{
		Base:      base,
		DomainMin: mn,
		DomainMax: mx,
		RangeMin:  rangeMin,
		RangeMax:  rangeMax,
	}, nil, nil
}

func resolvePow(values []any, rangeMin, rangeMax, exp float64) (Scale, *scene.Warning, error) {
	if exp == 0 {
		exp = 1
	}
	var mn, mx float64
	first := true
	for _, v := range values {
		f, ok := scale.ToFloat(v)
		if !ok {
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
	if first {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolvePow: no numeric values in domain.",
			map[string]any{"Field": "<pow>", "Source": "<scale>", "Available": "numeric"},
		)
	}
	if mn > 0 {
		mn = 0
	}
	if mx < 0 {
		mx = 0
	}
	if exp == 0.5 {
		return &SqrtScale{Inner: PowScale{Exp: 0.5, DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}}, nil, nil
	}
	return &PowScale{Exp: exp, DomainMin: mn, DomainMax: mx, RangeMin: rangeMin, RangeMax: rangeMax}, nil, nil
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
