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
func encodeBar(in Inputs) ([]scene.Mark, error) {
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
				if c := lookupCategoryColor(cat, in.Color.Categories, in.Color.Palette); c != nil {
					style.Fill = c
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
