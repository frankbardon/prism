package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// BulletBands implements PRISM_SPEC_036: a bullet mark's qualitative
// bands are cumulative range bounds measured from zero, so they must be
// strictly ascending. A flat or descending pair would render an
// inverted / overlapping background range. Bands are optional; the rule
// only fires when two or more bounds are present and out of order.
type BulletBands struct{}

// Code returns PRISM_SPEC_036.
func (BulletBands) Code() string { return "PRISM_SPEC_036" }

// Check fires when mark is bullet and any band bound is not strictly
// greater than its predecessor. One error per offending pair.
func (BulletBands) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil || s.Mark.TypeName() != "bullet" {
		return nil
	}
	def := s.Mark.Def
	if def == nil || len(def.Bands) < 2 {
		return nil
	}
	var out []*errors.AppError
	for i := 1; i < len(def.Bands); i++ {
		if def.Bands[i] <= def.Bands[i-1] {
			out = append(out, errors.New("PRISM_SPEC_036",
				fmt.Sprintf("Bullet bands must be strictly ascending; bands[%d]=%v is not greater than bands[%d]=%v.",
					i, def.Bands[i], i-1, def.Bands[i-1]),
				map[string]any{"Index": i, "Value": def.Bands[i], "Previous": def.Bands[i-1]},
			))
		}
	}
	return out
}
