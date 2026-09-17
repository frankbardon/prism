package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// OffsetStackExclusive implements PRISM_SPEC_065: an explicit `stack`
// on a position channel may not sit beside a bound offset channel.
//
// A stack accumulates the segments of a grouping along the measure
// axis; a dodge spreads the same segments across the category band.
// Two mechanisms, one piece of geometry, one grouping — so a spec
// asking for both asks for a chart that does not exist.
//
// Implicit stacking already yields: spec.ResolveStack drops the
// bar/area default the moment an offset is bound, so the ordinary
// grouped-bar spec (which is character for character the shape that
// would otherwise stack) needs no `stack` key and never reaches this
// rule. An author only lands here by typing the key out, which is why
// the fixup says to remove the `stack` and keep the offset.
//
// The `stack` key is tri-state on the wire, and only one of the three
// states collides. An absent key is the implicit default and yields;
// `"stack": null` (recorded in PositionChannel.StackNull) and
// `"stack": false` both DISABLE stacking, so they agree with the
// offset rather than contradicting it and must stay silent.
type OffsetStackExclusive struct{}

// Code returns PRISM_SPEC_065.
func (OffsetStackExclusive) Code() string { return "PRISM_SPEC_065" }

// Check walks every leaf that binds an offset channel and looks for an
// explicit stacking request on either position channel.
func (OffsetStackExclusive) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, leaf := range walkOffsetLeaves(s) {
		bound := leaf.Bound()
		if len(bound) == 0 {
			continue
		}
		stacked := explicitlyStackedChannels(leaf.Enc)
		for _, off := range bound {
			for _, ch := range stacked {
				out = append(out, errors.New("PRISM_SPEC_065",
					fmt.Sprintf("Channel %q declares an explicit stack beside offset channel %q.", ch, off.Name),
					map[string]any{
						"Channel": ch,
						"Offset":  off.Name,
						"Path":    leaf.Path,
					},
				))
			}
		}
	}
	return out
}

// explicitlyStackedChannels names the position channels carrying an
// explicit stacking REQUEST, x first.
func explicitlyStackedChannels(enc *spec.Encoding) []string {
	if enc == nil {
		return nil
	}
	var out []string
	if explicitStackRequest(enc.X) {
		out = append(out, "x")
	}
	if explicitStackRequest(enc.Y) {
		out = append(out, "y")
	}
	return out
}

// explicitStackRequest reports whether ch asks for stacking outright.
//
// This mirrors the explicit half of spec/stack.go's stackOffsetOf,
// which is unexported. The states that must NOT report true:
//
//	absent key      → implicit default, which yields to the offset
//	"stack": null   → StackNull, stacking disabled
//	"stack": false  → stacking disabled
//	unknown string  → disabled (the JSON Schema enum rejects it first)
func explicitStackRequest(ch *spec.PositionChannel) bool {
	if ch == nil || ch.StackNull {
		return false
	}
	switch v := ch.Stack.(type) {
	case bool:
		return v
	case string:
		switch v {
		case spec.StackOffsetZero, spec.StackOffsetNormalize, spec.StackOffsetCenter:
			return true
		}
	}
	return false
}
