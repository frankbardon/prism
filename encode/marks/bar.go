package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeBar emits one scene.Mark with RectGeom per table row.
// Expects x = band scale (categorical), y = linear scale. The bar
// grows from the plot's baseline (y where data value = 0) up or
// down to the encoded y pixel.
//
// When either span channel is bound (E9-S3) the bar is ranged
// instead: encodeBarSpan replaces the baseline anchor on that axis
// with the x→x2 / y→y2 interval. Unranged specs never reach that
// path, so their geometry is unchanged.
func encodeBar(in Inputs) ([]scene.Mark, error) {
	if spanBound(in.X2) || spanBound(in.Y2) {
		return encodeBarSpan(in)
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
		return nil, fmt.Errorf("encodeBar: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorVals = cv
	}

	band, ok := in.X.Scale.(BandScaler)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"bar mark requires a band scale for x, got a continuous scale instead.",
			map[string]any{"Field": in.X.Field, "Source": "<scale>", "Available": "band"},
		)
	}
	width := band.BandWidth()

	// Baseline = pixel y where data value = 0 (or plot bottom for
	// positive-only domains where 0 sits on the lower edge).
	baseline, err := in.Y.Scale.Apply(float64(0))
	if err != nil {
		// Fall back to plot bottom on apply failure (shouldn't happen
		// for linear scales).
		baseline = in.Layout.Bottom()
	}

	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
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
		// Rect lives between y (top) and baseline (bottom). For
		// positive values y < baseline so H = baseline - y; for
		// negative values y > baseline so we flip.
		top, h := y, baseline-y
		if h < 0 {
			top = baseline
			h = -h
		}
		style := in.Style
		if len(colorVals) > 0 {
			cat, ok := colorVals[i].(string)
			if ok {
				if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
					style.Fill = c
					style.FillVar = v
				}
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("bar-%d", i),
			Style: style,
			Rect: &scene.RectGeom{
				X:       x,
				Y:       top,
				W:       width,
				H:       h,
				CornerR: cornerR,
			},
		})
	}
	return marks, nil
}

// encodeBarSpan emits one ranged rect per row: each axis resolves
// either to its x→x2 / y→y2 pixel interval or, when that axis carries
// no span channel, to the band slot the base channel lands in. A bar
// therefore still needs a categorical axis on whichever side is not
// ranged — the classic Gantt shape is `y` band plus an `x`/`x2`
// interval.
//
// Color grouping and corner radius behave exactly as in the
// baseline-anchored path.
func encodeBarSpan(in Inputs) ([]scene.Mark, error) {
	xExt, err := rectAxisExtent(in, "x", in.X, in.X2, true)
	if err != nil {
		return nil, err
	}
	yExt, err := rectAxisExtent(in, "y", in.Y, in.Y2, true)
	if err != nil {
		return nil, err
	}
	if len(xExt) != len(yExt) {
		return nil, fmt.Errorf("encodeBarSpan: column length mismatch (x=%d, y=%d)", len(xExt), len(yExt))
	}
	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorVals = cv
	}
	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
	}

	marks := make([]scene.Mark, 0, len(xExt))
	for i := range xExt {
		style := in.Style
		if i < len(colorVals) {
			if cat, ok := colorVals[i].(string); ok {
				if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
					style.Fill = c
					style.FillVar = v
				}
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("bar-%d", i),
			Style: style,
			Rect: &scene.RectGeom{
				X:       xExt[i][0],
				Y:       yExt[i][0],
				W:       xExt[i][1],
				H:       yExt[i][1],
				CornerR: cornerR,
			},
		})
	}
	return marks, nil
}
