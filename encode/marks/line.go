package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeLine emits one scene.Mark with LineGeom per colour series.
//
// One mark per SERIES, not one per chart: see series.go for why a
// single polyline over every row was drawing connections between
// unrelated measurements.
//
// Row order within a series is the upstream order — sorting by x is
// the caller's decision (a Sort transform), not the encoder's.
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

	runs := splitSeries(in, len(xs))
	out := make([]scene.Mark, 0, len(runs))
	for si, run := range runs {
		pts := make([][2]float64, 0, len(run.Rows))
		for _, row := range run.Rows {
			x, err := in.X.Scale.Apply(xs[row])
			if err != nil {
				return nil, err
			}
			y, err := in.Y.Scale.Apply(ys[row])
			if err != nil {
				return nil, err
			}
			pts = append(pts, [2]float64{x, y})
		}
		if len(pts) == 0 {
			continue
		}
		style := in.Style
		if run.Color != nil {
			style.Stroke = run.Color
		}
		id := "line-0"
		if run.Category != "" {
			id = fmt.Sprintf("line-%d", si)
		}
		mark := scene.Mark{
			Type:  scene.MarkLine,
			ID:    id,
			Style: style,
			Line: &scene.LineGeom{
				Points: pts,
				Curve:  scene.CurveLinear,
			},
		}
		if run.Category != "" {
			mark.Series = run.Category
		}
		out = append(out, mark)
	}
	return out, nil
}
