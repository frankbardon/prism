package inmem

import (
	"context"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// executeFilter evaluates the FilterNode's structured predicate
// (spec.Predicate) per row over the input columns. Surviving rows are
// picked from every input column; the schema is preserved verbatim.
// No expr-lang is involved (E2-S1).
//
// Null semantics (2-valued, documented): a null operand in a leaf
// comparison / between / set-membership makes that leaf evaluate FALSE
// (the row is excluded unless rescued by an enclosing or / not). Null
// state is tested explicitly with is_null / not_null. Boolean and / or
// / not then operate on plain booleans.
func executeFilter(_ context.Context, n *nodes.FilterNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	pred := n.Predicate()
	rows := in.NumRows()
	mask := make([]bool, rows)
	keep := 0
	for i := 0; i < rows; i++ {
		ok, err := evalPredicate(&pred, in, i)
		if err != nil {
			return nil, err
		}
		if ok {
			mask[i] = true
			keep++
		}
	}

	out := make(map[string]table.Column, len(in.FieldNames()))
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		out[name] = pickRowsByMask(col, mask)
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(in.Schema(), out, keep, hash)
}

// evalPredicate evaluates a predicate node against row i of tbl.
func evalPredicate(p *spec.Predicate, tbl *table.Table, i int) (bool, error) {
	switch {
	case p.And != nil:
		for k := range p.And {
			ok, err := evalPredicate(&p.And[k], tbl, i)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case p.Or != nil:
		for k := range p.Or {
			ok, err := evalPredicate(&p.Or[k], tbl, i)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case p.Not != nil:
		ok, err := evalPredicate(p.Not, tbl, i)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}
	return evalLeaf(p, tbl, i)
}

// evalLeaf evaluates a leaf predicate (comparison / set / between /
// null check) against row i.
func evalLeaf(p *spec.Predicate, tbl *table.Table, i int) (bool, error) {
	col, ok := tbl.Column(p.Field)
	if !ok {
		return false, filterFieldErr(p.Field)
	}
	leftNull := col.IsNull(i)
	left := col.ValueAt(i)

	switch p.Op {
	case spec.PredIsNull:
		return leftNull || left == nil, nil
	case spec.PredNotNull:
		return !(leftNull || left == nil), nil
	}

	// Every remaining op excludes a null left operand.
	if leftNull || left == nil {
		return false, nil
	}

	switch p.Op {
	case spec.PredEq, spec.PredNe, spec.PredLt, spec.PredLte, spec.PredGt, spec.PredGte:
		right, rightNull, err := comparisonRight(p, tbl, i)
		if err != nil {
			return false, err
		}
		if rightNull {
			return false, nil
		}
		cmp, comparable := compareFilterValues(col.Kind(), left, right)
		if !comparable {
			return false, nil
		}
		return applyCmp(p.Op, cmp), nil
	case spec.PredOneOf, spec.PredNotOneOf:
		member := false
		for _, cand := range p.Values {
			cmp, comparable := compareFilterValues(col.Kind(), left, cand)
			if comparable && cmp == 0 {
				member = true
				break
			}
		}
		if p.Op == spec.PredOneOf {
			return member, nil
		}
		return !member, nil
	case spec.PredBetween:
		loCmp, loOK := compareFilterValues(col.Kind(), left, p.Lo)
		hiCmp, hiOK := compareFilterValues(col.Kind(), left, p.Hi)
		if !loOK || !hiOK {
			return false, nil
		}
		return loCmp >= 0 && hiCmp <= 0, nil
	}
	return false, filterOpErr(p.Op)
}

// comparisonRight resolves the right operand of a comparison: either the
// column named by to_field (returning its null state) or the literal
// value. A literal is never null.
func comparisonRight(p *spec.Predicate, tbl *table.Table, i int) (any, bool, error) {
	if p.ToField != "" {
		col, ok := tbl.Column(p.ToField)
		if !ok {
			return nil, false, filterFieldErr(p.ToField)
		}
		if col.IsNull(i) {
			return nil, true, nil
		}
		v := col.ValueAt(i)
		return v, v == nil, nil
	}
	return p.Value, false, nil
}

// applyCmp maps a comparison operator + a -1/0/1 ordering to a bool.
func applyCmp(op string, cmp int) bool {
	switch op {
	case spec.PredEq:
		return cmp == 0
	case spec.PredNe:
		return cmp != 0
	case spec.PredLt:
		return cmp < 0
	case spec.PredLte:
		return cmp <= 0
	case spec.PredGt:
		return cmp > 0
	case spec.PredGte:
		return cmp >= 0
	}
	return false
}

// compareFilterValues compares a column value (left) against another
// value (right) using the column's Kind to pick the comparison domain.
// Returns (-1|0|1, true) when the values are comparable, or (0, false)
// when the right operand cannot be coerced into the column's domain.
func compareFilterValues(kind table.Kind, left, right any) (int, bool) {
	switch kind {
	case table.KindInt, table.KindFloat, table.KindDate:
		lf, lok := coerceFloat(left)
		rf, rok := coerceFloat(right)
		if !lok || !rok {
			return 0, false
		}
		switch {
		case lf < rf:
			return -1, true
		case lf > rf:
			return 1, true
		default:
			return 0, true
		}
	case table.KindString:
		ls, lok := left.(string)
		rs, rok := right.(string)
		if !lok || !rok {
			return 0, false
		}
		switch {
		case ls < rs:
			return -1, true
		case ls > rs:
			return 1, true
		default:
			return 0, true
		}
	case table.KindBool:
		lb, lok := coerceBool(left)
		rb, rok := coerceBool(right)
		if !lok || !rok {
			return 0, false
		}
		if lb == rb {
			return 0, true
		}
		// false < true.
		if !lb && rb {
			return -1, true
		}
		return 1, true
	}
	return 0, false
}

func coerceFloat(v any) (float64, bool) {
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

func coerceBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func filterFieldErr(field string) error {
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Filter predicate references unknown column "+field+".",
		map[string]any{
			"Field": field,
			"Site":  "filter",
		},
	)
}

func filterOpErr(op string) error {
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Filter predicate uses unsupported operator "+op+".",
		map[string]any{
			"Op":   op,
			"Site": "filter",
		},
	)
}
