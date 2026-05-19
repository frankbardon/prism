package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeRect emits one RectGeom per row spanning x..x2 × y..y2 when
// both upper bounds are bound. When only x (or only y) bounds are
// present, falls back to the bar-style geometry: band scale on x,
// baseline-anchored bar height on y.
//
// For the heatmap-lite case (categorical x + categorical y) the
// width/height come from the BandSizer (or PointScale step) on each
// axis.
func encodeRect(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeRect: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}

	xBand, xIsBand := in.X.Scale.(BandScaler)
	yBand, yIsBand := in.Y.Scale.(BandScaler)

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
		var rect scene.RectGeom
		switch {
		case xIsBand && yIsBand:
			// Heatmap-lite cell.
			rect = scene.RectGeom{
				X: x, Y: y,
				W: xBand.BandWidth(),
				H: yBand.BandWidth(),
			}
		case xIsBand:
			// Bar-like (band x, quantitative y).
			baseline, err := in.Y.Scale.Apply(float64(0))
			if err != nil {
				baseline = in.Layout.Bottom()
			}
			top, h := y, baseline-y
			if h < 0 {
				top = baseline
				h = -h
			}
			rect = scene.RectGeom{X: x, Y: top, W: xBand.BandWidth(), H: h}
		case yIsBand:
			// Horizontal-bar variant.
			baseline, err := in.X.Scale.Apply(float64(0))
			if err != nil {
				baseline = in.Layout.X
			}
			left, w := baseline, x-baseline
			if w < 0 {
				left = x
				w = -w
			}
			rect = scene.RectGeom{X: left, Y: y, W: w, H: yBand.BandWidth()}
		default:
			// Fully quantitative; render a 1-px cell centered on the point.
			rect = scene.RectGeom{X: x - 0.5, Y: y - 0.5, W: 1, H: 1}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("rect-%d", i),
			Style: in.Style,
			Rect:  &rect,
		})
	}
	return marks, nil
}
