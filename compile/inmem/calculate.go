package inmem

import (
	"context"
	"fmt"
	"math"
	"strconv"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// executeCalculate appends one derived column produced by the node's
// structured CalcExpr (E2-S2 — no expression engine involved). The output column
// type is inferred by nodes.CalcResultType: a numeric expression yields
// an F64 column, a string expression a categorical column.
//
// Null / division policy (documented, 2-valued):
//
//   - arithmetic (add/sub/mul/div/mod) and the numeric functions
//     (abs/round/floor/ceil/neg/min/max) propagate nulls: any null
//     operand yields a null result;
//   - div / mod by zero yields a null result (silently — the Backend
//     Compile path has no warning sink; see FOLLOWUPS / E2-S4 docs);
//   - coalesce returns the first non-null argument (null only if all
//     arguments are null);
//   - concat treats null operands as the empty string and always yields
//     a (possibly empty) string;
//   - case evaluates its branches' `when` predicates via the shared
//     E2-S1 predicate evaluator (evalPredicate) and returns the first
//     matching `then`, else the mandatory `else`.
func executeCalculate(_ context.Context, n *nodes.CalculateNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	if n.As() == "" {
		return nil, fmt.Errorf("CalculateNode: missing 'as' name")
	}

	expr := n.Calc()
	rows := in.NumRows()
	resultType := nodes.CalcResultType(expr, in.Schema())
	isString := table.KindFromFieldType(resultType) == table.KindString

	nulls := table.NewNullBitmap(rows)
	floats := make(table.FloatColumn, 0)
	strs := make(table.StringColumn, 0)
	if isString {
		strs = make(table.StringColumn, rows)
	} else {
		floats = make(table.FloatColumn, rows)
	}

	for i := 0; i < rows; i++ {
		v, err := evalCalc(&expr, in, i)
		if err != nil {
			return nil, err
		}
		if v == nil {
			nulls.Set(i)
			continue
		}
		if isString {
			strs[i] = calcString(v)
			continue
		}
		f, ok := coerceFloat(v)
		if !ok {
			// A numeric-typed column produced a non-numeric value
			// (type mismatch) — record it as null rather than fail the
			// whole plan.
			nulls.Set(i)
			continue
		}
		floats[i] = f
	}

	nullable := nulls.Count() > 0
	schema := cloneSchemaShallow(in.Schema())
	schema.Fields = append(schema.Fields, table.Field{Name: n.As(), Type: resultType, Nullable: nullable})

	var outCol table.Column
	if isString {
		outCol = strs
	} else {
		outCol = floats
	}
	if nullable {
		outCol = table.NullableColumn{Inner: outCol, Nulls: nulls}
	}

	cols := make(map[string]table.Column, len(in.FieldNames())+1)
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		cols[name] = col
	}
	cols[n.As()] = outCol

	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(schema, cols, rows, hash)
}

// evalCalc evaluates a CalcExpr node against row i of tbl. It returns a
// float64, string, or bool for a value, or nil for a null result.
func evalCalc(e *spec.CalcExpr, tbl *table.Table, i int) (any, error) {
	switch e.Form() {
	case "field":
		col, ok := tbl.Column(e.Field)
		if !ok {
			return nil, calcFieldErr(e.Field)
		}
		if col.IsNull(i) {
			return nil, nil
		}
		return col.ValueAt(i), nil
	case "literal":
		return e.Literal, nil
	case "op":
		return evalArithmetic(e, tbl, i)
	case "fn":
		return evalFunction(e, tbl, i)
	case "concat":
		return evalConcat(e, tbl, i)
	case "case":
		return evalCase(e, tbl, i)
	}
	return nil, calcFormErr()
}

