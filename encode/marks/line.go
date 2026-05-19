package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeLine emits exactly one scene.Mark with LineGeom carrying
// every row's (x, y) point. Order = upstream row order; sorting by x
// is the encoder's responsibility (T05.10's tip-id resolution may
// inject an explicit Sort transform, but P05 trusts the spec).
func encodeLine(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeLine: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	if len(xs) == 0 {
		return nil, nil
	}

	pts := make([][2]float64, 0, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		y, err := in.Y.Scale.Apply(ys[i])
		if err != nil {
			return nil, err
		}
		pts = append(pts, [2]float64{x, y})
	}

	mark := scene.Mark{
		Type:  scene.MarkLine,
		ID:    "line-0",
		Style: in.Style,
		Line: &scene.LineGeom{
			Points: pts,
			Curve:  scene.CurveLinear,
		},
	}
	return []scene.Mark{mark}, nil
}
