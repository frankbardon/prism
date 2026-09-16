package marks

import (
	"fmt"
	"math"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// BoxplotSummary holds per-group statistics computed by the boxplot
// encoder. Exposed for parity tests (see boxplot_parity_test.go).
type BoxplotSummary struct {
	Group    string
	Q1       float64
	Median   float64
	Q3       float64
	Min      float64
	Max      float64
	ReachLow float64
	ReachHi  float64
	Outliers []float64
}

// encodeBoxplot emits primitive marks for each category group:
// 1 RectGeom (IQR box), 1 RuleGeom (median line), 2 RuleGeom
// (whisker stems), 2 RuleGeom (whisker caps), N PointGeom
// (outliers).
//
// Orientation (E9-S2) comes from MarkOrientation (orient.go): the
// category axis carries the band the box sits in and sizes it across
// its thickness, the measure axis carries the quantiles. `vertical`
// (band on x) is the default and historic shape; `horizontal` (band on
// y) lays the boxes out as rows. Every piece of the geometry — box,
// median, stems, caps, outliers — is expressed in category/measure
// terms and projected by OrientedRect / OrientedPoint.
//
// See D062 for the 1.5×IQR Tukey outlier rule + per-group Mark.ID
// prefix scheme.
func encodeBoxplot(in Inputs) ([]scene.Mark, error) {
	orient, err := MarkOrientation(in, "boxplot")
	if err != nil {
		return nil, err
	}
	summaries, err := ComputeBoxplotSummaries(in)
	if err != nil {
		return nil, err
	}
	category := CategoryChannel(in, orient)
	measure := MeasureChannel(in, orient)
	band, ok := category.Scale.(BandScaler)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"boxplot mark requires a band scale on the category axis.",
			map[string]any{"Field": category.Field, "Source": "<scale>", "Available": "band"},
		)
	}
	bandWidth := band.BandWidth()

	out := make([]scene.Mark, 0, len(summaries)*7)
	for _, s := range summaries {
		slotStart, err := category.Scale.Apply(s.Group)
		if err != nil {
			return nil, err
		}
		// A y band runs bottom-to-top, so its step is negative and
		// Apply returns the slot's far edge; normalise to
		// (start, positive length) exactly as CategorySlots does.
		width := bandWidth
		if width < 0 {
			slotStart, width = slotStart+width, -width
		}
		slot := [2]float64{slotStart, width}
		near, far := slotStart, slotStart+width
		center := slotStart + width/2

		m1, _ := measure.Scale.Apply(s.Q1)
		mM, _ := measure.Scale.Apply(s.Median)
		m3, _ := measure.Scale.Apply(s.Q3)
		mLo, _ := measure.Scale.Apply(s.ReachLow)
		mHi, _ := measure.Scale.Apply(s.ReachHi)

		// Box (IQR, q1 → q3), normalised so the rect extent is
		// positive whichever way the measure scale runs.
		boxSpan := [2]float64{m3, m1 - m3}
		if boxSpan[1] < 0 {
			boxSpan = [2]float64{m1, m3 - m1}
		}
		boxRect := OrientedRect(orient, slot, boxSpan)
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("boxplot-%s-box", s.Group),
			Style: in.Style,
			Rect:  &boxRect,
		})
		// Median line across the box.
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-median", s.Group),
			Style: in.Style,
			Rule:  orientedRule(orient, near, mM, far, mM),
		})
		// Upper whisker stem (q3 → reach hi).
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-stem-hi", s.Group),
			Style: in.Style,
			Rule:  orientedRule(orient, center, m3, center, mHi),
		})
		// Lower whisker stem (q1 → reach lo).
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-stem-lo", s.Group),
			Style: in.Style,
			Rule:  orientedRule(orient, center, m1, center, mLo),
		})
		// Upper whisker cap.
		capHalf := width * 0.25
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-cap-hi", s.Group),
			Style: in.Style,
			Rule:  orientedRule(orient, center-capHalf, mHi, center+capHalf, mHi),
		})
		// Lower whisker cap.
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-cap-lo", s.Group),
			Style: in.Style,
			Rule:  orientedRule(orient, center-capHalf, mLo, center+capHalf, mLo),
		})
		// Outliers as point marks.
		for i, v := range s.Outliers {
			mv, _ := measure.Scale.Apply(v)
			cx, cy := OrientedPoint(orient, center, mv)
			out = append(out, scene.Mark{
				Type:  scene.MarkPoint,
				ID:    fmt.Sprintf("boxplot-%s-out-%d", s.Group, i),
				Style: in.Style,
				Point: &scene.PointGeom{
					Cx:    cx,
					Cy:    cy,
					R:     2.5,
					Shape: scene.ShapeCircle,
				},
			})
		}
	}
	return out, nil
}

