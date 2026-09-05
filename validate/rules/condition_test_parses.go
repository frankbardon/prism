package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ConditionTestParses implements PRISM_SPEC_026: every `test` predicate
// in a condition entry must be well-formed. Since E2-S3 a condition
// `test` is a structured spec.Predicate (the same grammar `filter`
// uses), not a free-form expression string — so structural shape
// (exactly one branch, known op, required operands, string form
// rejected) is already enforced at decode time by
// spec.Predicate.UnmarshalJSON.
//
// This rule adds the schema-aware and cross-operand semantics: field
// existence and type compatibility. It reuses the shared checkPredicate
// helper (filter_predicate.go) under code PRISM_SPEC_026 so the
// predicate semantics stay a single source of truth across filter,
// calculate case-branches, and conditions.
type ConditionTestParses struct{}

// Code returns PRISM_SPEC_026.
func (ConditionTestParses) Code() string { return "PRISM_SPEC_026" }

// Check walks every condition-bearing channel across the spec tree and
// validates each non-empty test predicate against the owning node's
// dataset schema.
func (ConditionTestParses) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	var visit func(prefix string, sub *spec.Spec)
	visit = func(prefix string, sub *spec.Spec) {
		if sub == nil {
			return
		}
		_, schema, known := datasetForSpec(sub, schemas)
		outputs := collectTransformOutputs(sub.Transform, schemas)
		for _, cc := range channelConditionsAt(prefix, sub) {
			for i, entry := range cc.Cond.Entries() {
				if entry.Test == nil {
					continue
				}
				site := fmt.Sprintf("%s.condition[%d].test", cc.Path, i)
				out = append(out, checkPredicate(entry.Test, schema, known, outputs, site, "PRISM_SPEC_026")...)
			}
		}
		for i, l := range sub.Layer {
			visit(prefixf("layer[%d]", i), l)
		}
		for i, c := range sub.Concat {
			visit(prefixf("concat[%d]", i), c)
		}
		for i, c := range sub.HConcat {
			visit(prefixf("hconcat[%d]", i), c)
		}
		for i, c := range sub.VConcat {
			visit(prefixf("vconcat[%d]", i), c)
		}
		visit("spec", sub.ChildSpec)
	}
	visit("", s)
	return out
}
