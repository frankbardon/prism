package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// CalcExpr is the structured derived-column grammar that replaces the
// free-form expression string carried by CalculateTransform (E2-S2). A
// node is exactly one of:
//
//   - a field reference   ({field: "Horsepower"}),
//   - a literal           ({literal: 5} | {literal: "x"} | {literal: true}),
//   - an arithmetic op    ({op: add|sub|mul|div|mod, operands: [...]}),
//   - a pure function     ({fn: abs|round|floor|ceil|neg|coalesce|min|max, args: [...]}),
//   - a string concat     ({concat: [...]}),
//   - a conditional       ({case: [{when: <Predicate>, then: <CalcExpr>}...], else: <CalcExpr>}).
//
// `if` is accepted as an alias for `case`. The grammar is intentionally
// minimal — no log/sqrt/pow/trig, no substring, no date arithmetic.
// Anything richer is precomputed by the caller upstream.
//
// The `case` branch `when` reuses the E2-S1 Predicate grammar verbatim
// (no second predicate grammar is forked). Literal operands decode via
// encoding/json, so JSON numbers arrive as float64, strings as string,
// booleans as bool; the in-memory evaluator (compile/inmem) coerces
// against the operand's runtime kind.
type CalcExpr struct {
	// Field references an input column by name.
	Field string `json:"field,omitempty"`

	// Literal is a constant scalar (number / string / bool). A null
	// literal is rejected at decode.
	Literal any `json:"literal,omitempty"`

	// Op is the arithmetic operator: add, sub, mul, div, mod. Operands
	// carries its arguments.
	Op       string     `json:"op,omitempty"`
	Operands []CalcExpr `json:"operands,omitempty"`

	// Fn is the pure function name: abs, round, floor, ceil, neg,
	// coalesce, min, max. Args carries its arguments.
	Fn   string     `json:"fn,omitempty"`
	Args []CalcExpr `json:"args,omitempty"`

	// Concat joins its operands into a single string.
	Concat []CalcExpr `json:"concat,omitempty"`

	// Case is an ordered list of when→then branches; Else is the
	// mandatory fallback when no branch matches. If is a decode-time
	// alias for Case (normalised away in UnmarshalJSON).
	Case []CalcBranch `json:"case,omitempty"`
	If   []CalcBranch `json:"if,omitempty"`
	Else *CalcExpr    `json:"else,omitempty"`
}

// CalcBranch is one when→then arm of a conditional CalcExpr. `when` is a
// structured filter Predicate (E2-S1); `then` is the value produced when
// the predicate holds for the row.
type CalcBranch struct {
	When Predicate `json:"when"`
	Then CalcExpr  `json:"then"`
}

// Arithmetic operator constants.
const (
	CalcAdd = "add"
	CalcSub = "sub"
	CalcMul = "mul"
	CalcDiv = "div"
	CalcMod = "mod"
)

// CalcArithmeticOps lists every arithmetic operator in schema order.
var CalcArithmeticOps = []string{CalcAdd, CalcSub, CalcMul, CalcDiv, CalcMod}

// Pure-function constants.
const (
	CalcFnAbs      = "abs"
	CalcFnRound    = "round"
	CalcFnFloor    = "floor"
	CalcFnCeil     = "ceil"
	CalcFnNeg      = "neg"
	CalcFnCoalesce = "coalesce"
	CalcFnMin      = "min"
	CalcFnMax      = "max"
)

// CalcFunctions lists every pure function in schema order.
var CalcFunctions = []string{
	CalcFnAbs, CalcFnRound, CalcFnFloor, CalcFnCeil, CalcFnNeg,
	CalcFnCoalesce, CalcFnMin, CalcFnMax,
}

// calcDiscriminators are the mutually-exclusive keys that select a
// CalcExpr node form.
var calcDiscriminators = []string{"field", "literal", "op", "fn", "concat", "case", "if"}

// IsArithmeticOp reports whether op is a recognised arithmetic operator.
func IsArithmeticOp(op string) bool {
	for _, o := range CalcArithmeticOps {
		if o == op {
			return true
		}
	}
	return false
}

// IsCalcFunction reports whether fn is a recognised pure function.
func IsCalcFunction(fn string) bool {
	for _, f := range CalcFunctions {
		if f == fn {
			return true
		}
	}
	return false
}

// Form returns the discriminator naming which node form is populated, or
// "" for an empty node. Works for both decoded and Go-constructed nodes.
func (e *CalcExpr) Form() string {
	switch {
	case e.Field != "":
		return "field"
	case e.Literal != nil:
		return "literal"
	case e.Op != "":
		return "op"
	case e.Fn != "":
		return "fn"
	case e.Concat != nil:
		return "concat"
	case e.Case != nil:
		return "case"
	}
	return ""
}

