package encode

import (
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Stacking, encoder side (E5-S2).
//
// The accumulation itself is a Plan node (plan/nodes/stack.go): by the
// time the encoder runs, the tip table already carries the two derived
// bounds columns. All the encoder has to do is point the position
// channels at them — the stacked channel reads the upper bound and its
// span companion (x2 / y2) reads the lower one.
//
// Rebinding *before* scale resolution is the whole reason stacking is
// a Plan node rather than an encode-local pre-pass: the measure axis
// then derives its domain from the accumulated column (0…sum) through
// the ordinary resolveChannel / spanDomainValues path, so nothing
// about scale resolution, axis building or shared-domain unification
// needs to learn that stacking exists.
//
// Both the planner and the encoder call spec.ResolveStack on the same
// spec, so the two stages cannot disagree about whether a spec stacks.
// The column-presence check below is a pure safety net for callers
// that hand Encode a table built by something other than the plan the
// spec describes.

// rebindStack returns s with its stacked position channel pointed at
// the StackNode's bounds columns, or s unchanged when the spec does
// not stack. The input spec is never mutated — the encoding and the
// two affected channels are copied.
func rebindStack(s *spec.Spec, tbl *table.Table) *spec.Spec {
	st := spec.ResolveStack(s)
	if st == nil || tbl == nil || s.Encoding == nil {
		return s
	}
	if _, ok := tbl.Column(st.StartAs); !ok {
		return s
	}
	if _, ok := tbl.Column(st.EndAs); !ok {
		return s
	}

	enc := *s.Encoding
	var base *spec.PositionChannel
	if st.Channel == "x" {
		base = enc.X
	} else {
		base = enc.Y
	}
	if base == nil {
		return s
	}

	upper := *base
	upper.Field = st.EndAs
	upper.FieldRef = nil
	// The aggregate already ran upstream; the bounds columns are plain
	// quantities. Leaving the alias on would misdescribe the channel to
	// anything that reads it downstream.
	upper.Aggregate = ""
	upper.Type = "quantitative"
	upper.Axis = stackAxis(base, st)

	lower := spec.PositionChannel{}
	lower.Field = st.StartAs
	lower.Type = "quantitative"

	if st.Channel == "x" {
		enc.X, enc.X2 = &upper, &lower
	} else {
		enc.Y, enc.Y2 = &upper, &lower
	}

	out := *s
	out.Encoding = &enc
	return &out
}

// stackAxis returns the axis block the rebound channel should carry.
//
// One default has to survive the rebinding: the axis title, which
// axisOptsFor derives from the channel's own field name and which
// would therefore read "<field>_end". An axis block the spec wrote
// wins; a hidden axis (AxisHidden) is unaffected either way, since
// suppression is a separate flag.
//
// A normalized stack's measure axis is left on its natural 0…1 domain.
// Formatting it as a percentage would need `axis.format`, which the
// tick labeller still runs through fmt.Sprintf rather than the
// d3-format subset the rest of the encoder uses — see encode/ticks.go.
//
// A centred stack's axis is left signed for the same reason: every
// stack spans [-h/2, +h/2] about the shared baseline, so the resolved
// domain is symmetric and a tick honestly reads as distance from the
// midline. Authors who want the numbers gone suppress the axis
// outright with `"axis": null`.
func stackAxis(base *spec.PositionChannel, st *spec.StackBinding) *spec.Axis {
	var ax spec.Axis
	if base.Axis != nil {
		ax = *base.Axis
	}
	if !axisTitleExplicit(base) {
		ax.Title = st.Field
	}
	return &ax
}
