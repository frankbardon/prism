package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// FilterPredicate implements PRISM_SPEC_037: the structured filter
// predicate (spec.Predicate, E2-S1) must be well-formed. It checks:
//
//   - referenced fields (field + to_field) exist in the source schema
//     or are produced by a transform,
//   - comparisons are type-compatible (numeric vs numeric, string vs
//     string) for both field-vs-literal and field-vs-field,
//   - `between` bounds are same-typed and lo <= hi,
//   - `one_of` / `not_one_of` sets are non-empty.
//
// Structural shape (exactly one branch, known op, required operands) is
// already enforced at decode time by spec.Predicate.UnmarshalJSON; this
// rule adds the schema-aware and cross-operand semantics.
type FilterPredicate struct{}

// Code returns PRISM_SPEC_037.
func (FilterPredicate) Code() string { return "PRISM_SPEC_037" }

// Check walks every filter transform's predicate tree.
func (FilterPredicate) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	_, schema, known := datasetForSpec(s, schemas)
	outputs := collectTransformOutputs(s.Transform, schemas)

	var out []*errors.AppError
	for i, t := range s.Transform {
		if t.Filter == nil {
			continue
		}
		site := fmt.Sprintf("transform[%d].filter", i)
		out = append(out, checkPredicate(&t.Filter.Filter, schema, known, outputs, site, "PRISM_SPEC_037")...)
	}
	return out
}

// checkPredicate recurses through the predicate tree, emitting one error
// (under the caller-supplied code) per problem found. The filter rule
// passes PRISM_SPEC_037; the calculate case-branch and condition-test
// rules reuse this same walk under their own codes so there is one
// predicate-semantics source of truth.
func checkPredicate(
	p *spec.Predicate,
	schema *validate.SchemaShim,
	known bool,
	outputs map[string]bool,
	site string,
	code string,
) []*errors.AppError {
	if p == nil {
		return nil
	}
	var out []*errors.AppError
	if p.IsCombinator() {
		for k := range p.And {
			out = append(out, checkPredicate(&p.And[k], schema, known, outputs, site, code)...)
		}
		for k := range p.Or {
			out = append(out, checkPredicate(&p.Or[k], schema, known, outputs, site, code)...)
		}
		out = append(out, checkPredicate(p.Not, schema, known, outputs, site, code)...)
		return out
	}

	// Field existence for the left operand (and to_field when present).
	leftType, leftResolved := fieldMeasureType(p.Field, schema, known, outputs)
	if known {
		if _, ok := fieldOrOutput(p.Field, schema, outputs); !ok {
			return append(out, predicateErr(code, site, fmt.Sprintf("field %q does not exist in the source schema", p.Field)))
		}
	}

	switch {
	case spec.IsComparison(p.Op):
		if p.ToField != "" {
			if known {
				if _, ok := fieldOrOutput(p.ToField, schema, outputs); !ok {
					out = append(out, predicateErr(code, site, fmt.Sprintf("to_field %q does not exist in the source schema", p.ToField)))
					break
				}
			}
			rightType, rightResolved := fieldMeasureType(p.ToField, schema, known, outputs)
			if leftResolved && rightResolved && !sameDomain(leftType, rightType) {
				out = append(out, predicateErr(code, site, fmt.Sprintf(
					"comparison between %q (%s) and %q (%s) mixes incompatible types",
					p.Field, leftType, p.ToField, rightType)))
			}
		} else if leftResolved && !literalMatchesDomain(p.Value, leftType) {
			out = append(out, predicateErr(code, site, fmt.Sprintf(
				"comparison on %q (%s) uses a value of the wrong type", p.Field, leftType)))
		}
	case p.Op == spec.PredOneOf || p.Op == spec.PredNotOneOf:
		if len(p.Values) == 0 {
			out = append(out, predicateErr(code, site, fmt.Sprintf("%q requires a non-empty values set", p.Op)))
			break
		}
		if leftResolved {
			for _, v := range p.Values {
				if !literalMatchesDomain(v, leftType) {
					out = append(out, predicateErr(code, site, fmt.Sprintf(
						"%q on %q (%s) has a value of the wrong type", p.Op, p.Field, leftType)))
					break
				}
			}
		}
	case p.Op == spec.PredBetween:
		if !sameLiteralDomain(p.Lo, p.Hi) {
			out = append(out, predicateErr(code, site, "between bounds lo and hi must be the same type"))
			break
		}
		if leftResolved && (!literalMatchesDomain(p.Lo, leftType) || !literalMatchesDomain(p.Hi, leftType)) {
			out = append(out, predicateErr(code, site, fmt.Sprintf(
				"between on %q (%s) has bounds of the wrong type", p.Field, leftType)))
			break
		}
		if cmp, ok := compareLiterals(p.Lo, p.Hi); ok && cmp > 0 {
			out = append(out, predicateErr(code, site, "between requires lo <= hi"))
		}
	}
	return out
}

