package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeArea emits exactly one scene.Mark with AreaGeom whose Upper
// is the row-by-row points and Lower is nil (baseline-0). Stacked /
// streamgraph variants land in P08.
func encodeArea(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeArea: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	if len(xs) == 0 {
		return nil, nil
	}
	upper := make([][2]float64, 0, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		y, err := in.Y.Scale.Apply(ys[i])
		if err != nil {
			return nil, err
		}
		upper = append(upper, [2]float64{x, y})
	}
	mark := scene.Mark{
		Type:  scene.MarkArea,
		ID:    "area-0",
		Style: in.Style,
		Area: &scene.AreaGeom{
			Upper: upper,
			Curve: scene.CurveLinear,
		},
	}
	return []scene.Mark{mark}, nil
}
