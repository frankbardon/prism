package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeBar emits one scene.Mark with RectGeom per table row.
//
// A bar has a category axis (a band scale, which sizes the bar across
// its thickness) and a measure axis (a continuous scale, along which
// the bar grows from the data-zero baseline). Which physical axis is
// which is the mark's orientation: `vertical` is band-on-x, measure-on-y
// — the default — and `horizontal` is the mirror, band-on-y with bars
// growing rightward from a baseline on the left. MarkOrientation
// (orient.go) resolves it from `mark.orient` when set and infers it
// from which axis carries the band otherwise.
//
// When an offset channel is bound on the category axis (E1-S4) the
// bar takes a sub-band of its category's slot rather than the whole
// slot, so rows sharing a category render side by side — the grouped
// (dodged) bar. The bar encoder needs no offset-specific code for
// that: CategorySlots hands back the sub-band extent in place of the
// full one, in whichever orientation the mark resolved, and the rest
// of the geometry is unchanged.
//
// When either span channel is bound (E9-S3) the bar is ranged
// instead: encodeBarSpan replaces the baseline anchor on that axis
// with the x→x2 / y→y2 interval. Unranged specs never reach that
// path, so their geometry is unchanged.
func encodeBar(in Inputs) ([]scene.Mark, error) {
	if spanBound(in.X2) || spanBound(in.Y2) {
		return encodeBarSpan(in)
	}
	orient, err := MarkOrientation(in, "bar")
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
		return nil, fmt.Errorf("encodeBar: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(category), orient.MeasureAxis(), len(measure))
	}
	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorVals = cv
	}

	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
	}

	marks := make([]scene.Mark, 0, len(category))
	for i := range category {
		if skipRow(in, i) {
			continue
		}
		style := in.Style
		if i < len(colorVals) {
			cat, ok := colorVals[i].(string)
			if ok {
				if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
					style.Fill = c
					style.FillVar = v
				}
			}
		}
		rect := OrientedRect(orient, category[i], measure[i])
		rect.CornerR = cornerR
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("bar-%d", i),
			Style: style,
			Rect:  &rect,
		})
	}
	return marks, nil
}

// encodeBarSpan emits one ranged rect per row: each axis resolves
// either to its x→x2 / y→y2 pixel interval or, when that axis carries
// no span channel, to the band slot the base channel lands in. A bar
// therefore still needs a categorical axis on whichever side is not
// ranged — the classic Gantt shape is `y` band plus an `x`/`x2`
// interval.
//
// Color grouping and corner radius behave exactly as in the
// baseline-anchored path.
func encodeBarSpan(in Inputs) ([]scene.Mark, error) {
	xExt, err := rectAxisExtent(in, "x", in.X, in.X2, true)
	if err != nil {
		return nil, err
	}
	yExt, err := rectAxisExtent(in, "y", in.Y, in.Y2, true)
	if err != nil {
		return nil, err
	}
	if len(xExt) != len(yExt) {
		return nil, fmt.Errorf("encodeBarSpan: column length mismatch (x=%d, y=%d)", len(xExt), len(yExt))
	}
	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorVals = cv
	}
	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
	}

	marks := make([]scene.Mark, 0, len(xExt))
	for i := range xExt {
		style := in.Style
		if i < len(colorVals) {
			if cat, ok := colorVals[i].(string); ok {
				if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
					style.Fill = c
					style.FillVar = v
				}
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("bar-%d", i),
			Style: style,
			Rect: &scene.RectGeom{
				X:       xExt[i][0],
				Y:       yExt[i][0],
				W:       xExt[i][1],
				H:       yExt[i][1],
				CornerR: cornerR,
			},
		})
	}
	return marks, nil
}