// evalArithmetic evaluates an arithmetic node with null propagation.
// div / mod by zero yields nil (null).
func evalArithmetic(e *spec.CalcExpr, tbl *table.Table, i int) (any, error) {
	vals := make([]float64, len(e.Operands))
	for k := range e.Operands {
		v, err := evalCalc(&e.Operands[k], tbl, i)
		if err != nil {
			return nil, err
		}
		f, ok := coerceFloat(v)
		if v == nil || !ok {
			return nil, nil // null propagation / non-numeric operand
		}
		vals[k] = f
	}
	switch e.Op {
	case spec.CalcAdd:
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		return sum, nil
	case spec.CalcMul:
		prod := 1.0
		for _, v := range vals {
			prod *= v
		}
		return prod, nil
	case spec.CalcSub:
		return vals[0] - vals[1], nil
	case spec.CalcDiv:
		if vals[1] == 0 {
			return nil, nil
		}
		return vals[0] / vals[1], nil
	case spec.CalcMod:
		if vals[1] == 0 {
			return nil, nil
		}
		return math.Mod(vals[0], vals[1]), nil
	}
	return nil, calcOpErr(e.Op)
}

// evalFunction evaluates a pure-function node.
func evalFunction(e *spec.CalcExpr, tbl *table.Table, i int) (any, error) {
	switch e.Fn {
	case spec.CalcFnCoalesce:
		for k := range e.Args {
			v, err := evalCalc(&e.Args[k], tbl, i)
			if err != nil {
				return nil, err
			}
			if v != nil {
				return v, nil
			}
		}
		return nil, nil
	case spec.CalcFnMin, spec.CalcFnMax:
		var acc float64
		seen := false
		for k := range e.Args {
			v, err := evalCalc(&e.Args[k], tbl, i)
			if err != nil {
				return nil, err
			}
			f, ok := coerceFloat(v)
			if v == nil || !ok {
				continue // skip nulls
			}
			if !seen || (e.Fn == spec.CalcFnMin && f < acc) || (e.Fn == spec.CalcFnMax && f > acc) {
				acc = f
				seen = true
			}
		}
		if !seen {
			return nil, nil
		}
		return acc, nil
	}

	// Single-argument numeric functions: null-propagating.
	v, err := evalCalc(&e.Args[0], tbl, i)
	if err != nil {
		return nil, err
	}
	f, ok := coerceFloat(v)
	if v == nil || !ok {
		return nil, nil
	}
	switch e.Fn {
	case spec.CalcFnAbs:
		return math.Abs(f), nil
	case spec.CalcFnRound:
		return math.Round(f), nil
	case spec.CalcFnFloor:
		return math.Floor(f), nil
	case spec.CalcFnCeil:
		return math.Ceil(f), nil
	case spec.CalcFnNeg:
		return -f, nil
	}
	return nil, calcFnErr(e.Fn)
}

// evalConcat joins its operands into a string; null operands render as
// the empty string.
func evalConcat(e *spec.CalcExpr, tbl *table.Table, i int) (any, error) {
	out := ""
	for k := range e.Concat {
		v, err := evalCalc(&e.Concat[k], tbl, i)
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		out += calcString(v)
	}
	return out, nil
}

// evalCase evaluates the first branch whose `when` predicate holds,
// falling back to `else`. The predicate is evaluated by the shared
// E2-S1 evaluator (evalPredicate in filter.go).
func evalCase(e *spec.CalcExpr, tbl *table.Table, i int) (any, error) {
	for k := range e.Case {
		ok, err := evalPredicate(&e.Case[k].When, tbl, i)
		if err != nil {
			return nil, err
		}
		if ok {
			return evalCalc(&e.Case[k].Then, tbl, i)
		}
	}
	if e.Else != nil {
		return evalCalc(e.Else, tbl, i)
	}
	return nil, nil
}

func calcFieldErr(field string) error {
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Calculate expression references unknown column "+field+".",
		map[string]any{"Field": field, "Site": "calculate"},
	)
}

func calcOpErr(op string) error {
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Calculate expression uses unsupported arithmetic op "+op+".",
		map[string]any{"Op": op, "Site": "calculate"},
	)
}

func calcFnErr(fn string) error {
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Calculate expression uses unsupported function "+fn+".",
		map[string]any{"Fn": fn, "Site": "calculate"},
	)
}

func calcFormErr() error {
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Calculate expression node is empty (no field/literal/op/fn/concat/case).",
		map[string]any{"Site": "calculate"},
	)
}

// calcString renders a value as a string for concat / string columns.
func calcString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return fmt.Sprintf("%v", v)
}
