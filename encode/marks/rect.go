package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeRect emits one RectGeom per row.
//
// Three shapes fall out of which axes carry a band scale:
//
//   - band on exactly one axis — the bar-like variant. The band axis
//     is the category, the other axis is the measure, and the rect
//     grows from the data-zero baseline. Orientation follows
//     MarkOrientation (orient.go), so a band on y draws horizontally
//     just as a band on x draws vertically.
//   - band on both axes — the heatmap-lite cell: each axis contributes
//     its band slot.
//   - band on neither — the historic 1-px cell centred on the point.
//
// Every one of those slots routes through rectAxisExtent, the single
// band normaliser (E9-S3). That matters because a y band scale runs
// bottom-to-top and so has a *negative* step: reading BandWidth()
// straight into RectGeom.H — which this encoder used to do on both the
// horizontal-bar and heatmap-lite branches — produced an undrawable
// rect with a negative height. E9-S1 routes both through the
// normaliser instead.
//
// E9-S3 makes the ranged case real: when x2 and/or y2 is bound,
// encodeRectSpan draws the x→x2 / y→y2 interval on that axis and
// keeps the band (or the 1-px cell) on the other. Specs without a span
// channel never reach that path.
func encodeRect(in Inputs) ([]scene.Mark, error) {
	if spanBound(in.X2) || spanBound(in.Y2) {
		return encodeRectSpan(in)
	}
	_, xIsBand := bandOf(in.X)
	_, yIsBand := bandOf(in.Y)
	if xIsBand != yIsBand {
		return encodeRectBaseline(in)
	}
	// Band on both axes (cell) or neither (1-px point cell): each axis
	// resolves independently, exactly as the ranged path does with no
	// span bound.
	xExt, err := rectAxisExtent(in, "x", in.X, Channel{}, false)
	if err != nil {
		return nil, err
	}
	yExt, err := rectAxisExtent(in, "y", in.Y, Channel{}, false)
	if err != nil {
		return nil, err
	}
	if len(xExt) != len(yExt) {
		return nil, fmt.Errorf("encodeRect: column length mismatch (x=%d, y=%d)", len(xExt), len(yExt))
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

// encodeRectBaseline draws the bar-like rect variant: one band axis,
// one baseline-anchored measure axis, in whichever orientation the
// band implies (or `mark.orient` declares).
func encodeRectBaseline(in Inputs) ([]scene.Mark, error) {
	orient, err := MarkOrientation(in, "rect")
	if err != nil {
		return nil, err
	}
	category, err := CategorySlots(in, orient)
	if err != nil {
		return nil, err
	}
	measure, err := MeasureSpans(in, orient)
	if err != nil {
		return nil, err
	}
	if len(category) != len(measure) {
		return nil, fmt.Errorf("encodeRect: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(category), orient.MeasureAxis(), len(measure))
	}
	marks := make([]scene.Mark, 0, len(category))
	for i := range category {
		rect := OrientedRect(orient, category[i], measure[i])
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
