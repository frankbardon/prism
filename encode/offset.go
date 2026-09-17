package encode

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Offset position channels (E1-S4).
//
// `x_offset` / `y_offset` subdivide the band slot a category owns so
// rows sharing one category value dodge instead of overlapping. The
// subdivision is a band scale in its own right: its RANGE is the
// parent band's extent — (0, parent BandWidth()), signed, so the
// displacement it produces carries the parent's direction — and its
// DOMAIN is the global set of distinct offset values across the whole
// table.
//
// Global is the load-bearing word. A per-category domain would size a
// lone member's bar to the full slot while a two-member category got
// half-slots, and bar widths would stop being comparable between
// groups, which is the one thing a grouped bar exists to make
// possible. Resolving the domain once, off the whole column, is what
// gives every group the same sub-band width.
//
// The scale is built through NewBandScale — the one band constructor —
// so the flat encoder and the shared-scale path in encode_composite.go
// cannot drift apart on a padding knob.

const (
	// defaultOffsetPaddingInner / Outer are the offset scale's OWN
	// padding defaults, and they are zero rather than the 0.1 / 0.05 a
	// top-level band scale takes.
	//
	// Two consequences, both wanted. Sub-bands touch and together fill
	// the parent slot exactly, which is the classic grouped-bar look
	// and matches Vega-Lite's offset-scale defaults. And a single
	// distinct offset value reduces to one sub-band spanning the whole
	// slot at zero displacement — geometry byte-identical to the same
	// spec with the channel removed.
	//
	// The parent's defaults are untouched: they reproduce the
	// pre-split layout byte-for-byte and moving them would move every
	// committed bar, tick, heatmap and boxplot golden. Within-group
	// spacing is configured on the offset channel's own `scale` block.
	defaultOffsetPaddingInner = 0.0
	defaultOffsetPaddingOuter = 0.0
)

// resolveOffsetBinding builds the marks.OffsetBinding for enc, or the
// zero binding when no offset is in force.
//
// The decision "is an offset bound, on which axis, reading which
// field" is not made here — spec.ResolveOffset makes it, and the
// planner (which keeps the offset field alive through the synthetic
// aggregate's group-by) asks the same function about the same spec.
//
// The resolver stays TOTAL where validate is the reporter: an offset
// on an axis that carries no band scale, or on a mark that draws no
// band at all, yields the zero binding here and is rejected at
// validate (PRISM_SPEC_063 / PRISM_SPEC_064) with a message naming the
// spec. What it does reject is the incoherence validate cannot see
// from the spec alone — a declared type that cannot subdivide a slot,
// and a `sort` spelling nothing acts on. Those would otherwise be
// silent no-ops, the failure class this repo has already shipped three
// times.
func resolveOffsetBinding(enc *spec.Encoding, tbl *table.Table, xScale, yScale marks.Scale) (marks.OffsetBinding, error) {
	bind := spec.ResolveOffset(enc)
	if bind == nil || tbl == nil {
		return marks.OffsetBinding{}, nil
	}
	parent := xScale
	if bind.Channel == "y" {
		parent = yScale
	}
	if parent == nil {
		return marks.OffsetBinding{}, nil
	}
	band, ok := parent.(marks.BandScaler)
	if !ok {
		return marks.OffsetBinding{}, nil
	}
	channel := bind.Channel + "_offset"
	switch bind.Offset.Type {
	case "", "nominal", "ordinal":
		// Discrete: a band slot can be cut into sub-bands by it.
	default:
		return marks.OffsetBinding{}, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Offset channel %s declares type %q; a band slot is subdivided by a discrete field.", channel, bind.Offset.Type),
			map[string]any{"Field": bind.Field, "Source": "<encoding>", "Available": "nominal, ordinal"},
		)
	}
	col, ok := tbl.Column(bind.Field)
	if !ok {
		return marks.OffsetBinding{}, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Offset channel field %q not present in upstream table.", bind.Field),
			map[string]any{"Field": bind.Field, "Source": "<table>", "Available": joinTableFields(tbl)},
		)
	}
	values := make([]any, col.Len())
	for i := 0; i < col.Len(); i++ {
		values[i] = col.ValueAt(i)
	}
	opts := offsetScaleOpts(bind.Offset)
	cats, err := offsetCategories(values, bind.Offset, opts, channel)
	if err != nil {
		return marks.OffsetBinding{}, err
	}
	return marks.OffsetBinding{
		Axis:  bind.Channel,
		Field: bind.Field,
		Scale: NewBandScale(cats, 0, band.BandWidth(), opts),
	}, nil
}

