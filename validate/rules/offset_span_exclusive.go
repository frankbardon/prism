package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// OffsetSpanExclusive implements PRISM_SPEC_066: an offset channel and
// a span channel may not claim the same axis.
//
// A span (`x2` / `y2`) states both ends of the mark along its own
// axis, which leaves the band slot on that axis with nothing to
// subdivide. encode/marks/span.go's rectAxisExtent takes its span
// branch first and never consults the sub-band scale, so the offset is
// dropped and the mark drawn across the whole slot — a live silent
// no-op until this rule names it.
//
// The clash is per axis, and only per axis. `y2` together with
// `x_offset` is a ranged AND dodged bar: the y extent comes from the
// pair of measure columns while the x slot is still cut into
// sub-bands. That chart is well defined and must stay legal, so the
// rule never looks at the opposite axis.
type OffsetSpanExclusive struct{}

// Code returns PRISM_SPEC_066.
func (OffsetSpanExclusive) Code() string { return "PRISM_SPEC_066" }

// Check walks every bound offset channel and rejects a span bound on
// the same axis.
func (OffsetSpanExclusive) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, leaf := range walkOffsetLeaves(s) {
		for _, off := range leaf.Bound() {
			span := leaf.Span(off.Axis)
			if !channelBound(span) {
				continue
			}
			spanName := off.Axis + "2"
			out = append(out, errors.New("PRISM_SPEC_066",
				fmt.Sprintf("Offset channel %q and span channel %q are bound on the same axis (%s).",
					off.Name, spanName, off.Axis),
				map[string]any{
					"Offset": off.Name,
					"Span":   spanName,
					"Axis":   off.Axis,
					"Path":   leaf.Path,
				},
			))
		}
	}
	return out
}