// UnmarshalJSON decodes a structured calc node and rejects the legacy
// free-form expression string outright (a raw JSON string where an
// object is expected). Exactly one discriminator key must be present;
// unknown keys are rejected; nested CalcExprs and Predicates validate
// recursively via their own UnmarshalJSON.
func (e *CalcExpr) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		return fmt.Errorf("calculate must be a structured object, not a string: " +
			"the expression grammar was replaced by {field}/{literal}/{op,operands}/" +
			"{fn,args}/{concat}/{case,else} nodes")
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("calculate expression: %w", err)
	}
	matched := 0
	for _, k := range calcDiscriminators {
		if _, ok := probe[k]; ok {
			matched++
		}
	}
	switch {
	case matched == 0:
		return fmt.Errorf("calculate expression: empty node " +
			"(need one of field, literal, op, fn, concat, case)")
	case matched > 1:
		return fmt.Errorf("calculate expression: exactly one of " +
			"field/literal/op/fn/concat/case allowed per node")
	}

	type shadow CalcExpr
	var s shadow
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return fmt.Errorf("calculate expression: %w", err)
	}
	*e = CalcExpr(s)
	if e.If != nil {
		e.Case = e.If
		e.If = nil
	}
	return e.validateShape()
}

// validateShape enforces the structural invariants that make a decoded
// node interpretable: one populated form, a known operator/function,
// the right operand arity, and (for `case`) a mandatory `else` and
// well-formed branches. Schema-aware checks (field existence, literal
// type, literal-zero divisors) live in the PRISM_SPEC_038 validate rule.
func (e *CalcExpr) validateShape() error {
	switch e.Form() {
	case "field":
		return nil
	case "literal":
		switch e.Literal.(type) {
		case float64, float32, int, int32, int64, string, bool:
			return nil
		default:
			return fmt.Errorf("calculate expression: `literal` must be a number, string, or bool")
		}
	case "op":
		if !IsArithmeticOp(e.Op) {
			return fmt.Errorf("calculate expression: unknown op %q (valid: %v)", e.Op, CalcArithmeticOps)
		}
		n := len(e.Operands)
		switch e.Op {
		case CalcAdd, CalcMul:
			if n < 2 {
				return fmt.Errorf("calculate expression: op %q needs at least 2 operands", e.Op)
			}
		case CalcSub, CalcDiv, CalcMod:
			if n != 2 {
				return fmt.Errorf("calculate expression: op %q needs exactly 2 operands", e.Op)
			}
		}
		return nil
	case "fn":
		if !IsCalcFunction(e.Fn) {
			return fmt.Errorf("calculate expression: unknown fn %q (valid: %v)", e.Fn, CalcFunctions)
		}
		n := len(e.Args)
		switch e.Fn {
		case CalcFnAbs, CalcFnRound, CalcFnFloor, CalcFnCeil, CalcFnNeg:
			if n != 1 {
				return fmt.Errorf("calculate expression: fn %q takes exactly 1 argument", e.Fn)
			}
		case CalcFnCoalesce, CalcFnMin, CalcFnMax:
			if n < 2 {
				return fmt.Errorf("calculate expression: fn %q needs at least 2 arguments", e.Fn)
			}
		}
		return nil
	case "concat":
		if len(e.Concat) == 0 {
			return fmt.Errorf("calculate expression: `concat` needs at least one operand")
		}
		return nil
	case "case":
		if len(e.Case) == 0 {
			return fmt.Errorf("calculate expression: `case` needs at least one when/then branch")
		}
		if e.Else == nil {
			return fmt.Errorf("calculate expression: `case` requires an `else` fallback")
		}
		return nil
	}
	return fmt.Errorf("calculate expression: empty node (need one of field, literal, op, fn, concat, case)")
}

// ReferencedFields returns the deduplicated column names the expression
// reads — field references plus every field named by a `case` branch's
// `when` predicate — walking the whole tree. Order is deterministic
// (first-seen). Used by the validate rule and plan diagnostics.
func (e *CalcExpr) ReferencedFields() []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	var walk func(n *CalcExpr)
	walk = func(n *CalcExpr) {
		if n == nil {
			return
		}
		add(n.Field)
		for i := range n.Operands {
			walk(&n.Operands[i])
		}
		for i := range n.Args {
			walk(&n.Args[i])
		}
		for i := range n.Concat {
			walk(&n.Concat[i])
		}
		for i := range n.Case {
			for _, f := range n.Case[i].When.ReferencedFields() {
				add(f)
			}
			walk(&n.Case[i].Then)
		}
		walk(n.Else)
	}
	walk(e)
	return out
}
