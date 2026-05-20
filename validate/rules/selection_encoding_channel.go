package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SelectionEncodingChannel implements PRISM_SPEC_019: every channel
// listed in a selection's `encodings` array must be bound in the
// spec's encoding block.
//
// Both point + interval selections honour the rule. The channel name
// is case-sensitive (lower-case per the JSON schema enum).
type SelectionEncodingChannel struct{}

// Code returns PRISM_SPEC_019.
func (SelectionEncodingChannel) Code() string { return "PRISM_SPEC_019" }

// Check walks every selection's Encodings list and confirms each entry
// resolves to a bound channel in s.Encoding.
func (SelectionEncodingChannel) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || len(s.Selection) == 0 {
		return nil
	}
	bound := boundChannels(s.Encoding)
	availableStr := strings.Join(bound, ", ")
	available := map[string]bool{}
	for _, c := range bound {
		available[c] = true
	}

	var out []*errors.AppError
	for selName, sel := range s.Selection {
		var encs []string
		switch {
		case sel.Point != nil:
			encs = sel.Point.Encodings
		case sel.Interval != nil:
			encs = sel.Interval.Encodings
		}
		for _, c := range encs {
			if available[c] {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_019",
				fmt.Sprintf("Selection %q encoding %q is not bound in the spec encoding block (available: %s).",
					selName, c, availableStr),
				map[string]any{
					"Selection": selName,
					"Channel":   c,
					"Available": availableStr,
				},
			))
		}
	}
	// Deterministic ordering for golden / negative-fixture tests.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Context["Selection"].(string)+":"+out[i].Context["Channel"].(string) <
			out[j].Context["Selection"].(string)+":"+out[j].Context["Channel"].(string)
	})
	return out
}

// boundChannels returns the sorted list of channels with a non-empty
// field binding on enc. Used to compose the "available" diagnostic.
func boundChannels(enc *spec.Encoding) []string {
	if enc == nil {
		return nil
	}
	var out []string
	if enc.X != nil && enc.X.Field != "" {
		out = append(out, "x")
	}
	if enc.Y != nil && enc.Y.Field != "" {
		out = append(out, "y")
	}
	if enc.X2 != nil && enc.X2.Field != "" {
		out = append(out, "x2")
	}
	if enc.Y2 != nil && enc.Y2.Field != "" {
		out = append(out, "y2")
	}
	if enc.Theta != nil && enc.Theta.Field != "" {
		out = append(out, "theta")
	}
	if enc.Radius != nil && enc.Radius.Field != "" {
		out = append(out, "radius")
	}
	if enc.Color != nil && enc.Color.Field != "" {
		out = append(out, "color")
	}
	if enc.Fill != nil && enc.Fill.Field != "" {
		out = append(out, "fill")
	}
	if enc.Stroke != nil && enc.Stroke.Field != "" {
		out = append(out, "stroke")
	}
	if enc.Opacity != nil && enc.Opacity.Field != "" {
		out = append(out, "opacity")
	}
	if enc.Size != nil && enc.Size.Field != "" {
		out = append(out, "size")
	}
	if enc.Shape != nil && enc.Shape.Field != "" {
		out = append(out, "shape")
	}
	sort.Strings(out)
	return out
}
