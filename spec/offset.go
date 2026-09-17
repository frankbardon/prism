package spec

// Offset position channels (E1-S2).
//
// `encoding.x_offset` / `encoding.y_offset` subdivide a band slot so
// rows sharing one category value render side by side instead of on
// top of one another — the grouped (dodged) bar primitive Vega-Lite
// spells xOffset / yOffset.
//
// ResolveOffset is the SINGLE place the question "is an offset bound,
// on which axis, reading which field" is answered. The planner (which
// must keep the offset field alive through the synthetic encoding
// aggregate's group-by) and the encoder (which builds the sub-band
// scale and dodges the geometry) both call it on the *same* spec, so
// the two stages cannot reach different verdicts and no plan → encode
// side channel is needed. That is the contract spec/stack.go's
// ResolveStack and spec/order.go's ResolveOrder already establish; the
// offset channel follows it rather than growing a second answer.
//
// No caller may re-derive the question from `enc.XOffset != nil`. The
// nil-pointer test is not equivalent: an offset object carrying no
// field binds nothing, and an offset on both axes is incoherent (see
// ResolveOffset's doc comment).
//
// Returning nil is the signal every caller keys on: no group-by
// widening, no sub-band scale, geometry unchanged. An unbound offset
// is therefore a byte-identical no-op for every spec that predates
// this channel.

// OffsetBinding describes one resolved offset decision.
type OffsetBinding struct {
	// Channel is "x" or "y" — the position channel whose band slot
	// the offset subdivides. Named to match StackBinding.Channel.
	Channel string
	// Field is the column whose distinct values become the sub-band
	// categories.
	Field string
	// Offset is the source channel block, so a caller reads `type`,
	// `sort` and `scale` from the one place they are declared rather
	// than from a copy in this struct that could drift out of step
	// with the spec.
	Offset *OffsetChannel
}

// offsetBound reports whether an offset channel actually binds
// something. An absent channel and a channel carrying no field are
// the same thing to every consumer: there is no column to subdivide
// the band by.
func offsetBound(ch *OffsetChannel) bool {
	return ch != nil && ch.Field != ""
}

// ResolveOffset returns the offset binding enc declares, or nil when
// none is in force.
//
// Nil is returned in three cases:
//
//   - no offset channel is present;
//   - the offset channel is present but carries no field, so there is
//     nothing to subdivide the band by;
//   - BOTH x_offset and y_offset are bound.
//
// The last case is an incoherent spec — a mark cannot dodge along two
// axes at once — and validate rejects it with PRISM_SPEC_064. This
// resolver must stay total and must not panic, so it refuses rather
// than picking a winner. Refusing is the answer ResolveStack already
// gives the structurally identical ambiguity (`stack` declared on both
// x and y), and it is the safer of the two options: preferring one
// axis would render a chart that silently honours half of what the
// author wrote, whereas returning nil leaves the mark undodged and
// lets the validate error be the only thing the author has to read.
func ResolveOffset(enc *Encoding) *OffsetBinding {
	if enc == nil {
		return nil
	}
	x, y := offsetBound(enc.XOffset), offsetBound(enc.YOffset)
	switch {
	case x && y:
		return nil
	case x:
		return &OffsetBinding{Channel: "x", Field: enc.XOffset.Field, Offset: enc.XOffset}
	case y:
		return &OffsetBinding{Channel: "y", Field: enc.YOffset.Field, Offset: enc.YOffset}
	}
	return nil
}
