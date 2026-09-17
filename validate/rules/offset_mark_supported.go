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
// sharing that category draw side by side. Only `bar` documents that
// geometry — but every band-seated mark reaches the same
// CategorySlots / rectAxisExtent helpers, and marks.Inputs.Offset is
// filled for every mark type, so an offset bound on any of them
// genuinely DODGES. A `tick` with `x_offset` over 4 rows / 2 series
// at the default width moves from x = 225/225/595/595 to
// 141.75/308.25/511.75/678.25.
//
// That makes this rule load-bearing rather than tidy-minded. Without
// it `rect`, `heatmap`, `boxplot`, `violin`, `winloss`, `progress`
// and the spark adornments would each quietly draw dodged geometry
// that no mark documents, and an author would have no way to tell
// whether what they were looking at was a feature.
//
// The encoder stays TOTAL on the question — it raises no error of its
// own and renders whatever the geometry comes out as — which is what
// makes this rule the only reporter: a second decision point at
// encode would give the two stages two chances to disagree.
// encode.TestPrismOffsetOnUnsupportedMarkStillEncodes pins that the
// encoder keeps producing a scene rather than failing.
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
