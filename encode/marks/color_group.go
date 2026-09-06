package marks

import "github.com/frankbardon/prism/encode/scene"

// colorGroup is one color-partitioned subset of a mark's upstream
// rows: the row indices belonging to the group (in original upstream
// order) and the resolved per-group color (nil when no color channel
// is bound, or when the row's category has no palette match).
type colorGroup struct {
	category string
	indices  []int
	color    *scene.Color
	// varName (E4-S3) carries the "prism-resolved-N" var name when
	// in.ColorRegistry is active — the auto-dark counterpart to color
	// (see resolveCategoryColor). Empty means color (if non-nil) is a
	// baked literal, the pre-E4-S3 path.
	varName string
}

// groupRowsByColor partitions n upstream rows into per-category
// groups keyed by in.Color's bound field, mirroring the palette
// resolution encode.go already performs for the legend
// (lookupCategoryColor resolves the same in.Color.Categories /
// in.Color.Palette pair used there). Group order follows first
// appearance in the upstream table, which matches in.Color.Categories
// (built the same way in encode.go), so emission order lines up with
// the legend.
//
// Line/area marks connect points into a single polyline/ribbon per
// group — Vega-Lite semantics split a line/area mark into one path
// per distinct color (or detail) value rather than drawing one path
// across every row regardless of group. Detail-only splitting (no
// color channel bound) is a separate, larger gap: spec.Encoding's
// detail channel is decoded but never reaches marks.Inputs today, so
// it is out of scope here — see marks.Inputs' Color field, the only
// discrete-grouping channel currently wired through to mark encoders.
//
// When no color channel is bound (in.Color == nil or its Field is
// empty), returns a single group holding every row index 0..n-1 in
// original upstream order with a nil category/color — the pre-existing
// single-series behavior for line/area callers is preserved exactly.
func groupRowsByColor(in Inputs, n int) ([]colorGroup, error) {
	if in.Color == nil || in.Color.Field == "" {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return []colorGroup{{indices: idx}}, nil
	}

	colorVals, err := readField(in.Table, in.Color.Field)
	if err != nil {
		return nil, err
	}

	order := make([]string, 0)
	seen := map[string]bool{}
	byCat := map[string][]int{}
	for i := 0; i < n; i++ {
		cat := ""
		if i < len(colorVals) {
			if s, ok := colorVals[i].(string); ok {
				cat = s
			}
		}
		if !seen[cat] {
			seen[cat] = true
			order = append(order, cat)
		}
		byCat[cat] = append(byCat[cat], i)
	}

	groups := make([]colorGroup, 0, len(order))
	for _, cat := range order {
		c, v := resolveCategoryColor(in, cat)
		groups = append(groups, colorGroup{
			category: cat,
			indices:  byCat[cat],
			color:    c,
			varName:  v,
		})
	}
	return groups, nil
}
