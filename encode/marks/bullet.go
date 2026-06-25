package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// Bullet layout fractions (of the cross-axis span). The measure bar is
// the headline; the comparative bar is intentionally thinner so it reads
// as an overlay; the target tick over-reaches both so it stays visible.
const (
	bulletMeasureFrac = 0.34
	bulletCompFrac    = 0.16
	bulletTargetFrac  = 0.55
	bulletTargetWidth = 2.0
)

// encodeBullet emits a bullet KPI mark as an ordered set of primitive
// scene marks, layered back-to-front:
//
//  1. qualitative band rects   — graded background ranges from mark.bands
//  2. the measure bar          — the encoded data value (thick)
//  3. a thinner comparative bar — overlaid on the measure (mark.comparative)
//  4. a target rule            — the value to beat (mark.target)
//
// Orientation ("horizontal" default | "vertical") rotates the whole
// assembly consistently: horizontal grows the bars left→right along the
// x scale with the target as a vertical tick; vertical grows them
// bottom→top along the y scale with the target as a horizontal tick.
//
// Unlike the spark family, bullet keeps its scale axis — it is
// intentionally absent from encode.sparkMarks, so the standard layout
// (with axis + legend) applies.
func encodeBullet(in Inputs) ([]scene.Mark, error) {
	vertical := in.Mark != nil && in.Mark.Orientation == "vertical"

	measureField := in.X.Field
	measureScale := in.X.Scale
	if vertical {
		measureField = in.Y.Field
		measureScale = in.Y.Scale
	}
	if measureField == "" || measureScale == nil {
		axis := "x"
		if vertical {
			axis = "y"
		}
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("bullet mark requires a %s channel binding for the measure value.", axis),
			map[string]any{"Field": measureField, "Source": "<encoding>", "Available": axis},
		)
	}

	var (
		bands       []float64
		comparative any
		target      any
	)
	if in.Mark != nil {
		bands = in.Mark.Bands
		comparative = in.Mark.Comparative
		target = in.Mark.Target
	}

	measureVal, err := bulletMeasureValue(in, measureField)
	if err != nil {
		return nil, err
	}

	// Pixel where the measure axis reads zero — the common origin of
	// every band and bar. Fall back to the plot edge on apply failure
	// (mirrors encodeBar's baseline handling).
	base, err := measureScale.Apply(float64(0))
	if err != nil {
		if vertical {
			base = in.Layout.Bottom()
		} else {
			base = in.Layout.X
		}
	}

	out := make([]scene.Mark, 0, len(bands)+3)

	// 1. Band rects (graded background ranges), low → high. Each range
	//    runs from the previous bound (0 for the first) to its bound.
	prev := float64(0)
	for i, bound := range bands {
		hi, err := measureScale.Apply(bound)
		if err != nil {
			return nil, err
		}
		lo, err := measureScale.Apply(prev)
		if err != nil {
			return nil, err
		}
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("bullet-band-%d", i),
			Style: scene.Style{Fill: bulletBandShade(i, len(bands))},
			Rect:  bulletRect(in.Layout, lo, hi, 1.0, vertical),
		})
		prev = bound
	}

	// 2. Measure bar (the data value).
	mPix, err := measureScale.Apply(measureVal)
	if err != nil {
		return nil, err
	}
	out = append(out, scene.Mark{
		Type:  scene.MarkRect,
		ID:    "bullet-measure",
		Style: in.Style,
		Rect:  bulletRect(in.Layout, base, mPix, bulletMeasureFrac, vertical),
	})

	// 3. Comparative bar (thinner overlay), when supplied.
	if cmp, ok, err := bulletRefValue(in, comparative); err != nil {
		return nil, err
	} else if ok {
		cPix, err := measureScale.Apply(cmp)
		if err != nil {
			return nil, err
		}
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    "bullet-comparative",
			Style: scene.Style{Fill: bulletComparativeShade()},
			Rect:  bulletRect(in.Layout, base, cPix, bulletCompFrac, vertical),
		})
	}

	// 4. Target rule (the value to beat), when supplied.
	if tgt, ok, err := bulletRefValue(in, target); err != nil {
		return nil, err
	} else if ok {
		tPix, err := measureScale.Apply(tgt)
		if err != nil {
			return nil, err
		}
		out = append(out, scene.Mark{
			Type: scene.MarkRule,
			ID:   "bullet-target",
			Style: scene.Style{
				Stroke:      bulletTargetStroke(),
				StrokeWidth: bulletTargetWidth,
			},
			Rule: bulletTargetRule(in.Layout, tPix, vertical),
		})
	}

	return out, nil
}

