package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SpanChannelSupported implements PRISM_SPEC_042: a bound `x2` / `y2`
// must be something the mark can actually draw.
//
// Before E9-S3 no geometry read either channel, so every span binding
// was silently discarded — a ranged bar rendered as an ordinary
// baseline bar with no diagnostic. Now bar, rect and rule range on
// both axes and area takes y2 as its lower edge; on every other mark
// a span channel has no geometry to land in, and this rule says so
// instead of dropping it.
//
// It also catches the two structural mistakes that survive on a
// span-capable mark: a span channel with no base channel to extend
// (`x2` without `x`), and a span channel that names no field at all.
type SpanChannelSupported struct{}

// Code returns PRISM_SPEC_042.
func (SpanChannelSupported) Code() string { return "PRISM_SPEC_042" }

// Check walks every span binding in the spec tree. Emits at most one
// error per bound span channel.
func (SpanChannelSupported) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, b := range walkSpanBindings(s) {
		name := b.SpanName()
		details := map[string]any{"Channel": name, "Mark": b.Mark, "Path": b.Path}
		switch {
		case b.Mark == "":
			// Mark type is resolved elsewhere (PRISM_SPEC_003 and the
			// shape validator); without one there is nothing to judge
			// the span channel against.
			continue
		case !markDrawsSpan(b.Mark, name):
			details["Allowed"] = strings.Join(spanCapableMarkList(name), ", ")
			out = append(out, errors.New("PRISM_SPEC_042",
				fmt.Sprintf("Encoding channel %q draws no geometry on mark type %q.", name, b.Mark),
				details,
			))
		case !channelBound(b.Base):
			out = append(out, errors.New("PRISM_SPEC_042",
				fmt.Sprintf("Encoding channel %q extends %q, which is not bound.", name, b.Name),
				details,
			))
		case !channelBound(b.Span):
			out = append(out, errors.New("PRISM_SPEC_042",
				fmt.Sprintf("Encoding channel %q must bind a field to draw a span.", name),
				details,
			))
		}
	}
	return out
}
