package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// winlossHeightRatio is the fraction of the plot's measure-axis extent
// each win/loss bar occupies on either side of the baseline. Every bar
// is the same length — only its direction (which side of the zero
// baseline it falls on) carries meaning.
const winlossHeightRatio = 0.4

// encodeWinloss emits one RectGeom per row, an equal-length win/loss
// bar driven solely by the sign of the measured value: positive → bar
// on the positive side of the baseline, negative → the other side,
// zero → a flat (zero-length) bar on the baseline itself. Magnitude is
// intentionally ignored; |value| never affects bar length.
//
// Like sparkline/sparkbar this is a chrome-suppressed spark mark
// (isSparkMark in encode/encode.go strips axes/legend/title and routes
// layout through ComputeSparkline). The baseline is the measure axis's
// zero pixel, mirroring the bar encoder (bar.go), so a sign-crossing
// domain centres the streak in the plot region.
//
// Orientation (E9-S2) comes from MarkOrientation (orient.go), so a
// band on y — or an explicit `"orient": "horizontal"` — draws the
// streak as rows growing left/right from a vertical baseline instead.
func encodeWinloss(in Inputs) ([]scene.Mark, error) {
	orient, err := MarkOrientation(in, "winloss")
	if err != nil {
		return nil, err
	}
	slots, err := CategorySlots(in, orient)
	if err != nil {
		return nil, err
	}
	measure := MeasureChannel(in, orient)
	vals, err := readField(in.Table, measure.Field)
	if err != nil {
		return nil, err
	}
	if len(slots) != len(vals) {
		return nil, fmt.Errorf("encodeWinloss: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(slots), orient.MeasureAxis(), len(vals))
	}
	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorVals = cv
	}

	// Baseline = the measure axis's data-zero pixel (mirrors bar.go).
	// For a sign-crossing domain this lands mid-plot.
	baseline := BaselinePixel(in, orient)

	// Every bar is the same length regardless of |value| — a fixed
	// fraction of the plot's measure-axis extent. `dir` is the pixel
	// direction a positive value points in, so a win grows up when
	// vertical and rightward when horizontal.
	barLen := PlotMeasureExtent(orient, in.Layout)[1] * winlossHeightRatio
	dir := MeasureDirection(orient)

	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
	}

	marks := make([]scene.Mark, 0, len(slots))
	for i := range slots {
		// Direction by sign; magnitude ignored. A positive value grows
		// along dir, a negative one against it, and zero (or a
		// non-numeric cell) is a flat marker on the baseline so the
		// per-row datum alignment stays 1:1.
		span := [2]float64{baseline, 0}
		if v, ok := toFloat64(vals[i]); ok {
			switch {
			case v > 0:
				span = spanFromBaseline(baseline+dir*barLen, baseline)
			case v < 0:
				span = spanFromBaseline(baseline-dir*barLen, baseline)
			}
		}
		style := in.Style
		if len(colorVals) > 0 {
			cat, ok := colorVals[i].(string)
			if ok {
				if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
					style.Fill = c
					style.FillVar = v
				}
			}
		}
		rect := OrientedRect(orient, slots[i], span)
		rect.CornerR = cornerR
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("winloss-%d", i),
			Style: style,
			Rect:  &rect,
		})
	}
	return marks, nil
}
