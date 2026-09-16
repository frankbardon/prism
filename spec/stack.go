package spec

// Stacking (E5-S2).
//
// Vega-Lite models stacking as an implicit transform rather than a
// mark option, and Prism follows: `StackBinding` is the single
// decision record that says whether a leaf spec stacks, on which
// position channel, with which grouping, and into which output
// columns. Both the planner (which injects the StackNode) and the
// encoder (which rebinds the position channels onto the node's output
// columns) call ResolveStack on the *same* spec, so the two stages can
// never disagree about whether stacking happened.
//
// The offsets that resolve today are "zero" and "normalize".
// "center" is accepted by the schema but intentionally resolves to no
// stacking until the streamgraph work lands — which is exactly the
// pre-E5-S2 behaviour for that value, so nothing regresses.

// Stack offset names. StackOffsetCenter is declared for completeness
// (the schema accepts it) but ResolveStack does not yet select it.
const (
	StackOffsetZero      = "zero"
	StackOffsetNormalize = "normalize"
	StackOffsetCenter    = "center"
)

// StackSuffixStart / StackSuffixEnd are the default output-column
// suffixes, matching Vega-Lite's `<field>_start` / `<field>_end`.
const (
	StackSuffixStart = "_start"
	StackSuffixEnd   = "_end"
)

// StackBinding describes one resolved stacking decision.
type StackBinding struct {
	// Channel is "x" or "y" — the position channel whose values
	// accumulate.
	Channel string
	// Field is the upstream column the accumulation reads. After the
	// synthetic encoding aggregate runs, an aggregated channel's
	// output column carries the same name as its input field, so this
	// is simply the channel's field name.
	Field string
	// Offset is StackOffsetZero or StackOffsetNormalize.
	Offset string
	// Groupby names the fields that delimit one stack — the opposite
	// position channel (the dimension axis).
	Groupby []string
	// StackBy names the discrete grouping fields that split a stack
	// into segments, in the same order encode/marks/group.go's
	// groupChannels walks them: colour first, then each detail field
	// in spec order. The stack node ranks segments by first
	// appearance of this tuple so every stack orders its segments
	// identically and in legend order.
	StackBy []string
	// StartAs / EndAs are the output column names.
	StartAs string
	EndAs   string
	// Implicit is true when the binding came from the bar/area
	// default rather than an explicit `stack` value on the channel.
	Implicit bool
}

// StackByFields returns the discrete grouping fields of enc in the
// order encode/marks/group.go partitions on: colour first (when
// bound), then each detail-channel field in spec order. Shared by the
// planner (to order stack segments) so grouping is derived once.
func StackByFields(enc *Encoding) []string {
	if enc == nil {
		return nil
	}
	var out []string
	if enc.Color != nil && enc.Color.Field != "" {
		out = append(out, enc.Color.Field)
	}
	for _, d := range DetailEntries(enc) {
		if d.Field == "" {
			continue
		}
		out = append(out, d.Field)
	}
	return out
}

// DetailEntries flattens encoding.detail — which decodes as either a
// single entry or an array — into a flat slice.
func DetailEntries(enc *Encoding) []DetailChannelEntry {
	if enc == nil || enc.Detail == nil {
		return nil
	}
	out := make([]DetailChannelEntry, 0, len(enc.Detail.Multi)+1)
	if enc.Detail.Single != nil {
		out = append(out, *enc.Detail.Single)
	}
	out = append(out, enc.Detail.Multi...)
	return out
}

// stackOffsetOf normalises a channel's raw `stack` value.
//
//	nil  + StackNull false → ("", false, false)  — absent, defaults apply
//	nil  + StackNull true  → ("", false, true)   — explicit null, disabled
//	false                  → ("", false, true)   — disabled
//	true                   → ("zero", true, false)
//	"zero" | "normalize"   → (value, true, false)
//	"center"               → ("", false, true)   — reserved, no stacking
//
// "center" reports disabled rather than absent on purpose: an author
// who asked for a streamgraph must not silently get a zero-offset
// stack from the implicit default instead. An unrecognised string is
// treated the same way; the JSON Schema enum is the gate that rejects
// it before it ever reaches here.
func stackOffsetOf(ch *PositionChannel) (offset string, explicit, disabled bool) {
	if ch == nil {
		return "", false, false
	}
	if ch.StackNull {
		return "", false, true
	}
	switch v := ch.Stack.(type) {
	case nil:
		return "", false, false
	case bool:
		if v {
			return StackOffsetZero, true, false
		}
		return "", false, true
	case string:
		switch v {
		case StackOffsetZero, StackOffsetNormalize:
			return v, true, false
		}
		return "", false, true
	}
	return "", false, false
}

