package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeTick emits one tick LineGeom per table row: a short segment
// drawn *along* the measure axis at the row's value, positioned at the
// centre of its category slot. Used for ranking / strip-plot style
// charts. tickSize defaults to 10px; mark.size overrides it.
//
// Orientation comes from MarkOrientation (orient.go) in its
// band-optional form: a band on x reads vertical, a band on y reads
// horizontal, an explicit `mark.orient` overrides either, and two
// continuous axes fall back to horizontal — the strip-plot shape tick
// has always drawn when neither axis is discrete.
func encodeTick(in Inputs) ([]scene.Mark, error) {
	orient, err := MarkOrientationOr(in, "tick", OrientHorizontal)
	if err != nil {
		return nil, err
	}
	centers, err := CategoryCenters(in, orient)
	if err != nil {
		return nil, err
	}
	measure := MeasureChannel(in, orient)
	vals, err := readField(in.Table, measure.Field)
	if err != nil {
		return nil, err
	}
	if len(centers) != len(vals) {
		return nil, fmt.Errorf("encodeTick: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(centers), orient.MeasureAxis(), len(vals))
	}

	tickSize := 10.0
	if in.Mark != nil && in.Mark.Size != nil {
		tickSize = *in.Mark.Size
	}

	marks := make([]scene.Mark, 0, len(centers))
	for i := range centers {
		m, err := measure.Scale.Apply(vals[i])
		if err != nil {
			return nil, err
		}
		x1, y1 := OrientedPoint(orient, centers[i], m-tickSize/2)
		x2, y2 := OrientedPoint(orient, centers[i], m+tickSize/2)
		marks = append(marks, scene.Mark{
			Type:  scene.MarkLine,
			ID:    fmt.Sprintf("tick-%d", i),
			Style: in.Style,
			Line: &scene.LineGeom{
				Points: [][2]float64{{x1, y1}, {x2, y2}},
				Curve:  scene.CurveLinear,
			},
		})
	}
	return marks, nil
}
