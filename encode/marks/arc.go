package marks

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeArc emits ArcGeom marks. mode is "arc" (raw arc primitive,
// each row → one slice from the theta channel as a relative weight
// share normalised to 2π), "pie" (same — arc semantics already
// share-based), or "donut" (inner radius = OuterR * 0.55 unless
// overridden via mark_def). See D059.
//
// Layout: Cx, Cy = plot center; OuterR = min(plot.W, plot.H) / 2 - 8
// px margin (overrideable via mark_def.outer_radius or
// encoding.radius.value); InnerR = 0 for pie/arc, OuterR * 0.55 for
// donut (overrideable via mark_def.inner_radius or
// mark_def.inner_radius_ratio).
//
// Color: when in.Color is bound, each slice picks its color via
// lookupCategoryColor; when no color channel, slices share in.Style.Fill.
//
// Slice placement: contiguous from -π/2 (12 o'clock) going clockwise
// (positive direction in SVG with inverted y → matches standard pie
// convention). Last slice snaps to startAngle + 2π exactly so
// floating-point accumulation cannot drift the sum off 2π.
func encodeArc(in Inputs, mode string) ([]scene.Mark, error) {
	// Locate the theta channel via the table. The encoder's standard
	// dispatch path strips Theta into in.X.Field for legacy reasons;
	// here we look at whichever field the caller passed via in.X
	// (the encoder routes encoding.theta.field into in.X.Field when
	// markType is arc/pie/donut).
	thetaField := in.X.Field
	if thetaField == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("%s mark requires a theta channel; got no field binding.", mode),
			map[string]any{"Field": "<theta>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}
	values, err := readField(in.Table, thetaField)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}

	// Coerce values to float64 weights; reject negative.
	weights := make([]float64, len(values))
	total := 0.0
	for i, v := range values {
		var f float64
		switch t := v.(type) {
		case float64:
			f = t
		case int64:
			f = float64(t)
		case int:
			f = float64(t)
		case float32:
			f = float64(t)
		default:
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("%s mark theta value at row %d is not numeric (got %T).", mode, i, v),
				map[string]any{"Field": thetaField, "Source": "<theta>", "Available": "numeric"},
			)
		}
		if f < 0 {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("%s mark theta value at row %d is negative (%g); shares require non-negative values.", mode, i, f),
				map[string]any{"Field": thetaField, "Source": "<theta>", "Available": "non-negative"},
			)
		}
		weights[i] = f
		total += f
	}
	if total == 0 {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("%s mark theta values sum to zero; cannot compute share.", mode),
			map[string]any{"Field": thetaField, "Source": "<theta>", "Available": "positive sum"},
		)
	}

	// Geometry: center of plot region.
	cx := in.Layout.X + in.Layout.W/2
	cy := in.Layout.Y + in.Layout.H/2

	// OuterR / InnerR defaults + overrides.
	outerR := math.Min(in.Layout.W, in.Layout.H)/2 - 8
	if outerR < 1 {
		outerR = 1
	}
	innerR := 0.0
	if mode == "donut" {
		innerR = outerR * 0.55
	}
	if in.Mark != nil {
		if in.Mark.OuterRadius != nil {
			outerR = *in.Mark.OuterRadius
		}
		if in.Mark.InnerRadius != nil {
			innerR = *in.Mark.InnerRadius
		} else if in.Mark.InnerRadiusRatio != nil {
			innerR = outerR * (*in.Mark.InnerRadiusRatio)
		}
	}

	// Color channel resolution.
	var colorValues []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorValues = cv
	}

	// Slice placement: start at -π/2 (12 o'clock), wind clockwise.
	startAngle := -math.Pi / 2
	twoPi := 2 * math.Pi
	endAngleTarget := startAngle + twoPi

	out := make([]scene.Mark, 0, len(weights))
	cursor := startAngle
	for i, w := range weights {
		share := w / total
		end := cursor + share*twoPi
		// Snap last slice to exact target to avoid FP drift.
		if i == len(weights)-1 {
			end = endAngleTarget
		}
		style := in.Style
		if in.Color != nil && i < len(colorValues) {
			cat, _ := colorValues[i].(string)
			c := lookupCategoryColor(cat, in.Color.Categories, in.Color.Palette)
			if c != nil {
				style.Fill = c
			}
		}
		out = append(out, scene.Mark{
			Type:  scene.MarkArc,
			ID:    fmt.Sprintf("%s-%d", mode, i),
			Style: style,
			Arc: &scene.ArcGeom{
				Cx:         cx,
				Cy:         cy,
				StartAngle: cursor,
				EndAngle:   end,
				InnerR:     innerR,
				OuterR:     outerR,
			},
		})
		cursor = end
	}
	return out, nil
}
