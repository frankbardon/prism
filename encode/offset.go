package encode

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/marks"
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
