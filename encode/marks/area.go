package marks

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeArea partitions rows by the bound discrete grouping channels
// — color and/or detail (Vega-Lite semantics: either one on an area
// mark splits it into one ribbon per distinct value — mirrors
// encodeLine) — and emits one scene.Mark per group, each carrying its
// own resolved fill color via groupRows / lookupCategoryColor — the
// same palette resolution the legend uses. A detail-only split leaves
// in.Style's fill untouched: detail groups series without consuming a
// palette slot or producing a legend. Within each group, points are
// sorted by resolved x pixel ascending so the ribbon traces
// left-to-right rather than upstream row order — unless the encoding
// binds `order` (E5-S4), which hands the point sequence to the
// author: the plan has already sorted the rows and each group traces
// in table order.
//
// Each group's Upper is its row-by-row points and Lower is the y=0
// baseline edge (one point per Upper x, snapped to the pixel where
// the data value is 0). The baseline is the scale's zero, so
// positive-only domains fill down to the plot bottom and
// zero-crossing domains fill above and below the mid-plot zero line.
//
// Stacking (E5-S2) needs no code here: the StackNode's bounds columns
// reach this encoder as an ordinary y / y2 pair (encode/stack.go
// rebinds the channels before scales resolve), so a stacked area is
// just the y2 path below with a per-series lower edge. The
// streamgraph offset lands later.
//
// Binding y2 (E9-S3) replaces that implicit baseline with an explicit
// lower edge read per row from the y2 column and resolved through the
// y scale — the band shape behind a confidence interval. The
// geometry is otherwise identical, so grouping, sorting and curve
// interpolation all carry over unchanged. x2 is not expressible on an
// area and is rejected at validate (PRISM_SPEC_041).
//
// When neither channel is bound, behavior is unchanged from before
// grouping existed: a single scene.Mark ("area-0") carrying every
// row's points in raw upstream order.
func encodeArea(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeArea: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	if len(xs) == 0 {
		return nil, nil
	}
	// Baseline = pixel y where the data value = 0 (mirrors bar.go).
	// Positive-only domains snap this to the plot bottom; zero-crossing
	// domains land it mid-plot. Fall back to the plot bottom on apply
	// failure (shouldn't happen for linear scales).
	baseline, err := in.Y.Scale.Apply(float64(0))
	if err != nil {
		baseline = in.Layout.Bottom()
	}
	// y2 (E9-S3), when bound, supplies the lower edge per row instead
	// of the baseline. Left nil otherwise, which keeps the baseline
	// path byte-identical.
	var lows []any
	if spanBound(in.Y2) {
		lows, err = readField(in.Table, in.Y2.Field)
		if err != nil {
			return nil, err
		}
		if len(lows) != len(xs) {
			return nil, fmt.Errorf("encodeArea: column length mismatch (x=%d, y2=%d)", len(xs), len(lows))
		}
	}
	upperAll := make([][2]float64, len(xs))
	lowerAll := make([][2]float64, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		y, err := in.Y.Scale.Apply(ys[i])
		if err != nil {
			return nil, err
		}
		low := baseline
		if lows != nil {
			if low, err = in.Y2.Scale.Apply(lows[i]); err != nil {
				return nil, err
			}
		}
		upperAll[i] = [2]float64{x, y}
		lowerAll[i] = [2]float64{x, low}
	}

	// An `order` binding (E5-S4) means the plan already sequenced the
	// rows; honour that sequence instead of the default left-to-right
	// x-sort, which is the whole point of the channel's path sense.
	sortByX := len(groupChannels(in)) > 0 && !in.Ordered
	groups, err := groupRows(in, len(xs))
	if err != nil {
		return nil, err
	}

	curve, tension := curveFor(in)

	marks := make([]scene.Mark, 0, len(groups))
	for gi, g := range groups {
		idxs := append([]int(nil), g.indices...)
		if sortByX {
			sort.SliceStable(idxs, func(a, b int) bool {
				return upperAll[idxs[a]][0] < upperAll[idxs[b]][0]
			})
		}
		upper := make([][2]float64, len(idxs))
		lower := make([][2]float64, len(idxs))
		for j, idx := range idxs {
			upper[j] = upperAll[idx]
			lower[j] = lowerAll[idx]
		}
		style := in.Style
		if g.color != nil || g.varName != "" {
			style.Fill = g.color
			style.FillVar = g.varName
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkArea,
			ID:    fmt.Sprintf("area-%d", gi),
			Style: style,
			Area: &scene.AreaGeom{
				Upper:   upper,
				Lower:   lower,
				Curve:   curve,
				Tension: tension,
			},
		})
	}
	return marks, nil
}
