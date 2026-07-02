package rules

import (
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SelectionRef implements PRISM_SPEC_004: every reference to a named
// selection must resolve to a selection declared in the spec's
// "selection" block.
//
// The v1 "selection:<name>" shorthand lived inside free-form filter
// expression strings. As of E2-S1 filters carry a structured predicate
// (spec.Predicate) that has no selection-reference operand, so there is
// nothing to scan there. The rule stays registered for the condition-
// encoding reference path (deferred to v2); it is a no-op today.
type SelectionRef struct{}

// Code returns PRISM_SPEC_004.
func (SelectionRef) Code() string { return "PRISM_SPEC_004" }

// Check currently reports no errors: structured filter predicates carry
// no selection references, and condition-encoding references land in v2.
func (SelectionRef) Check(_ *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	return nil
}

func joinSortedKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
