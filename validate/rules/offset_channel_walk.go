package rules

import (
	"sort"

	"github.com/frankbardon/prism/spec"
)

// Shared walk + capability table for the offset position channels
// (E2-S2). Four rules read it — PRISM_SPEC_063 / 064 / 065 / 066 —
// and none of them owns it, exactly as span_channel_walk.go serves
// PRISM_SPEC_042 and PRISM_SPEC_043.
//
// The offset channels are the only place in the spec where a binding
// can be structurally impossible in four different ways at once (wrong
// mark, wrong axis, stacked as well, ranged as well), so collecting
// the leaves once and judging them separately keeps each rule a single
// readable question.

// offsetLeaf is one spec node in the tree whose encoding mentions an
// offset channel. Path is a dotted slug naming the node ("" for the
// root, "layer[1]", "concat[0].layer[2]", "spec") so an error can
// point at the right leaf; Mark is the resolved mark type, empty when
// the node declares none.
type offsetLeaf struct {
	Path string
	Mark string
	Enc  *spec.Encoding
}

// offsetAxis is one bound offset channel on a leaf.
type offsetAxis struct {
	// Axis is "x" or "y" — the position channel whose band slot the
	// offset subdivides.
	Axis string
	// Name is the channel's own name, "x_offset" or "y_offset".
	Name string
	// Offset is the declared channel block.
	Offset *spec.OffsetChannel
}

// offsetChannelBound reports whether an offset channel actually binds
// something. An absent channel and a channel carrying no field are the
// same thing to every consumer — there is no column to subdivide the
// band by — which is the test spec.ResolveOffset applies internally.
//
// The raw enc.XOffset / enc.YOffset pointers are read here rather than
// through spec.ResolveOffset on purpose: that resolver returns nil in
// exactly the both-axes-bound case (it refuses the ambiguity instead of
// picking a winner), so the rule that has to REPORT that case cannot
// ask it.
func offsetChannelBound(ch *spec.OffsetChannel) bool {
	return ch != nil && ch.Field != ""
}

// Bound lists the offset channels this leaf actually binds, x first.
func (l offsetLeaf) Bound() []offsetAxis {
	if l.Enc == nil {
		return nil
	}
	var out []offsetAxis
	if offsetChannelBound(l.Enc.XOffset) {
		out = append(out, offsetAxis{Axis: "x", Name: "x_offset", Offset: l.Enc.XOffset})
	}
	if offsetChannelBound(l.Enc.YOffset) {
		out = append(out, offsetAxis{Axis: "y", Name: "y_offset", Offset: l.Enc.YOffset})
	}
	return out
}

// Position returns the position channel whose band slot an offset on
// axis subdivides.
func (l offsetLeaf) Position(axis string) *spec.PositionChannel {
	if l.Enc == nil {
		return nil
	}
	if axis == "y" {
		return l.Enc.Y
	}
	return l.Enc.X
}

// Span returns the span channel sharing axis with an offset — x2 for
// x, y2 for y. The OTHER axis is never consulted: a y2 beside an
// x_offset is a ranged, dodged bar and is legal.
func (l offsetLeaf) Span(axis string) *spec.PositionChannel {
	if l.Enc == nil {
		return nil
	}
	if axis == "y" {
		return l.Enc.Y2
	}
	return l.Enc.X2
}

// walkOffsetLeaves collects every node in the spec tree whose encoding
// mentions x_offset or y_offset, including layer / concat / hconcat /
// vconcat / facet / repeat children. The walk recurses, so an offset
// bound on a layer nested inside a concat cell is still found.
//
// A spec that mentions neither channel yields nothing and none of the
// four offset rules fires, which is what keeps every pre-E1 spec
// reported exactly as before.
func walkOffsetLeaves(s *spec.Spec) []offsetLeaf {
	var out []offsetLeaf
	var visit func(prefix string, sub *spec.Spec)
	visit = func(prefix string, sub *spec.Spec) {
		if sub == nil {
			return
		}
		if enc := sub.Encoding; enc != nil && (enc.XOffset != nil || enc.YOffset != nil) {
			mark := ""
			if sub.Mark != nil {
				mark = sub.Mark.TypeName()
			}
			out = append(out, offsetLeaf{Path: prefix, Mark: mark, Enc: enc})
		}
		for i, l := range sub.Layer {
			visit(offsetPath(prefix, prefixf("layer[%d]", i)), l)
		}
		for i, c := range sub.Concat {
			visit(offsetPath(prefix, prefixf("concat[%d]", i)), c)
		}
		for i, c := range sub.HConcat {
			visit(offsetPath(prefix, prefixf("hconcat[%d]", i)), c)
		}
		for i, c := range sub.VConcat {
			visit(offsetPath(prefix, prefixf("vconcat[%d]", i)), c)
		}
		visit(offsetPath(prefix, "spec"), sub.ChildSpec)
	}
	visit("", s)
	return out
}

// offsetPath joins a parent slug with a child slug.
func offsetPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// offsetCapableMarks maps a mark type to the offset channels it can
// actually dodge along. A mark absent from the map draws none.
//
// Only `bar` implements the geometry: encode/marks/bar.go hands the
// resolved sub-band scale to rectAxisExtent, which cuts the category's
// slot into one rect per offset value. Every other band-seated mark
// fills its slot with a single shape — a `tick` is one line, a
// `heatmap` cell one rect, a `boxplot` one summary of the whole
// category — so there is nothing to divide.
//
// Those marks still reach CategorySlots, so without this table an
// offset bound on one of them would silently change nothing (rect,
// tick, boxplot, violin, heatmap, winloss, progress and the spark
// adornments all route through it). Rejecting is the same answer
// spanCapableMarks and orientAwareMarks give the same question.
var offsetCapableMarks = map[string][]string{
	"bar": {"x_offset", "y_offset"},
}

// markDrawsOffset reports whether mark dodges along the named offset
// channel.
func markDrawsOffset(mark, offset string) bool {
	for _, c := range offsetCapableMarks[mark] {
		if c == offset {
			return true
		}
	}
	return false
}

// offsetCapableMarkList returns the mark types that dodge along the
// named offset channel, in a stable order for error text.
func offsetCapableMarkList(offset string) []string {
	var out []string
	for m := range offsetCapableMarks {
		if markDrawsOffset(m, offset) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}
