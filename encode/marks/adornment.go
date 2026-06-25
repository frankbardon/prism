package marks

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// adornmentDotRadius is the pixel radius of a spark emphasis dot. Small
// so it reads as an accent on the compact spark, not a scatter point.
const adornmentDotRadius = 2.5

// adornmentBandOpacity is the default fill opacity for the reference
// band — faint enough to sit behind the series without obscuring it.
const adornmentBandOpacity = 0.12

// Adornments captures the opt-in, default-off spark embellishments
// resolved from a MarkDef (E4). The zero value enables nothing.
type Adornments struct {
	// PointLast draws an emphasis dot on the final series value.
	PointLast bool
	// PointExtent draws highlight dots on the highest and lowest values.
	PointExtent bool
	// ReferenceBand shades a horizontal normal-range band behind the
	// spark. Nil = no band.
	ReferenceBand *spec.ReferenceBand
}

// enabled reports whether any adornment is requested.
func (a Adornments) enabled() bool {
	return a.PointLast || a.PointExtent || a.ReferenceBand != nil
}

// adornmentsFromMark extracts the spark adornment toggles from a mark
// definition. Returns the zero Adornments (nothing enabled) when m is
// nil.
func adornmentsFromMark(m *spec.MarkDef) Adornments {
	if m == nil {
		return Adornments{}
	}
	return Adornments{
		PointLast:     m.PointLast,
		PointExtent:   m.PointExtent,
		ReferenceBand: m.ReferenceBand,
	}
}

// encodeAdornments emits the opt-in adornment scene marks for a spark
// series. points are the encoded series points in plot-space pixels,
// one per datum in row order. yScale maps value-axis data to pixel y
// for the reference band. plot is the spark plot region — the band
// spans its full width. base supplies the spark's resolved style: dots
// inherit its stroke (line) color, the band a faint fill of the same.
//
// Order: the reference band is emitted first so it sits behind the
// extent and last-point dots in paint order.
//
// With no adornment enabled (or no points), the helper emits nothing
// (nil) — so a spark with these fields unset renders byte-identically
// to one without them. All geometry is snapped to render precision via
// roundTo (matching render.FormatFloat) so cross-impl goldens are
// stable.
func encodeAdornments(points [][2]float64, yScale Scale, plot scene.Rect, base scene.Style, ad Adornments) ([]scene.Mark, error) {
	if !ad.enabled() || len(points) == 0 {
		return nil, nil
	}

	var out []scene.Mark

	// Reference band — behind the series.
	if ad.ReferenceBand != nil && yScale != nil {
		y0, err := yScale.Apply(ad.ReferenceBand.From)
		if err != nil {
			return nil, err
		}
		y1, err := yScale.Apply(ad.ReferenceBand.To)
		if err != nil {
			return nil, err
		}
		top := math.Min(y0, y1)
		height := math.Abs(y1 - y0)
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    "adornment-band",
			Style: bandStyle(base),
			Rect: &scene.RectGeom{
				X: roundTo(plot.X, 3),
				Y: roundTo(top, 3),
				W: roundTo(plot.W, 3),
				H: roundTo(height, 3),
			},
		})
	}

	// Min/max extent dots. y pixels grow downward, so the smallest y is
	// the highest value and the largest y the lowest value.
	if ad.PointExtent {
		highIdx, lowIdx := extentIndices(points)
		out = append(out, dotMark("adornment-max", points[highIdx], dotStyle(base)))
		if lowIdx != highIdx {
			out = append(out, dotMark("adornment-min", points[lowIdx], dotStyle(base)))
		}
	}

	// Last-point dot.
	if ad.PointLast {
		out = append(out, dotMark("adornment-last", points[len(points)-1], dotStyle(base)))
	}

	return out, nil
}

// extentIndices returns the index of the highest-value point (smallest
// pixel y) and the lowest-value point (largest pixel y). On ties it
// keeps the first occurrence. points must be non-empty.
func extentIndices(points [][2]float64) (highIdx, lowIdx int) {
	highIdx, lowIdx = 0, 0
	for i := 1; i < len(points); i++ {
		if points[i][1] < points[highIdx][1] {
			highIdx = i
		}
		if points[i][1] > points[lowIdx][1] {
			lowIdx = i
		}
	}
	return highIdx, lowIdx
}

// dotMark builds a circle point mark at p with the adornment radius.
func dotMark(id string, p [2]float64, style scene.Style) scene.Mark {
	return scene.Mark{
		Type:  scene.MarkPoint,
		ID:    id,
		Style: style,
		Point: &scene.PointGeom{
			Cx:    roundTo(p[0], 3),
			Cy:    roundTo(p[1], 3),
			R:     adornmentDotRadius,
			Shape: scene.ShapeCircle,
		},
	}
}

// dotStyle derives an adornment-dot style from the spark's base style:
// the dot fills with the spark line color (stroke, falling back to
// fill).
func dotStyle(base scene.Style) scene.Style {
	s := scene.Style{}
	if base.Stroke != nil {
		s.Fill = base.Stroke
	} else if base.Fill != nil {
		s.Fill = base.Fill
	}
	return s
}

// bandStyle derives the reference-band style: a faint fill of the spark
// line color.
func bandStyle(base scene.Style) scene.Style {
	s := scene.Style{Opacity: adornmentBandOpacity}
	if base.Stroke != nil {
		s.Fill = base.Stroke
	} else if base.Fill != nil {
		s.Fill = base.Fill
	}
	return s
}
