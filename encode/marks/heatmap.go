package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeHeatmap emits one RectGeom per (x, y) cell. When both axes
// are band scales (the categorical fixture case), each row maps to
// one cell sized by the band widths. The color channel must be a
// quantitative field; the encoder builds a sequential color via
// SequentialColor over the field's [min, max] range.
//
// 2D quantitative binning (continuous x + y) is forward-looking; v1
// requires both axes to be discrete (band/ordinal). Validator
// PRISM_SPEC_013 catches missing x or y.
func encodeHeatmap(in Inputs) ([]scene.Mark, error) {
	if in.X.Field == "" || in.Y.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"heatmap mark requires both x and y channel bindings.",
			map[string]any{"Field": "<xy>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}
	xBand, xIsBand := in.X.Scale.(BandScaler)
	yBand, yIsBand := in.Y.Scale.(BandScaler)
	if !xIsBand || !yIsBand {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"heatmap mark requires band/ordinal scales on both x and y (2D quantitative binning lands in P11).",
			map[string]any{"Field": "<xy>", "Source": "<scale>", "Available": "band"},
		)
	}

	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeHeatmap: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}

	// Color channel: when bound, read the numeric values + compute
	// sequential color per row. When not bound, default to count = 1
	// per cell (encoder did not aggregate; v1 expects pre-aggregated
	// heatmap data per the fixture shape).
	var colorValues []float64
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorValues = make([]float64, len(cv))
		for i, v := range cv {
			f, ok := toFloat64(v)
			if !ok {
				return nil, prismerrors.New(
					"PRISM_ENCODE_001",
					fmt.Sprintf("heatmap color value at row %d is not numeric (got %T).", i, v),
					map[string]any{"Field": in.Color.Field, "Source": "<color>", "Available": "numeric"},
				)
			}
			colorValues[i] = f
		}
	}

	// Compute color range.
	mn, mx := 0.0, 0.0
	if len(colorValues) > 0 {
		mn, mx = colorValues[0], colorValues[0]
		for _, v := range colorValues[1:] {
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
	}

	marks := make([]scene.Mark, 0, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		y, err := in.Y.Scale.Apply(ys[i])
		if err != nil {
			return nil, err
		}
		style := in.Style
		if len(colorValues) > 0 {
			c := SequentialColor(colorValues[i], mn, mx)
			if c != nil {
				style.Fill = c
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("heatmap-%d", i),
			Style: style,
			Rect: &scene.RectGeom{
				X: x,
				Y: y,
				W: xBand.BandWidth(),
				H: yBand.BandWidth(),
			},
		})
	}
	return marks, nil
}

// SequentialColor returns a color along a light-blue → dark-blue
// gradient for v in [min, max]. Degenerate range (min == max) returns
// the mid-tone anchor. P10 keeps the gradient hardcoded; theme-level
// sequential palettes land in P12 alongside richer color tooling.
func SequentialColor(v, mn, mx float64) *scene.Color {
	// Anchors: #dbeafe (blue-100) → #1d4ed8 (blue-700).
	r0, g0, b0 := uint8(0xdb), uint8(0xea), uint8(0xfe)
	r1, g1, b1 := uint8(0x1d), uint8(0x4e), uint8(0xd8)
	if mx == mn {
		return &scene.Color{R: (r0 + r1) / 2, G: (g0 + g1) / 2, B: (b0 + b1) / 2, A: 0xff}
	}
	t := (v - mn) / (mx - mn)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	lerp := func(a, b uint8) uint8 {
		return uint8(float64(a) + t*(float64(b)-float64(a)))
	}
	return &scene.Color{R: lerp(r0, r1), G: lerp(g0, g1), B: lerp(b0, b1), A: 0xff}
}
