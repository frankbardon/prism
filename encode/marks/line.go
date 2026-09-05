package marks

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeLine partitions rows by the bound color channel (Vega-Lite
// semantics: color/detail on a line mark splits it into one polyline
// per distinct value) and emits one scene.Mark per group, each
// carrying its own resolved stroke color via groupRowsByColor /
// lookupCategoryColor — the same palette resolution the legend uses,
// so per-group stroke colors match the legend swatches exactly.
// Within each group, points are sorted by resolved x pixel ascending
// so the polyline traces left-to-right rather than upstream row
// order (which may interleave groups, as in the gallery's
// multi_series_line fixture).
//
// When no color channel is bound, behavior is unchanged from before
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

	grouped := in.Color != nil && in.Color.Field != ""
	groups, err := groupRowsByColor(in, len(xs))
	if err != nil {
		return nil, err
	}

	marks := make([]scene.Mark, 0, len(groups))
	for gi, g := range groups {
		idxs := append([]int(nil), g.indices...)
		if grouped {
			sort.SliceStable(idxs, func(a, b int) bool {
				return pts[idxs[a]][0] < pts[idxs[b]][0]
			})
		}
		groupPts := make([][2]float64, len(idxs))
		for j, idx := range idxs {
			groupPts[j] = pts[idx]
		}
		style := in.Style
		if g.color != nil {
			style.Stroke = g.color
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkLine,
			ID:    fmt.Sprintf("line-%d", gi),
			Style: style,
			Line: &scene.LineGeom{
				Points: groupPts,
				Curve:  scene.CurveLinear,
			},
		})
	}
	return marks, nil
}
