package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// CalculateExpr implements PRISM_SPEC_038: the structured calculate
// expression (spec.CalcExpr, E2-S2) must be well-formed. It checks:
//
//   - referenced operand fields (and every `case` branch's `when`
//     predicate) exist in the source schema or are produced by an
//     earlier transform,
//   - a `div` / `mod` by a literal zero is rejected (a runtime zero
//     divisor is a null-producing policy, not an error),
//   - `as` is non-empty and does not shadow an existing source column.
//
// Structural shape (one populated form, known op/fn, operand arity,
// mandatory `else`) is already enforced at decode time by
// spec.CalcExpr.UnmarshalJSON; this rule adds the schema-aware and
// cross-operand semantics. Case-branch `when` predicates are validated
// by the shared checkPredicate helper (filter_predicate.go), so no
// second predicate grammar is forked.
type CalculateExpr struct{}

// Code returns PRISM_SPEC_038.
func (CalculateExpr) Code() string { return "PRISM_SPEC_038" }

// Check walks every calculate transform's expression tree.
func (CalculateExpr) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	_, schema, known := datasetForSpec(s, schemas)
	outputs := collectTransformOutputs(s.Transform)

	var out []*errors.AppError
	for i, t := range s.Transform {
		if t.Calculate == nil {
			continue
		}
		site := fmt.Sprintf("transform[%d].calculate", i)
		if t.Calculate.As == "" {
			out = append(out, calcErr(site, "`as` must be a non-empty output column name"))
		} else if known {
			if _, ok := schema.Field(t.Calculate.As); ok {
				out = append(out, calcErr(site, fmt.Sprintf("`as` name %q shadows an existing source column", t.Calculate.As)))
			}
		}
		out = append(out, checkCalcExpr(&t.Calculate.Calculate, schema, known, outputs, site)...)
	}
	return out
}

// checkCalcExpr recurses through the expression tree, emitting one
// PRISM_SPEC_038 per problem found.
func checkCalcExpr(
	e *spec.CalcExpr,
	schema *validate.PulseSchemaShim,
	known bool,
	outputs map[string]bool,
	site string,
) []*errors.AppError {
	if e == nil {
		return nil
	}
	var out []*errors.AppError
	switch e.Form() {
	case "field":
		if known {
			if _, ok := fieldOrOutput(e.Field, schema, outputs); !ok {
				out = append(out, calcErr(site, fmt.Sprintf("field %q does not exist in the source schema", e.Field)))
			}
		}
	case "literal":
		// Scalar type already enforced at decode.
	case "op":
		if (e.Op == spec.CalcDiv || e.Op == spec.CalcMod) && len(e.Operands) == 2 && isLiteralZero(&e.Operands[1]) {
			out = append(out, calcErr(site, fmt.Sprintf("`%s` by a literal zero is undefined", e.Op)))
		}
		for k := range e.Operands {
			out = append(out, checkCalcExpr(&e.Operands[k], schema, known, outputs, site)...)
		}
	case "fn":
		for k := range e.Args {
			out = append(out, checkCalcExpr(&e.Args[k], schema, known, outputs, site)...)
		}
	case "concat":
		for k := range e.Concat {
			out = append(out, checkCalcExpr(&e.Concat[k], schema, known, outputs, site)...)
		}
	case "case":
		for k := range e.Case {
			out = append(out, checkPredicate(&e.Case[k].When, schema, known, outputs, site+".when", "PRISM_SPEC_037")...)
			out = append(out, checkCalcExpr(&e.Case[k].Then, schema, known, outputs, site)...)
		}
		out = append(out, checkCalcExpr(e.Else, schema, known, outputs, site)...)
	}
	return out
}

// isLiteralZero reports whether e is a literal node whose value is a
// numeric zero.
func isLiteralZero(e *spec.CalcExpr) bool {
	if e.Form() != "literal" {
		return false
	}
	f, ok := literalFloat(e.Literal)
	return ok && f == 0
}

func calcErr(site, reason string) *errors.AppError {
	return errors.New("PRISM_SPEC_038",
		fmt.Sprintf("Calculate expression is not well-formed (%s) at %s.", reason, site),
		map[string]any{
			"Reason": reason,
			"Site":   site,
		},
	)
}