// offsetScaleOpts lifts the offset channel's own `scale` block and
// seeds the offset-specific padding defaults into it.
//
// Seeding rather than branching inside ScaleOpts.bandPadding keeps the
// parent band scale's defaults exactly where they are: this function
// fills in what the author did not write, and anything the author DID
// write — `padding`, `padding_inner`, `padding_outer`, `align`,
// `reverse`, `round`, `domain` — flows through untouched, so
// within-group spacing is configurable independently of the spacing
// between groups.
func offsetScaleOpts(ch *spec.OffsetChannel) ScaleOpts {
	opts := ScaleOptsFromSpec(ch.Scale)
	if opts.Padding != nil {
		// The shorthand set both; leave it alone.
		return opts
	}
	if opts.PaddingInner == nil {
		inner := defaultOffsetPaddingInner
		opts.PaddingInner = &inner
	}
	if opts.PaddingOuter == nil {
		outer := defaultOffsetPaddingOuter
		opts.PaddingOuter = &outer
	}
	return opts
}

// offsetCategories resolves the sub-band order.
//
// Precedence, highest first: an explicit `scale.domain`, then a `sort`
// naming an explicit category array, then a `sort` direction over the
// distinct values. The first two reuse bandCategories, so an entry the
// domain does not list is appended rather than dropped — the same rule
// every other discrete scale in Prism follows.
//
// With none of them the order is the distinct values ASCENDING, never
// first-seen table order. That is a considered divergence from the
// position channels: an offset's order decides which sub-band a series
// sits in, and deriving it from row order would make the same data
// draw differently after an upstream sort. It is also what Vega-Lite
// does for a discrete channel, where "ascending" is the documented
// default.
func offsetCategories(values []any, ch *spec.OffsetChannel, opts ScaleOpts, channel string) ([]string, error) {
	if len(opts.Domain) > 0 {
		return bandCategories(values, opts, channel, "<"+channel+">")
	}
	if list, ok := offsetSortList(ch.Sort); ok {
		pinned := opts
		pinned.Domain = list
		return bandCategories(values, pinned, channel, "<"+channel+">")
	}
	descending, err := offsetSortDescending(ch.Sort, channel)
	if err != nil {
		return nil, err
	}
	cats := uniqueStrings(values)
	if len(cats) == 0 {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Offset channel %s: no string categories in domain.", channel),
			map[string]any{"Field": "<" + channel + ">", "Source": "<scale>", "Available": "string"},
		)
	}
	sort.Strings(cats)
	if descending {
		for i, j := 0, len(cats)-1; i < j; i, j = i+1, j-1 {
			cats[i], cats[j] = cats[j], cats[i]
		}
	}
	return cats, nil
}

// offsetSortList reads a `sort` that names the category order
// outright. Reports false for every other shape, including a list
// holding a non-string, which falls through to the direction reading
// and is rejected there rather than half-honoured.
func offsetSortList(v any) ([]any, bool) {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	for _, it := range items {
		if _, ok := it.(string); !ok {
			return nil, false
		}
	}
	return items, true
}

// offsetSortDescending reads a `sort` direction. Absent and null both
// mean ascending. An unrecognised spelling is an ERROR, not a fallback
// to ascending: a direction that silently does nothing is exactly the
// bug `sort: "descending"` shipped as before E5-S4.
func offsetSortDescending(v any, channel string) (bool, error) {
	switch s := v.(type) {
	case nil:
		return false, nil
	case string:
		if !spec.SortDirectionValid(s) {
			return false, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Offset channel %s declares unknown sort %q.", channel, s),
				map[string]any{"Field": "<" + channel + ">", "Source": "<sort>", "Available": "ascending, descending, asc, desc"},
			)
		}
		return spec.SortDirectionDescending(s), nil
	}
	return false, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("Offset channel %s declares a sort shape nothing reads.", channel),
		map[string]any{"Field": "<" + channel + ">", "Source": "<sort>", "Available": "ascending, descending, asc, desc, [category, …]"},
	)
}

// Duplicate sub-band keys (E2-S3).
//
// A sub-band is identified by the pair (category value, offset
// value). Two rows carrying the same pair land on the same rect and
// only the last one drawn stays visible — the marks still overlap,
// which is the one thing binding an offset channel was supposed to
// stop. Vega-Lite says nothing here. Saying nothing would reproduce,
// one level down, the invisible failure this channel exists to
// remove: an author binds the offset, sees bars still stacked on one
// another, and has nothing to read.
//
// So it is reported, and nothing geometric changes. Both rects are
// emitted exactly as before; the author gets a PRISM_WARN_OFFSET_COLLISION
// on SceneDoc.Warnings naming how many rows repeated and one key that
// did.
//
// It is unreachable once the measure channel aggregates: the
// synthetic group-by injected in plan/build keeps both the category
// field and the offset field, so each pair yields exactly one row.
// Raw un-aggregated tables are the only place duplicates survive to
// encode.

