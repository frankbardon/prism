package marks

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// encodePoint emits one scene.Mark per row with a PointGeom. R
// defaults to 4 px (Vega-Lite default). Shape defaults to circle.
// When a color channel is bound, each point's Style.Fill is the
// palette entry for the row's category.
func encodePoint(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodePoint: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}

	radius := 4.0
	if in.Mark != nil && in.Mark.Size != nil {
		// Spec size is the symbol's area (Vega-Lite convention); r = sqrt(size/pi).
		radius = math.Sqrt(*in.Mark.Size / math.Pi)
	}

	var colorValues []any
	if in.Color != nil {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorValues = cv
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
		if in.Color != nil && i < len(colorValues) {
			cat, _ := colorValues[i].(string)
			c := lookupCategoryColor(cat, in.Color.Categories, in.Color.Palette)
			if c != nil {
				style.Fill = c
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkPoint,
			ID:    fmt.Sprintf("point-%d", i),
			Style: style,
			Point: &scene.PointGeom{
				Cx:    x,
				Cy:    y,
				R:     radius,
				Shape: scene.ShapeCircle,
			},
		})
	}
	return marks, nil
}

// lookupCategoryColor mirrors encode.CategoryToColor without the
// reverse import (encode imports marks, not the other way around).
func lookupCategoryColor(category string, categories []string, palette []*scene.Color) *scene.Color {
	if len(palette) == 0 {
		return nil
	}
	for i, c := range categories {
		if c == category {
			return palette[i%len(palette)]
		}
	}
	return palette[0]
}
