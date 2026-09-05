package marks

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeArea partitions rows by the bound color channel (Vega-Lite
// semantics: color/detail on an area mark splits it into one ribbon
// per distinct value — mirrors encodeLine) and emits one scene.Mark
// per group, each carrying its own resolved fill color via
// groupRowsByColor / lookupCategoryColor — the same palette
// resolution the legend uses. Within each group, points are sorted by
// resolved x pixel ascending so the ribbon traces left-to-right
// rather than upstream row order.
//
// Each group's Upper is its row-by-row points and Lower is the y=0
// baseline edge (one point per Upper x, snapped to the pixel where
// the data value is 0). The baseline is the scale's zero, so
// positive-only domains fill down to the plot bottom and
// zero-crossing domains fill above and below the mid-plot zero line.
// Stacked / streamgraph variants land in P08.
//
// When no color channel is bound, behavior is unchanged from before
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
		upperAll[i] = [2]float64{x, y}
		lowerAll[i] = [2]float64{x, baseline}
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
		if g.color != nil {
			style.Fill = g.color
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkArea,
			ID:    fmt.Sprintf("area-%d", gi),
			Style: style,
			Area: &scene.AreaGeom{
				Upper: upper,
				Lower: lower,
				Curve: scene.CurveLinear,
			},
		})
	}
	return marks, nil
}
