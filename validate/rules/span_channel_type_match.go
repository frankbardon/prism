package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SpanChannelTypeMatch implements PRISM_SPEC_043: a span channel's
// `type` must match its base channel's.
//
// `x2` never resolves a scale of its own — the encoder measures it on
// the scale `x` resolved, which is the only way both ends of a span
// can land on one axis. Declaring `x` quantitative and `x2` temporal
// therefore does not produce two scales; it produces one scale
// silently reading the second column under the first column's rules.
// Rejecting the mismatch keeps the declared types honest about what
// the renderer does.
type SpanChannelTypeMatch struct{}

// Code returns PRISM_SPEC_043.
func (SpanChannelTypeMatch) Code() string { return "PRISM_SPEC_043" }

// Check compares the declared type of every bound span channel with
// its base channel's. Skips a pair where either type is absent — the
// missing type is PRISM_SPEC_042's or the shape validator's business.
func (SpanChannelTypeMatch) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, b := range walkSpanBindings(s) {
		if b.Base == nil || b.Span == nil {
			continue
		}
		if b.Base.Type == "" || b.Span.Type == "" || b.Base.Type == b.Span.Type {
			continue
		}
		out = append(out, errors.New("PRISM_SPEC_043",
			fmt.Sprintf("Encoding channel %q is %q but %q is %q; a span shares its base channel's scale, so both ends must declare the same type.",
				b.SpanName(), b.Span.Type, b.Name, b.Base.Type),
			map[string]any{
				"Channel":  b.SpanName(),
				"Type":     b.Span.Type,
				"Base":     b.Name,
				"BaseType": b.Base.Type,
				"Path":     b.Path,
			},
		))
	}
	return out
}