// offsetCollisionWarning reports the rows of tbl that repeat a
// (category, offset) pair an earlier row already claimed, or nil when
// every pair is distinct.
//
// The category is read off the position channel the offset
// subdivides — the same field encode/marks/span.go hands to the
// parent band scale — so the key this names is exactly the key the
// geometry keyed on. Rows masked out by mark.invalid: "break" are not
// counted: a row that is never drawn cannot overlap one that is.
func offsetCollisionWarning(enc *spec.Encoding, bind marks.OffsetBinding, tbl *table.Table, skip []bool, layerID string) *scene.Warning {
	if enc == nil || tbl == nil || !bind.On(bind.Axis) {
		return nil
	}
	catField := fieldOf(enc.X)
	if bind.Axis == "y" {
		catField = fieldOf(enc.Y)
	}
	if catField == "" {
		return nil
	}
	catCol, ok := tbl.Column(catField)
	if !ok {
		return nil
	}
	offCol, ok := tbl.Column(bind.Field)
	if !ok {
		return nil
	}
	rows := tbl.NumRows()
	if rows > catCol.Len() {
		rows = catCol.Len()
	}
	if rows > offCol.Len() {
		rows = offCol.Len()
	}

	seen := make(map[string]bool, rows)
	count := 0
	example := ""
	for i := 0; i < rows; i++ {
		if i < len(skip) && skip[i] {
			continue
		}
		cat, off := offsetKeyString(catCol.ValueAt(i)), offsetKeyString(offCol.ValueAt(i))
		// The NUL separator keeps ("a", "b\x00c") from colliding with
		// ("a\x00b", "c") — a collision report that was itself a
		// collision would be a poor advertisement.
		key := cat + "\x00" + off
		if seen[key] {
			count++
			if count == 1 {
				example = offsetCollisionKey(catField, cat, bind.Field, off)
			}
			continue
		}
		seen[key] = true
	}
	if count == 0 {
		return nil
	}
	return &scene.Warning{
		Code:    scene.WarnOffsetCollision,
		Layer:   layerID,
		Message: offsetCollisionMessage(count, example),
		Details: map[string]any{
			"count":          count,
			"key":            example,
			"channel":        bind.Axis + "_offset",
			"category_field": catField,
			"offset_field":   bind.Field,
		},
	}
}

// offsetCollisionMessage is the one place the warning's prose lives,
// so the per-layer emission and the collapsed whole-chart entry
// cannot word the same fact two ways.
func offsetCollisionMessage(count int, key string) string {
	rows, repeat := "rows", "repeat"
	if count == 1 {
		rows, repeat = "row", "repeats"
	}
	return fmt.Sprintf(
		"%d %s %s an offset key already drawn — first repeat %s — so their marks share one sub-band and overlap.",
		count, rows, repeat, key)
}

// offsetCollisionKey renders a key an author can read against what
// they wrote: `quarter=Q1, series=Nike`, never a struct dump.
func offsetCollisionKey(catField, cat, offField, off string) string {
	return catField + "=" + cat + ", " + offField + "=" + off
}

// offsetKeyString coerces a cell to its sub-band key. Every value is
// stringified rather than only strings, the way detailKeyOf does:
// a category or offset column is frequently a numeric id, and
// bucketing all of those together would report collisions that the
// geometry never had.
func offsetKeyString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

// collapseOffsetCollisions folds every offset-collision warning in
// warnings into a single entry carrying the total repeat count and
// the first example key.
//
// One per chart, not one per cell. A composition encodes one scene
// per facet / repeat / concat cell and one mark set per layer, and
// each of those resolves its own offset binding off its own table —
// which is right, because a collision is a property of the rows that
// cell actually draws. But the author wrote one chart, and N
// identically-worded warnings for N cells is noise that trains them
// to skip the whole warning block. So the leaves report and the top
// of the tree collapses, the same split Encode / encodeLeaf already
// uses for the inert-field pass.
func collapseOffsetCollisions(warnings []scene.Warning) []scene.Warning {
	first := -1
	total := 0
	for i, w := range warnings {
		if w.Code != scene.WarnOffsetCollision {
			continue
		}
		if first < 0 {
			first = i
		}
		if n, ok := w.Details["count"].(int); ok {
			total += n
		}
	}
	if first < 0 || total == 0 {
		return warnings
	}
	merged := warnings[first]
	key, _ := merged.Details["key"].(string)
	details := make(map[string]any, len(merged.Details))
	for k, v := range merged.Details {
		details[k] = v
	}
	details["count"] = total
	merged.Details = details
	merged.Message = offsetCollisionMessage(total, key)

	out := make([]scene.Warning, 0, len(warnings))
	for i, w := range warnings {
		switch {
		case i == first:
			out = append(out, merged)
		case w.Code == scene.WarnOffsetCollision:
			// Already folded into merged.
		default:
			out = append(out, w)
		}
	}
	return out
}