// orientedRule builds a RuleGeom from two (category, measure) pairs,
// projecting each onto scene-space through the mark's orientation.
func orientedRule(o Orientation, c1, m1, c2, m2 float64) *scene.RuleGeom {
	x1, y1 := OrientedPoint(o, c1, m1)
	x2, y2 := OrientedPoint(o, c2, m2)
	return &scene.RuleGeom{X1: x1, Y1: y1, X2: x2, Y2: y2}
}

// ComputeBoxplotSummaries partitions the table by the category axis
// and computes q1/median/q3/min/max + whisker reach + outliers per
// group. Which axis is the category one follows the mark's
// orientation, so a horizontal boxplot groups by y and summarises x.
// Exposed so the parity test can compare summaries directly against
// externally computed quantiles.
func ComputeBoxplotSummaries(in Inputs) ([]BoxplotSummary, error) {
	if in.X.Field == "" || in.Y.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"boxplot mark requires both x (category) and y (quantitative) channel bindings.",
			map[string]any{"Field": "<xy>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}
	orient, err := MarkOrientation(in, "boxplot")
	if err != nil {
		return nil, err
	}
	category := CategoryChannel(in, orient)
	measure := MeasureChannel(in, orient)
	xs, err := readField(in.Table, category.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, measure.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("boxplot: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(xs), orient.MeasureAxis(), len(ys))
	}

	// Group rows by category, preserving first-seen order.
	groupValues := map[string][]float64{}
	groupOrder := []string{}
	for i, xv := range xs {
		cat, ok := xv.(string)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("boxplot category value at row %d is not a string (got %T).", i, xv),
				map[string]any{"Field": category.Field, "Source": "<" + orient.CategoryAxis() + ">", "Available": "string"},
			)
		}
		yv, ok := toFloat64(ys[i])
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("boxplot value at row %d is not numeric (got %T).", i, ys[i]),
				map[string]any{"Field": measure.Field, "Source": "<" + orient.MeasureAxis() + ">", "Available": "numeric"},
			)
		}
		if _, seen := groupValues[cat]; !seen {
			groupOrder = append(groupOrder, cat)
		}
		groupValues[cat] = append(groupValues[cat], yv)
	}

	out := make([]BoxplotSummary, 0, len(groupOrder))
	for _, g := range groupOrder {
		vals := append([]float64(nil), groupValues[g]...)
		sort.Float64s(vals)
		q1 := Quantile(vals, 0.25)
		median := Quantile(vals, 0.50)
		q3 := Quantile(vals, 0.75)
		iqr := q3 - q1
		reachLowTarget := q1 - 1.5*iqr
		reachHiTarget := q3 + 1.5*iqr
		// Whiskers extend to most extreme actual datum within reach.
		minV := vals[0]
		maxV := vals[len(vals)-1]
		reachLow := minV
		for _, v := range vals {
			if v >= reachLowTarget {
				reachLow = v
				break
			}
		}
		reachHi := maxV
		for i := len(vals) - 1; i >= 0; i-- {
			if vals[i] <= reachHiTarget {
				reachHi = vals[i]
				break
			}
		}
		var outliers []float64
		for _, v := range vals {
			if v < reachLow || v > reachHi {
				outliers = append(outliers, v)
			}
		}
		out = append(out, BoxplotSummary{
			Group:    g,
			Q1:       q1,
			Median:   median,
			Q3:       q3,
			Min:      minV,
			Max:      maxV,
			ReachLow: reachLow,
			ReachHi:  reachHi,
			Outliers: outliers,
		})
	}
	return out, nil
}

// Quantile returns the q-th quantile (q in [0, 1]) of a sorted slice
// via linear interpolation between order statistics. Matches Pulse's
// AGG_PERCENTILE convention (R-7 quantile algorithm).
func Quantile(sorted []float64, q float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	if n == 1 {
		return sorted[0]
	}
	pos := q * float64(n-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo] + frac*(sorted[hi]-sorted[lo])
}