// stackableMark reports whether markType draws stacked geometry. Bar
// renders each segment as a ranged rect; area renders each series as a
// ribbon between its start and end edges.
func stackableMark(markType string) bool {
	return markType == "bar" || markType == "area"
}

// ResolveStack decides whether s stacks, and how. Returns nil when it
// does not — which is every spec that predates E5-S2 except the
// bar/area + aggregate + grouping shape Vega-Lite also stacks.
//
// Explicit `stack` on x or y wins outright (subject to the structural
// guards below). Absent that, stacking is implicit when all of:
//
//   - the mark is bar or area;
//   - exactly one of x / y carries an aggregate and is quantitative;
//   - the other position channel is bound (it becomes the groupby);
//   - at least one discrete grouping channel (colour or detail) is
//     bound, so there is more than one segment to stack;
//   - neither span channel is bound on the stacked axis — an explicit
//     x2 / y2 interval already says where the mark starts and ends;
//   - the stacked channel declares no scale type other than linear.
func ResolveStack(s *Spec) *StackBinding {
	if s == nil || s.Encoding == nil || s.Mark == nil {
		return nil
	}
	if !stackableMark(s.Mark.TypeName()) {
		return nil
	}
	enc := s.Encoding

	xOffset, xExplicit, xDisabled := stackOffsetOf(enc.X)
	yOffset, yExplicit, yDisabled := stackOffsetOf(enc.Y)
	// Declaring stack on both axes is ambiguous; refuse rather than
	// pick one.
	if xExplicit && yExplicit {
		return nil
	}

	var (
		channel  string
		stackCh  *PositionChannel
		otherCh  *PositionChannel
		offset   string
		implicit bool
	)
	switch {
	case xExplicit:
		channel, stackCh, otherCh, offset = "x", enc.X, enc.Y, xOffset
	case yExplicit:
		channel, stackCh, otherCh, offset = "y", enc.Y, enc.X, yOffset
	default:
		if xDisabled || yDisabled {
			return nil
		}
		xAgg := enc.X != nil && enc.X.Field != "" && enc.X.Aggregate != ""
		yAgg := enc.Y != nil && enc.Y.Field != "" && enc.Y.Aggregate != ""
		if xAgg == yAgg {
			// Neither aggregated, or both — no unambiguous measure axis.
			return nil
		}
		if len(StackByFields(enc)) == 0 {
			return nil
		}
		if xAgg {
			channel, stackCh, otherCh, offset = "x", enc.X, enc.Y, StackOffsetZero
		} else {
			channel, stackCh, otherCh, offset = "y", enc.Y, enc.X, StackOffsetZero
		}
		implicit = true
	}

	if stackCh == nil || stackCh.Field == "" {
		return nil
	}
	if stackCh.Type != "" && stackCh.Type != "quantitative" {
		return nil
	}
	if otherCh == nil || otherCh.Field == "" {
		return nil
	}
	// An explicit span on either axis supersedes stacking: the mark
	// already knows both of its edges.
	if spanFieldBound(enc.X2) || spanFieldBound(enc.Y2) {
		return nil
	}
	// A non-linear measure scale cannot accumulate meaningfully.
	if stackCh.Scale != nil && stackCh.Scale.Type != "" && stackCh.Scale.Type != "linear" {
		return nil
	}

	return &StackBinding{
		Channel:  channel,
		Field:    stackCh.Field,
		Offset:   offset,
		Groupby:  []string{otherCh.Field},
		StackBy:  StackByFields(enc),
		StartAs:  stackCh.Field + StackSuffixStart,
		EndAs:    stackCh.Field + StackSuffixEnd,
		Implicit: implicit,
	}
}

// spanFieldBound reports whether a span channel carries a field name.
func spanFieldBound(ch *PositionChannel) bool {
	return ch != nil && ch.Field != ""
}
