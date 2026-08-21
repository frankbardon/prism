package marks

import "github.com/frankbardon/prism/encode/scene"

// series.go splits a table into one run per colour category, for the
// marks whose geometry is a CONNECTED PATH rather than one shape per
// row.
//
// Without it, a line bound to a colour channel produced a single
// polyline over every row in table order: the path ran to the end of
// the first brand, jumped back across the chart to the start of the
// second, and so on, and every "series" was drawn in the same colour
// while the legend confidently listed six. The chart was wrong, not
// merely plain — it drew connections between measurements that have
// nothing to do with each other.
//
// Splitting also fixes what the legend claims: each run now carries
// its own palette colour, so the swatch beside "Borealis" is the
// colour the Borealis line is actually drawn in.

// seriesRun is one colour category's rows, in table order.
type seriesRun struct {
	// Category is the colour value this run belongs to. Empty when the
	// chart has no colour channel — the single-series case, which is
	// one run over every row.
	Category string
	// Rows are indices into the source table, ascending.
	Rows []int
	// Color is the run's resolved colour, nil for the single-series
	// case (the mark keeps its themed default).
	Color *scene.Color
}

// splitSeries groups row indices by the colour channel's value.
//
// Category ORDER follows Color.Categories, which is the order the
// legend lists and the palette assigns from — so the nth series and
// the nth legend entry are the same thing. Rows within a run keep
// their table order, because that order is the caller's (a sort
// transform upstream, or the order the data arrived in) and a line's
// direction is data, not decoration.
//
// A row whose colour value is not a string, or names a category the
// channel does not list, is dropped from every run rather than
// silently folded into the first: it belongs to no series, and
// guessing one would draw a segment that does not exist.
func splitSeries(in Inputs, n int) []seriesRun {
	if in.Color == nil || in.Color.Field == "" || len(in.Color.Categories) < 1 {
		rows := make([]int, n)
		for i := range rows {
			rows[i] = i
		}
		return []seriesRun{{Rows: rows}}
	}
	vals, err := readField(in.Table, in.Color.Field)
	if err != nil {
		rows := make([]int, n)
		for i := range rows {
			rows[i] = i
		}
		return []seriesRun{{Rows: rows}}
	}

	index := make(map[string]int, len(in.Color.Categories))
	for i, c := range in.Color.Categories {
		index[c] = i
	}
	runs := make([]seriesRun, len(in.Color.Categories))
	for i, c := range in.Color.Categories {
		runs[i] = seriesRun{
			Category: c,
			Color:    lookupCategoryColor(c, in.Color.Categories, in.Color.Palette),
		}
	}
	for row := 0; row < n && row < len(vals); row++ {
		cat, ok := vals[row].(string)
		if !ok {
			continue
		}
		i, ok := index[cat]
		if !ok {
			continue
		}
		runs[i].Rows = append(runs[i].Rows, row)
	}
	// Drop empty runs so a category present in the domain but absent
	// from this layer's rows does not emit a zero-point mark.
	out := runs[:0]
	for _, r := range runs {
		if len(r.Rows) > 0 {
			out = append(out, r)
		}
	}
	return out
}
