package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeTick emits one perpendicular tick LineGeom per table row.
// Used for ranking / strip-plot style charts. When the X scale is a
// band/point scale (categorical), ticks are vertical inside each
// band; when X is quantitative and Y is categorical, ticks are
// horizontal. tickSize defaults to 10px; mark.size override applies.
func encodeTick(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeTick: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}

	tickSize := 10.0
	if in.Mark != nil && in.Mark.Size != nil {
		tickSize = *in.Mark.Size
	}

	// Orientation: categorical X → vertical tick; categorical Y →
	// horizontal tick; both categorical → vertical fallback.
	_, xBand := in.X.Scale.(BandScaler)
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
		var points [][2]float64
		if xBand {
			// Vertical tick centered on the band; y is the value.
			cx := x
			if bs, ok := in.X.Scale.(BandScaler); ok {
				cx = x + bs.BandWidth()/2
			}
			points = [][2]float64{{cx, y - tickSize/2}, {cx, y + tickSize/2}}
		} else {
			// Horizontal tick at x; y is the category center.
			points = [][2]float64{{x - tickSize/2, y}, {x + tickSize/2, y}}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkLine,
			ID:    fmt.Sprintf("tick-%d", i),
			Style: in.Style,
			Line: &scene.LineGeom{
				Points: points,
				Curve:  scene.CurveLinear,
			},
		})
	}
	return marks, nil
}
