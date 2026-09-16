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
//
// E9-S3 makes the ranged case real: when x2 and/or y2 is bound,
// encodeRectSpan draws the x→x2 / y→y2 interval on that axis and
// keeps the band (or the historic 1-px cell) on the other. Specs
// without a span channel never reach that path.
func encodeRect(in Inputs) ([]scene.Mark, error) {
	if spanBound(in.X2) || spanBound(in.Y2) {
		return encodeRectSpan(in)
	}
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

// encodeRectSpan emits one ranged rect per row. Each axis resolves
// independently: a bound span channel gives the interval, a band
// scale gives the cell slot, and a plain continuous scale falls back
// to the 1-px cell encodeRect has always drawn — so `x`/`x2` against
// a quantitative y still renders rather than failing.
func encodeRectSpan(in Inputs) ([]scene.Mark, error) {
	xExt, err := rectAxisExtent(in, "x", in.X, in.X2, false)
	if err != nil {
		return nil, err
	}
	yExt, err := rectAxisExtent(in, "y", in.Y, in.Y2, false)
	if err != nil {
		return nil, err
	}
	if len(xExt) != len(yExt) {
		return nil, fmt.Errorf("encodeRectSpan: column length mismatch (x=%d, y=%d)", len(xExt), len(yExt))
	}
	marks := make([]scene.Mark, 0, len(xExt))
	for i := range xExt {
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("rect-%d", i),
			Style: in.Style,
			Rect: &scene.RectGeom{
				X: xExt[i][0],
				Y: yExt[i][0],
				W: xExt[i][1],
				H: yExt[i][1],
			},
		})
	}
	return marks, nil
}