// fieldOrOutput reports whether name exists in the schema or is produced
// by a transform.
func fieldOrOutput(name string, schema *validate.SchemaShim, outputs map[string]bool) (validate.FieldShim, bool) {
	if f, ok := schema.Field(name); ok {
		return f, true
	}
	if outputs[name] {
		return validate.FieldShim{Name: name}, true
	}
	return validate.FieldShim{}, false
}

// fieldMeasureType resolves a field's measure type. It reports
// resolved=false when the schema is unknown or the field is a transform
// output (type unknowable statically) — callers then skip type checks.
func fieldMeasureType(name string, schema *validate.SchemaShim, known bool, outputs map[string]bool) (string, bool) {
	if !known {
		return "", false
	}
	if f, ok := schema.Field(name); ok {
		return f.Type, true
	}
	// Transform output or unknown: skip typed checks.
	_ = outputs
	return "", false
}

// sameDomain reports whether two measure types share a comparison
// domain: numeric (quantitative/temporal) or string (nominal/ordinal).
func sameDomain(a, b string) bool {
	return isNumericType(a) == isNumericType(b)
}

func isNumericType(t string) bool {
	return t == "quantitative" || t == "temporal"
}

// literalMatchesDomain reports whether a JSON literal is compatible with
// a field's measure type. A null literal (nil) is treated as compatible
// (the value/type check does not apply to null operands).
func literalMatchesDomain(v any, measureType string) bool {
	if v == nil {
		return true
	}
	if isNumericType(measureType) {
		return isNumberLiteral(v)
	}
	// nominal / ordinal → string domain.
	_, isStr := v.(string)
	return isStr
}

func isNumberLiteral(v any) bool {
	switch v.(type) {
	case float64, float32, int, int32, int64:
		return true
	}
	return false
}

// sameLiteralDomain reports whether two literals share a domain (both
// numbers or both strings). Nil operands are permissive.
func sameLiteralDomain(a, b any) bool {
	if a == nil || b == nil {
		return true
	}
	if isNumberLiteral(a) && isNumberLiteral(b) {
		return true
	}
	_, aStr := a.(string)
	_, bStr := b.(string)
	return aStr && bStr
}

// compareLiterals compares two literals within a shared domain, returning
// (-1|0|1, true) or (0, false) when they are not comparable.
func compareLiterals(a, b any) (int, bool) {
	if af, aok := literalFloat(a); aok {
		if bf, bok := literalFloat(b); bok {
			switch {
			case af < bf:
				return -1, true
			case af > bf:
				return 1, true
			default:
				return 0, true
			}
		}
		return 0, false
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		switch {
		case as < bs:
			return -1, true
		case as > bs:
			return 1, true
		default:
			return 0, true
		}
	}
	return 0, false
}

func literalFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func predicateErr(code, site, reason string) *errors.AppError {
	noun := "Filter predicate"
	if code == "PRISM_SPEC_026" {
		noun = "Condition test predicate"
	}
	return errors.New(code,
		fmt.Sprintf("%s is not well-formed (%s) at %s.", noun, reason, site),
		map[string]any{
			"Reason": reason,
			"Site":   site,
		},
	)
}
