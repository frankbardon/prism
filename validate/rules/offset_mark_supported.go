package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// OffsetMarkSupported implements PRISM_SPEC_063: a bound `x_offset` /
// `y_offset` must sit on a mark that can actually dodge.
//
// The offset channels subdivide one category's band slot so the rows
// sharing that category draw side by side. Only `bar` implements that
// geometry. Every other band-seated mark draws one shape per category
// and reaches the same CategorySlots helper, so an offset there would
// be accepted, decoded, and then change nothing — the silent-no-op
// class this repo has already shipped three times.
//
// The encoder is intentionally total on this question (it yields the
// zero binding and draws the undodged mark), which is what makes this
// rule the only reporter: a second decision point at encode would give
// the two stages two chances to disagree.
type OffsetMarkSupported struct{}

// Code returns PRISM_SPEC_063.
func (OffsetMarkSupported) Code() string { return "PRISM_SPEC_063" }

// Check walks every bound offset channel in the spec tree. Emits at
// most one error per bound channel.
func (OffsetMarkSupported) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, leaf := range walkOffsetLeaves(s) {
		if leaf.Mark == "" {
			// Mark resolution is PRISM_SPEC_003's and the shape
			// validator's business; without one there is nothing to
			// judge the offset against.
			continue
		}
		for _, off := range leaf.Bound() {
			if markDrawsOffset(leaf.Mark, off.Name) {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_063",
				fmt.Sprintf("Encoding channel %q cannot be dodged by mark type %q.", off.Name, leaf.Mark),
				map[string]any{
					"Channel": off.Name,
					"Mark":    leaf.Mark,
					"Path":    leaf.Path,
					"Allowed": strings.Join(offsetCapableMarkList(off.Name), ", "),
				},
			))
		}
	}
	return out
}
