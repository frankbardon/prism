package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ConditionSelectionRef implements PRISM_SPEC_025: every `selection`
// name referenced from a condition entry must resolve to a declared
// selection in the spec's "selection" block.
type ConditionSelectionRef struct{}

// Code returns PRISM_SPEC_025.
func (ConditionSelectionRef) Code() string { return "PRISM_SPEC_025" }

// Check walks every condition-bearing channel and reports unresolved
// selection references.
func (ConditionSelectionRef) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	declared := collectSelectionNames(s)
	var out []*errors.AppError
	for _, cc := range walkConditionsTree(s) {
		for i, entry := range cc.Cond.Entries() {
			if entry.Selection == "" {
				continue
			}
			if declared[entry.Selection] {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_025",
				fmt.Sprintf("Condition on channel %s references selection %q which is not declared.",
					cc.Path, entry.Selection),
				map[string]any{
					"Channel":   cc.Path,
					"Selection": entry.Selection,
					"Available": joinSortedKeys(declared),
					"Entry":     i,
				},
			))
		}
	}
	return out
}

func collectSelectionNames(s *spec.Spec) map[string]bool {
	declared := map[string]bool{}
	walk := func(node *spec.Spec) {
		if node == nil {
			return
		}
		for name := range node.Selection {
			declared[name] = true
		}
	}
	walk(s)
	for _, l := range s.Layer {
		walk(l)
	}
	for _, c := range s.Concat {
		walk(c)
	}
	for _, c := range s.HConcat {
		walk(c)
	}
	for _, c := range s.VConcat {
		walk(c)
	}
	if s.ChildSpec != nil {
		walk(s.ChildSpec)
	}
	return declared
}
