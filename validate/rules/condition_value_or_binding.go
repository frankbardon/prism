package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ConditionValueOrBinding implements PRISM_SPEC_027: a condition entry
// must carry exactly one of {value, field}. Both set or neither set
// fails — except that a selection-form entry with neither value nor
// field is allowed: it implicitly inherits the channel's own field
// binding. test-form entries always require value or field.
type ConditionValueOrBinding struct{}

// Code returns PRISM_SPEC_027.
func (ConditionValueOrBinding) Code() string { return "PRISM_SPEC_027" }

// Check walks every condition-bearing channel and flags entries with
// invalid value/field combinations.
func (ConditionValueOrBinding) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	for _, cc := range walkConditionsTree(s) {
		for i, entry := range cc.Cond.Entries() {
			hasValue := entry.Value != nil
			hasField := entry.Field != ""
			isSelection := entry.Selection != ""

			if hasValue && hasField {
				out = append(out, conditionValueOrBindingErr(cc.Path, i, "both value and field set"))
				continue
			}
			if !hasValue && !hasField {
				// Selection entries may inherit the channel's field;
				// test entries must specify one.
				if !isSelection {
					out = append(out, conditionValueOrBindingErr(cc.Path, i, "neither value nor field set"))
				}
				continue
			}
		}
	}
	return out
}

func conditionValueOrBindingErr(path string, i int, got string) *errors.AppError {
	return errors.New("PRISM_SPEC_027",
		fmt.Sprintf("Condition entry on channel %s entry[%d] must carry exactly one of value or field (got: %s).",
			path, i, got),
		map[string]any{
			"Channel": path,
			"Entry":   i,
			"Got":     got,
		},
	)
}
