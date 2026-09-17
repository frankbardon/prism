package marks

import (
	"fmt"
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

// appendSparkAdornments appends the opt-in adornment marks (E4) on top
// of a spark's base geometry and returns the combined slice. When no
// adornment field is set it returns base unchanged, so a bare spark
// renders byte-identically to one without the fields. Series points
// are recomputed from the X/Y scales (mirroring the base encoder; band
// x is centred so dots land on the bar) and handed to encodeAdornments.
func appendSparkAdornments(in Inputs, base []scene.Mark) ([]scene.Mark, error) {
	ad := adornmentsFromMark(in.Mark)
	if !ad.enabled() {
		return base, nil
	}
	orient, err := MarkOrientationOr(in, "spark", OrientVertical)
	if err != nil {
		return nil, err
	}
	pts, err := sparkSeriesPoints(in, orient)
	if err != nil {
		return nil, err
	}
	extra, err := encodeAdornments(pts, MeasureChannel(in, orient).Scale, orient, in.Layout, in.Style, ad)
	if err != nil {
		return nil, err
	}
	return append(base, extra...), nil
}

// sparkSeriesPoints resolves the spark's value points in plot-space
// pixels, one per row in upstream order. The category axis supplies
// the slot centre via CategoryCenters — which is the band midpoint on
// a sparkbar, so a dot sits over the column rather than its left edge,
// and the value's own pixel on a continuous axis — while the measure
// axis lands the adornment on the bar tip / line vertex / area crest.
// Orientation (E9-S2) decides which physical axis is which.
func sparkSeriesPoints(in Inputs, o Orientation) ([][2]float64, error) {
	centers, err := CategoryCenters(in, o)
	if err != nil {
		return nil, err
	}
	measure := MeasureChannel(in, o)
	vals, err := readField(in.Table, measure.Field)
	if err != nil {
		return nil, err
	}
	if len(centers) != len(vals) {
		return nil, fmt.Errorf("sparkSeriesPoints: column length mismatch (%s=%d, %s=%d)",
			o.CategoryAxis(), len(centers), o.MeasureAxis(), len(vals))
	}
	pts := make([][2]float64, 0, len(centers))
	for i := range centers {
		m, err := measure.Scale.Apply(vals[i])
		if err != nil {
			return nil, err
		}
		x, y := OrientedPoint(o, centers[i], m)
		pts = append(pts, [2]float64{x, y})
	}
	return pts, nil
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
// one per datum in row order. measureScale maps value-axis data to a
// pixel on the measure axis for the reference band. o is the spark's
// orientation (E9-S2), which decides which physical axis the band
// spans and which direction counts as "high". plot is the spark plot
// region — the band spans its full extent across the category axis.
// base supplies the spark's resolved style: dots inherit its stroke
// (line) color, the band a faint fill of the same.
//
// Order: the reference band is emitted first so it sits behind the
// extent and last-point dots in paint order.
//
// With no adornment enabled (or no points), the helper emits nothing
// (nil) — so a spark with these fields unset renders byte-identically
// to one without them. All geometry is snapped to render precision via
// roundTo (matching render.FormatFloat) so cross-impl goldens are
// stable.
func encodeAdornments(points [][2]float64, measureScale Scale, o Orientation, plot scene.Rect, base scene.Style, ad Adornments) ([]scene.Mark, error) {
	if !ad.enabled() || len(points) == 0 {
		return nil, nil
	}

	var out []scene.Mark

	// Reference band — behind the series. It spans the plot across the
	// category axis and covers [from, to] on the measure axis.
	if ad.ReferenceBand != nil && measureScale != nil {
		m0, err := measureScale.Apply(ad.ReferenceBand.From)
		if err != nil {
			return nil, err
		}
		m1, err := measureScale.Apply(ad.ReferenceBand.To)
		if err != nil {
			return nil, err
		}
		span := [2]float64{math.Min(m0, m1), math.Abs(m1 - m0)}
		rect := OrientedRect(o, PlotCategoryExtent(o, plot), span)
		rect.X = roundTo(rect.X, 3)
		rect.Y = roundTo(rect.Y, 3)
		rect.W = roundTo(rect.W, 3)
		rect.H = roundTo(rect.H, 3)
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    "adornment-band",
			Style: bandStyle(base),
			Rect:  &rect,
		})
	}

	// Min/max extent dots, read along the measure axis in the
	// direction a larger value points.
	if ad.PointExtent {
		highIdx, lowIdx := extentIndices(points, o)
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

// extentIndices returns the index of the highest-value point and the
// lowest-value point. "Highest" follows the measure axis's direction:
// vertically the smallest pixel y (the SVG y axis grows downward),
// horizontally the largest pixel x. On ties it keeps the first
// occurrence. points must be non-empty.
func extentIndices(points [][2]float64, o Orientation) (highIdx, lowIdx int) {
	axis := 1
	if o == OrientHorizontal {
		axis = 0
	}
	dir := MeasureDirection(o)
	highIdx, lowIdx = 0, 0
	for i := 1; i < len(points); i++ {
		if dir*points[i][axis] > dir*points[highIdx][axis] {
			highIdx = i
		}
		if dir*points[i][axis] < dir*points[lowIdx][axis] {
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
