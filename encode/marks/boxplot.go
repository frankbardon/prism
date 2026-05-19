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
// Orientation: vertical (x=category, y=quantitative). Horizontal
// (x=quantitative, y=category) is forward-looking; v1 covers the
// common vertical case used by the boxplot.json fixture.
//
// See D062 for the 1.5×IQR Tukey outlier rule + per-group Mark.ID
// prefix scheme.
func encodeBoxplot(in Inputs) ([]scene.Mark, error) {
	summaries, err := ComputeBoxplotSummaries(in)
	if err != nil {
		return nil, err
	}
	band, ok := in.X.Scale.(BandScaler)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"boxplot mark requires a band scale on the category axis.",
			map[string]any{"Field": in.X.Field, "Source": "<scale>", "Available": "band"},
		)
	}
	bandWidth := band.BandWidth()

	out := make([]scene.Mark, 0, len(summaries)*7)
	for _, s := range summaries {
		left, err := in.X.Scale.Apply(s.Group)
		if err != nil {
			return nil, err
		}
		right := left + bandWidth
		center := (left + right) / 2

		y1, _ := in.Y.Scale.Apply(s.Q1)
		yM, _ := in.Y.Scale.Apply(s.Median)
		y3, _ := in.Y.Scale.Apply(s.Q3)
		yLo, _ := in.Y.Scale.Apply(s.ReachLow)
		yHi, _ := in.Y.Scale.Apply(s.ReachHi)

		// Box (IQR, q1 → q3). In SVG y-inverted, y3 is on top, y1 on bottom.
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("boxplot-%s-box", s.Group),
			Style: in.Style,
			Rect: &scene.RectGeom{
				X: left,
				Y: y3,
				W: bandWidth,
				H: y1 - y3,
			},
		})
		// Median line across the box.
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-median", s.Group),
			Style: in.Style,
			Rule:  &scene.RuleGeom{X1: left, Y1: yM, X2: right, Y2: yM},
		})
		// Upper whisker stem (q3 → reach hi).
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-stem-hi", s.Group),
			Style: in.Style,
			Rule:  &scene.RuleGeom{X1: center, Y1: y3, X2: center, Y2: yHi},
		})
		// Lower whisker stem (q1 → reach lo).
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-stem-lo", s.Group),
			Style: in.Style,
			Rule:  &scene.RuleGeom{X1: center, Y1: y1, X2: center, Y2: yLo},
		})
		// Upper whisker cap.
		capHalf := bandWidth * 0.25
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-cap-hi", s.Group),
			Style: in.Style,
			Rule:  &scene.RuleGeom{X1: center - capHalf, Y1: yHi, X2: center + capHalf, Y2: yHi},
		})
		// Lower whisker cap.
		out = append(out, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("boxplot-%s-w-cap-lo", s.Group),
			Style: in.Style,
			Rule:  &scene.RuleGeom{X1: center - capHalf, Y1: yLo, X2: center + capHalf, Y2: yLo},
		})
		// Outliers as point marks.
		for i, v := range s.Outliers {
			yv, _ := in.Y.Scale.Apply(v)
			out = append(out, scene.Mark{
				Type:  scene.MarkPoint,
				ID:    fmt.Sprintf("boxplot-%s-out-%d", s.Group, i),
				Style: in.Style,
				Point: &scene.PointGeom{
					Cx:    center,
					Cy:    yv,
					R:     2.5,
					Shape: scene.ShapeCircle,
				},
			})
		}
	}
	return out, nil
}

// ComputeBoxplotSummaries partitions the table by the category axis
// (in.X.Field) and computes q1/median/q3/min/max + whisker reach +
// outliers per group. Exposed so the parity test can compare
// summaries directly against Pulse-mapped quantiles.
func ComputeBoxplotSummaries(in Inputs) ([]BoxplotSummary, error) {
	if in.X.Field == "" || in.Y.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"boxplot mark requires both x (category) and y (quantitative) channel bindings.",
			map[string]any{"Field": "<xy>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
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
		return nil, fmt.Errorf("boxplot: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
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
				map[string]any{"Field": in.X.Field, "Source": "<x>", "Available": "string"},
			)
		}
		yv, ok := toFloat64(ys[i])
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("boxplot value at row %d is not numeric (got %T).", i, ys[i]),
				map[string]any{"Field": in.Y.Field, "Source": "<y>", "Available": "numeric"},
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
