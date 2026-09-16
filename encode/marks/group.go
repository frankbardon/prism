package marks

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
)

// groupChannel is one discrete grouping channel a mark partitions its
// upstream rows on: the table column to read, plus whether the
// channel participates in palette resolution.
//
// Only the color channel is a palette consumer. Detail channels
// (E5-S1) split a mark into separate series without consuming a
// palette slot, producing a legend entry, or altering mark styling —
// that asymmetry is the whole point of Vega-Lite's detail channel.
type groupChannel struct {
	field string
	// palette is true only for the color channel. A group's category
	// (and therefore its resolved color) is read from the palette
	// channel alone; detail components never reach
	// resolveCategoryColor.
	palette bool
}

// rowGroup is one partitioned subset of a mark's upstream rows: the
// row indices belonging to the group (in original upstream order),
// the key values it was partitioned on, and the resolved per-group
// color (nil when no color channel is bound, or when the row's
// category has no palette match).
type rowGroup struct {
	// category is the color channel's value for this group, or "" when
	// no color channel is bound. Detail-only groups keep it empty so
	// styling stays untouched.
	category string
	// detail holds the detail-channel values positionally aligned with
	// the detail fields groupRows partitioned on. Empty when no detail
	// channel is bound.
	detail  []string
	indices []int
	color   *scene.Color
	// varName (E4-S3) carries the "prism-resolved-N" var name when
	// in.ColorRegistry is active — the auto-dark counterpart to color
	// (see resolveCategoryColor). Empty means color (if non-nil) is a
	// baked literal, the pre-E4-S3 path.
	varName string
}

// groupChannels returns the ordered list of discrete grouping
// channels bound on in: the color channel first (when bound), then
// every detail-channel field in spec order.
//
// Returns nil when neither is bound, which callers use as the
// "single ungrouped series" signal (line / area skip their per-group
// x-sort in that case, preserving raw upstream point order).
func groupChannels(in Inputs) []groupChannel {
	var out []groupChannel
	if in.Color != nil && in.Color.Field != "" {
		out = append(out, groupChannel{field: in.Color.Field, palette: true})
	}
	for _, f := range in.Detail {
		if f == "" {
			continue
		}
		out = append(out, groupChannel{field: f})
	}
	return out
}

// groupRows partitions n upstream rows into groups keyed by the tuple
// of every bound discrete grouping channel (color, then each detail
// field — see groupChannels).
//
// Palette resolution mirrors what encode.go already performs for the
// legend (resolveCategoryColor resolves the same in.Color.Categories
// / in.Color.Palette pair used there), and keys on the color
// component of the tuple alone, so detail never consumes a palette
// slot.
//
// Line/area marks connect points into a single polyline/ribbon per
// group — Vega-Lite semantics split a line/area mark into one path
// per distinct (color, detail…) value rather than drawing one path
// across every row regardless of group.
//
// Ordering: groups are emitted in first-appearance order of the color
// value, and within one color in first-appearance order of the full
// tuple. With no detail bound this reduces exactly to the historical
// groupRowsByColor contract — first appearance of the color value in
// the upstream table, which matches in.Color.Categories (built the
// same way in encode.go), so emission order lines up with the legend.
// With detail bound, every group sharing a color still emits
// contiguously and in legend order, so the color component of the
// emission order is unchanged.
//
// When no grouping channel is bound at all, returns a single group
// holding every row index 0..n-1 in original upstream order with an
// empty category and nil color — the pre-existing single-series
// behavior for line/area callers, preserved exactly.
func groupRows(in Inputs, n int) ([]rowGroup, error) {
	chans := groupChannels(in)
	if len(chans) == 0 {
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		return []rowGroup{{indices: idx}}, nil
	}

	// Read every grouping column once, in channel order.
	cols := make([][]any, len(chans))
	for i, ch := range chans {
		vals, err := readField(in.Table, ch.field)
		if err != nil {
			return nil, err
		}
		cols[i] = vals
	}

	type bucket struct {
		category string
		detail   []string
		colorOrd int
		firstIdx int
		indices  []int
	}
	var (
		buckets    []*bucket
		byKey      = map[string]*bucket{}
		colorOrder = map[string]int{}
	)
	for i := 0; i < n; i++ {
		category := ""
		detail := make([]string, 0, len(chans))
		key := ""
		for c, ch := range chans {
			var raw any
			if i < len(cols[c]) {
				raw = cols[c][i]
			}
			var part string
			if ch.palette {
				// The color category keeps its historical string-only
				// coercion: a non-string value buckets under "", which
				// is also what lookupCategoryColor matches on. Changing
				// it would move existing color-grouped output.
				part = colorCategoryOf(raw)
				category = part
			} else {
				part = detailKeyOf(raw)
				detail = append(detail, part)
			}
			// Length-prefix each component so ("a", "b|c") and
			// ("a|b", "c") cannot collide on the joined key.
			key += fmt.Sprintf("%d:%s|", len(part), part)
		}
		if b, ok := byKey[key]; ok {
			b.indices = append(b.indices, i)
			continue
		}
		ord, seen := colorOrder[category]
		if !seen {
			ord = len(colorOrder)
			colorOrder[category] = ord
		}
		b := &bucket{
			category: category,
			detail:   detail,
			colorOrd: ord,
			firstIdx: i,
			indices:  []int{i},
		}
		byKey[key] = b
		buckets = append(buckets, b)
	}

	// Color first-appearance order outer, tuple first-appearance order
	// inner. Stable so the comparison never has to break ties itself.
	sort.SliceStable(buckets, func(a, b int) bool {
		if buckets[a].colorOrd != buckets[b].colorOrd {
			return buckets[a].colorOrd < buckets[b].colorOrd
		}
		return buckets[a].firstIdx < buckets[b].firstIdx
	})

	groups := make([]rowGroup, 0, len(buckets))
	for _, b := range buckets {
		g := rowGroup{
			category: b.category,
			detail:   b.detail,
			indices:  b.indices,
		}
		if in.Color != nil && in.Color.Field != "" {
			g.color, g.varName = resolveCategoryColor(in, b.category)
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// colorCategoryOf coerces a color-channel cell to its category key.
// Only strings are categories; anything else (including nil and
// numbers) buckets under "" — the historical groupRowsByColor
// behavior, retained verbatim so color-only specs keep producing
// byte-identical output.
func colorCategoryOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// detailKeyOf coerces a detail-channel cell to its grouping key.
// Unlike colorCategoryOf this stringifies every value, because a
// detail field is frequently a numeric series id and collapsing all
// of those into one bucket would defeat the channel entirely. Detail
// keys never feed palette lookup, so the looser coercion is safe.
func detailKeyOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
