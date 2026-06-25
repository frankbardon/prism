package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeArea emits exactly one scene.Mark with AreaGeom whose Upper
// is the row-by-row points and whose Lower is the y=0 baseline edge
// (one point per Upper x, snapped to the pixel where the data value
// is 0). The baseline is the scale's zero, so positive-only domains
// fill down to the plot bottom and zero-crossing domains fill above
// and below the mid-plot zero line. Stacked / streamgraph variants
// land in P08.
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
	upper := make([][2]float64, 0, len(xs))
	lower := make([][2]float64, 0, len(xs))
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
		lower = append(lower, [2]float64{x, baseline})
	}
	mark := scene.Mark{
		Type:  scene.MarkArea,
		ID:    "area-0",
		Style: in.Style,
		Area: &scene.AreaGeom{
			Upper: upper,
			Lower: lower,
			Curve: scene.CurveLinear,
		},
	}
	return []scene.Mark{mark}, nil
}