// bulletRect builds a rect spanning from pixel a to pixel b along the
// measure axis and frac of the plot's cross-axis span, centered. For
// horizontal bullets the measure axis is x; for vertical it is y.
func bulletRect(plot scene.Rect, a, b, frac float64, vertical bool) *scene.RectGeom {
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	if vertical {
		thick := plot.W * frac
		x := plot.CenterX() - thick/2
		return &scene.RectGeom{X: x, Y: lo, W: thick, H: hi - lo}
	}
	thick := plot.H * frac
	y := plot.CenterY() - thick/2
	return &scene.RectGeom{X: lo, Y: y, W: hi - lo, H: thick}
}

// bulletTargetRule builds the perpendicular target tick at pixel p along
// the measure axis, reaching bulletTargetFrac of the cross-axis span.
func bulletTargetRule(plot scene.Rect, p float64, vertical bool) *scene.RuleGeom {
	if vertical {
		half := plot.W * bulletTargetFrac / 2
		cx := plot.CenterX()
		return &scene.RuleGeom{X1: cx - half, Y1: p, X2: cx + half, Y2: p}
	}
	half := plot.H * bulletTargetFrac / 2
	cy := plot.CenterY()
	return &scene.RuleGeom{X1: p, Y1: cy - half, X2: p, Y2: cy + half}
}

// bulletMeasureValue reads the headline measure from row 0 of the
// measure field (a bullet renders a single KPI readout).
func bulletMeasureValue(in Inputs, field string) (float64, error) {
	vals, err := readField(in.Table, field)
	if err != nil {
		return 0, err
	}
	if len(vals) == 0 {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("bullet measure field %q has no rows.", field),
			map[string]any{"Field": field, "Source": "<table>", "Available": joinFieldNames(in.Table)},
		)
	}
	v, ok := toFloat64(vals[0])
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("bullet measure value is not numeric (got %T).", vals[0]),
			map[string]any{"Field": field, "Source": "<table>", "Available": "numeric"},
		)
	}
	return v, nil
}

// bulletRefValue resolves a target / comparative reference: a literal
// number is used as-is; a string is treated as a data-field name read
// from row 0. Returns (value, present, error); present is false when raw
// is nil.
func bulletRefValue(in Inputs, raw any) (float64, bool, error) {
	switch t := raw.(type) {
	case nil:
		return 0, false, nil
	case string:
		if t == "" {
			return 0, false, nil
		}
		v, err := bulletMeasureValue(in, t)
		if err != nil {
			return 0, false, err
		}
		return v, true, nil
	default:
		v, ok := toFloat64(t)
		if !ok {
			return 0, false, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("bullet target / comparative must be a number or field name (got %T).", raw),
				map[string]any{"Field": "<bullet>", "Source": "<mark>", "Available": "number|field"},
			)
		}
		return v, true, nil
	}
}

// bulletBandShade returns a neutral gray for band index i of n, ramping
// from dark (worst, low range) to light (best, high range) so the
// colored measure bar stands out against them.
func bulletBandShade(i, n int) *scene.Color {
	const dark, light = 0x6e, 0xdc
	if n <= 1 {
		g := uint8((dark + light) / 2)
		return &scene.Color{R: g, G: g, B: g, A: 255}
	}
	t := float64(i) / float64(n-1)
	g := uint8(float64(dark) + t*float64(light-dark))
	return &scene.Color{R: g, G: g, B: g, A: 255}
}

// bulletComparativeShade is the neutral fill for the thin comparative
// bar — darker than the bands so it reads as a distinct overlay.
func bulletComparativeShade() *scene.Color {
	return &scene.Color{R: 0x44, G: 0x44, B: 0x44, A: 255}
}

// bulletTargetStroke is the strong dark stroke for the target tick.
func bulletTargetStroke() *scene.Color {
	return &scene.Color{R: 0x22, G: 0x22, B: 0x22, A: 255}
}
