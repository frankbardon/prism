package marks

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// HistogramResult bundles the encoded marks with the synthetic
// scales the histogram encoder builds inline. The encode.go path
// uses XScale / YScale to build axes; standard callers via
// marks.Encode read only Marks. See D060.
//
// XScale / YScale are exported as concrete *scale.LinearScale so
// callers can hand them to encode.BuildAxisWithOpts (which expects
// the richer encode.Scale interface, not the minimal marks.Scale).
type HistogramResult struct {
	Marks    []scene.Mark
	XScale   *scale.LinearScale
	YScale   *scale.LinearScale
	BinEdges []float64
	Counts   []int
}

// EncodeHistogram builds bins inline, counts rows per bin, then
// emits one RectGeom per bin. Defaults to Sturges' rule
// (ceil(log2(n) + 1)) for bin count; honors mark_def.maxbins when
// set.
//
// Bin edges use the nice-step algorithm from compile/inmem/bin.go
// (duplicated here per D060; cross-package re-export was rejected to
// keep encode/marks dependency-free of compile/).
func EncodeHistogram(in Inputs) (*HistogramResult, error) {
	if in.X.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"histogram mark requires an x channel binding.",
			map[string]any{"Field": "<x>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}
	values, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return &HistogramResult{}, nil
	}
	nums := make([]float64, 0, len(values))
	for i, v := range values {
		f, ok := toFloat64(v)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("histogram x value at row %d is not numeric (got %T).", i, v),
				map[string]any{"Field": in.X.Field, "Source": "<x>", "Available": "numeric"},
			)
		}
		nums = append(nums, f)
	}

	lo := nums[0]
	hi := nums[0]
	for _, v := range nums[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if hi == lo {
		// Single value — emit one degenerate bin.
		hi = lo + 1
	}

	// Bin count: Sturges' rule default; mark_def.maxbins overrides.
	maxbins := int(math.Ceil(math.Log2(float64(len(nums)))) + 1)
	if maxbins < 1 {
		maxbins = 1
	}
	if in.Mark != nil && in.Mark.Maxbins != nil && *in.Mark.Maxbins > 0 {
		maxbins = *in.Mark.Maxbins
	}

	width := histogramNiceStep(lo, hi, maxbins)
	if width == 0 {
		width = (hi - lo) / float64(maxbins)
	}

	// Build edges + counts.
	nbins := int(math.Ceil((hi - lo) / width))
	if nbins < 1 {
		nbins = 1
	}
	edges := make([]float64, nbins+1)
	for i := 0; i <= nbins; i++ {
		edges[i] = lo + float64(i)*width
	}
	counts := make([]int, nbins)
	for _, v := range nums {
		idx := int(math.Floor((v - lo) / width))
		if idx < 0 {
			idx = 0
		}
		if idx >= nbins {
			idx = nbins - 1
		}
		counts[idx]++
	}

	// Synthetic linear scales over [edges[0], edges[N]] and [0, max].
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	xScale := &scale.LinearScale{
		DomainMin: edges[0],
		DomainMax: edges[nbins],
		RangeMin:  in.Layout.X,
		RangeMax:  in.Layout.X + in.Layout.W,
	}
	yScale := &scale.LinearScale{
		DomainMin: 0,
		DomainMax: float64(maxCount),
		RangeMin:  in.Layout.Y + in.Layout.H,
		RangeMax:  in.Layout.Y,
	}

	// Emit one RectGeom per bin.
	baseline, _ := yScale.Apply(float64(0))
	marks := make([]scene.Mark, 0, nbins)
	for i, c := range counts {
		x, _ := xScale.Apply(edges[i])
		x2, _ := xScale.Apply(edges[i+1])
		y, _ := yScale.Apply(float64(c))
		top, h := y, baseline-y
		if h < 0 {
			top = baseline
			h = -h
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("hist-%d", i),
			Style: in.Style,
			Rect: &scene.RectGeom{
				X: x,
				Y: top,
				W: x2 - x,
				H: h,
			},
		})
	}

	return &HistogramResult{
		Marks:    marks,
		XScale:   xScale,
		YScale:   yScale,
		BinEdges: edges,
		Counts:   counts,
	}, nil
}

// histogramNiceStep is a copy of compile/inmem.niceStep — kept here
// to avoid pulling compile/ into encode/marks. See D060.
func histogramNiceStep(lo, hi float64, maxbins int) float64 {
	if hi == lo || maxbins <= 0 {
		return 0
	}
	rough := (hi - lo) / float64(maxbins)
	if rough <= 0 {
		return 0
	}
	mag := math.Pow(10, math.Floor(math.Log10(rough)))
	frac := rough / mag
	var nice float64
	switch {
	case frac < 1.5:
		nice = 1
	case frac < 3:
		nice = 2
	case frac < 7:
		nice = 5
	default:
		nice = 10
	}
	return nice * mag
}

// toFloat64 coerces common numeric kinds to float64.
func toFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case int:
		return float64(t), true
	case uint64:
		return float64(t), true
	}
	return 0, false
}
