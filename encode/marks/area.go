package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeArea emits one scene.Mark with AreaGeom per colour series,
// whose Upper is that series' row-by-row points and whose Lower is
// the y=0 baseline edge (one point per Upper x, snapped to the pixel
// where the data value is 0). The baseline is the scale's zero, so
// positive-only domains fill down to the plot bottom and
// zero-crossing domains fill above and below the mid-plot zero line.
//
// Per-series like the line mark, and for the same reason: a single
// polygon over every row of a multi-series table closes across
// unrelated points and shades a region that means nothing. Stacked /
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
	// Baseline = pixel y where the data value = 0 (mirrors bar.go).
	// Positive-only domains snap this to the plot bottom; zero-crossing
	// domains land it mid-plot. Fall back to the plot bottom on apply
	// failure (shouldn't happen for linear scales).
	baseline, err := in.Y.Scale.Apply(float64(0))
	if err != nil {
		baseline = in.Layout.Bottom()
	}
	runs := splitSeries(in, len(xs))
	out := make([]scene.Mark, 0, len(runs))
	for si, run := range runs {
		upper := make([][2]float64, 0, len(run.Rows))
		lower := make([][2]float64, 0, len(run.Rows))
		for _, row := range run.Rows {
			x, err := in.X.Scale.Apply(xs[row])
			if err != nil {
				return nil, err
			}
			y, err := in.Y.Scale.Apply(ys[row])
			if err != nil {
				return nil, err
			}
			upper = append(upper, [2]float64{x, y})
			lower = append(lower, [2]float64{x, baseline})
		}
		if len(upper) == 0 {
			continue
		}
		style := in.Style
		if run.Color != nil {
			style.Fill = run.Color
			if style.Stroke != nil {
				style.Stroke = run.Color
			}
		}
		id := "area-0"
		if run.Category != "" {
			id = fmt.Sprintf("area-%d", si)
		}
		mark := scene.Mark{
			Type:  scene.MarkArea,
			ID:    id,
			Style: style,
			Area: &scene.AreaGeom{
				Upper: upper,
				Lower: lower,
				Curve: scene.CurveLinear,
			},
		}
		if run.Category != "" {
			mark.Series = run.Category
		}
		out = append(out, mark)
	}
	return out, nil
}
