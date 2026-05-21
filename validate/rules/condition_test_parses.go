package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ConditionTestParses implements PRISM_SPEC_026: every `test`
// expression in a condition entry must parse via the Pulse expression
// parser (mirroring PRISM_SPEC_006 for transforms).
type ConditionTestParses struct{}

// Code returns PRISM_SPEC_026.
func (ConditionTestParses) Code() string { return "PRISM_SPEC_026" }

// Check walks every condition-bearing channel and tries to parse each
// non-empty test expression. Reuses the parser shim from
// ExpressionParses so the syntax surface stays in sync with Pulse.
func (ConditionTestParses) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	for _, cc := range walkConditionsTree(s) {
		for i, entry := range cc.Cond.Entries() {
			if entry.Test == "" {
				continue
			}
			if err := tryParse(entry.Test); err != nil {
				out = append(out, errors.New("PRISM_SPEC_026",
					fmt.Sprintf("Condition on channel %s entry[%d]: test expression failed to parse: %v.",
						cc.Path, i, err),
					map[string]any{
						"Channel":    cc.Path,
						"Entry":      i,
						"Expression": entry.Test,
						"Reason":     err.Error(),
					},
				))
			}
		}
	}
	return out
}
