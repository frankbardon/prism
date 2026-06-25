package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// winlossHeightRatio is the fraction of the plot height each win/loss
// bar occupies above (or below) the baseline. Every bar is the same
// height — only its direction (above vs below the y==0 baseline)
// carries meaning.
const winlossHeightRatio = 0.4

// encodeWinloss emits one RectGeom per row, an equal-height up/down bar
// driven solely by the sign of y: y > 0 → bar above the baseline,
// y < 0 → bar below, y == 0 → a flat (zero-height) bar at the baseline.
// Magnitude is intentionally ignored; |y| never affects bar height.
//
// Like sparkline/sparkbar this is a chrome-suppressed spark mark
// (isSparkMark in encode/encode.go strips axes/legend/title and routes
// layout through ComputeSparkline). The baseline is the y==0 pixel,
// mirroring the bar encoder (bar.go), so a sign-crossing domain centres
// the streak in the plot region.
func encodeWinloss(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeWinloss: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorVals = cv
	}

	band, ok := in.X.Scale.(BandScaler)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"winloss mark requires a band scale for x, got a continuous scale instead.",
			map[string]any{"Field": in.X.Field, "Source": "<scale>", "Available": "band"},
		)
	}
	width := band.BandWidth()

	// Baseline = pixel y where data value = 0 (mirrors bar.go). For a
	// sign-crossing domain this lands mid-plot; falls back to the plot
	// bottom only if the scale cannot map 0 (shouldn't happen for
	// linear scales).
	baseline, err := in.Y.Scale.Apply(float64(0))
	if err != nil {
		baseline = in.Layout.Bottom()
	}

	// Every bar is the same height regardless of |y|.
	barHeight := in.Layout.H * winlossHeightRatio

	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
	}

	marks := make([]scene.Mark, 0, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		// Direction by sign of y; magnitude ignored. y > 0 grows up,
		// y < 0 grows down, y == 0 (or non-numeric) is a flat marker on
		// the baseline so the per-row datum alignment stays 1:1.
		top, h := baseline, 0.0
		if v, ok := toFloat64(ys[i]); ok {
			switch {
			case v > 0:
				top = baseline - barHeight
				h = barHeight
			case v < 0:
				top = baseline
				h = barHeight
			}
		}
		style := in.Style
		if len(colorVals) > 0 {
			cat, ok := colorVals[i].(string)
			if ok {
				if c := lookupCategoryColor(cat, in.Color.Categories, in.Color.Palette); c != nil {
					style.Fill = c
				}
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("winloss-%d", i),
			Style: style,
			Rect: &scene.RectGeom{
				X:       x,
				Y:       top,
				W:       width,
				H:       h,
				CornerR: cornerR,
			},
		})
	}
	return marks, nil
}
