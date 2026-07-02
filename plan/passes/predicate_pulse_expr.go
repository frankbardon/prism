package passes

import (
	"strconv"
	"strings"

	"github.com/frankbardon/prism/spec"
)

// predicateToPulseExpr serializes a structured spec.Predicate into the
// Pulse FILTER_EXPRESSION string consumed by PulseChainFusionPass. It
// exists only to keep the Pulse-chain fusion optimization working now
// that filters carry a structured predicate instead of an expression
// string (E2-S1); it is removed with the Pulse backend.
//
// Returns ok=false for any node the grammar can produce but the
// serializer chooses not to represent, so the caller treats the filter
// as a fusion boundary and lets the in-memory evaluator handle it.
func predicateToPulseExpr(p spec.Predicate) (string, bool) {
	return renderPulseExpr(&p)
}

func renderPulseExpr(p *spec.Predicate) (string, bool) {
	if p == nil {
		return "", false
	}
	switch {
	case p.And != nil:
		return joinPulseExpr(p.And, "and")
	case p.Or != nil:
		return joinPulseExpr(p.Or, "or")
	case p.Not != nil:
		inner, ok := renderPulseExpr(p.Not)
		if !ok {
			return "", false
		}
		return "not (" + inner + ")", true
	}

	switch p.Op {
	case spec.PredEq, spec.PredNe, spec.PredLt, spec.PredLte, spec.PredGt, spec.PredGte:
		rhs, ok := comparisonRHS(p)
		if !ok {
			return "", false
		}
		return p.Field + " " + pulseCmpOp(p.Op) + " " + rhs, true
	case spec.PredOneOf:
		set, ok := pulseLiteralList(p.Values)
		if !ok {
			return "", false
		}
		return p.Field + " in [" + set + "]", true
	case spec.PredNotOneOf:
		set, ok := pulseLiteralList(p.Values)
		if !ok {
			return "", false
		}
		return "not (" + p.Field + " in [" + set + "])", true
	case spec.PredBetween:
		lo, ok1 := pulseLiteral(p.Lo)
		hi, ok2 := pulseLiteral(p.Hi)
		if !ok1 || !ok2 {
			return "", false
		}
		return "(" + p.Field + " >= " + lo + " and " + p.Field + " <= " + hi + ")", true
	case spec.PredIsNull:
		return p.Field + " == nil", true
	case spec.PredNotNull:
		return p.Field + " != nil", true
	}
	return "", false
}

func joinPulseExpr(preds []spec.Predicate, op string) (string, bool) {
	parts := make([]string, 0, len(preds))
	for i := range preds {
		s, ok := renderPulseExpr(&preds[i])
		if !ok {
			return "", false
		}
		parts = append(parts, "("+s+")")
	}
	return strings.Join(parts, " "+op+" "), true
}

func comparisonRHS(p *spec.Predicate) (string, bool) {
	if p.ToField != "" {
		return p.ToField, true
	}
	return pulseLiteral(p.Value)
}

func pulseCmpOp(op string) string {
	switch op {
	case spec.PredEq:
		return "=="
	case spec.PredNe:
		return "!="
	case spec.PredLt:
		return "<"
	case spec.PredLte:
		return "<="
	case spec.PredGt:
		return ">"
	case spec.PredGte:
		return ">="
	}
	return op
}

func pulseLiteralList(vs []any) (string, bool) {
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		s, ok := pulseLiteral(v)
		if !ok {
			return "", false
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", "), true
}

// pulseLiteral renders a JSON-decoded literal in Pulse expression
// syntax: strings single-quoted, numbers via strconv, bools bare.
func pulseLiteral(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(x, "'", "\\'") + "'", true
	case bool:
		return strconv.FormatBool(x), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 64), true
	case int:
		return strconv.FormatInt(int64(x), 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int64:
		return strconv.FormatInt(x, 10), true
	}
	return "", false
}
