package marks

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeLine partitions rows by the bound discrete grouping channels
// — color and/or detail (Vega-Lite semantics: either one on a line
// mark splits it into one polyline per distinct value) — and emits
// one scene.Mark per group, each carrying its own resolved stroke
// color via groupRows / lookupCategoryColor — the same palette
// resolution the legend uses, so per-group stroke colors match the
// legend swatches exactly. A detail-only split leaves in.Style's
// stroke untouched: detail groups series without consuming a palette
// slot or producing a legend. Within each group, points are sorted by
// resolved x pixel ascending so the polyline traces left-to-right
// rather than upstream row order (which may interleave groups, as in
// the gallery's multi_series_line fixture) — unless the encoding binds
// `order` (E5-S4), which hands the point sequence to the author: the
// plan has already sorted the rows and each group traces in table
// order.
//
// When neither channel is bound, behavior is unchanged from before
// grouping existed: a single scene.Mark ("line-0") carrying every
// row's (x, y) point in raw upstream order — sorting by x remains the
// caller's responsibility in that case (an explicit Sort transform),
// matching every other line fixture that has no color encoding.
func encodeLine(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeLine: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	if len(xs) == 0 {
		return nil, nil
	}

	pts := make([][2]float64, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		y, err := in.Y.Scale.Apply(ys[i])
		if err != nil {
			return nil, err
		}
		pts[i] = [2]float64{x, y}
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
				return pts[idxs[a]][0] < pts[idxs[b]][0]
			})
		}
		groupPts := make([][2]float64, len(idxs))
		for j, idx := range idxs {
			groupPts[j] = pts[idx]
		}
		style := in.Style
		if g.color != nil || g.varName != "" {
			style.Stroke = g.color
			style.StrokeVar = g.varName
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkLine,
			ID:    fmt.Sprintf("line-%d", gi),
			Style: style,
			Line: &scene.LineGeom{
				Points:  groupPts,
				Curve:   curve,
				Tension: tension,
			},
		})
	}
	return marks, nil
}
