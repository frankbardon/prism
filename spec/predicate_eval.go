package spec

// EvalRow evaluates the predicate against a single row of values keyed by
// column name. It exists for the encode-time conditional-encoding path
// (encode/encode_condition.go), where marks are evaluated one row at a
// time rather than columnar.
//
// The semantics are IDENTICAL to the columnar evaluator in
// compile/inmem/filter.go — same operator table, same 2-valued null
// rule:
//
//   - is_null / not_null test the operand's null state directly.
//   - every other leaf with a null (missing key or nil) operand — left
//     or right — evaluates FALSE (the row is excluded unless rescued by
//     an enclosing or / not).
//   - and / or / not then combine plain booleans.
//
// The one deliberate difference from the columnar path is where the
// comparison domain comes from: inmem keys it off the column Kind, while
// EvalRow infers it from the operand's Go type. For rows materialised
// from a table.Table (Column.ValueAt), the Go type matches the column
// Kind, so the two evaluators agree value-for-value. The two are kept in
// sync by contract; there is no shared table dependency because spec/
// must stay free of the table package.
func (p Predicate) EvalRow(row map[string]any) bool {
	return evalPredicateRow(&p, row)
}

func evalPredicateRow(p *Predicate, row map[string]any) bool {
	switch {
	case p.And != nil:
		for k := range p.And {
			if !evalPredicateRow(&p.And[k], row) {
				return false
			}
		}
		return true
	case p.Or != nil:
		for k := range p.Or {
			if evalPredicateRow(&p.Or[k], row) {
				return true
			}
		}
		return false
	case p.Not != nil:
		return !evalPredicateRow(p.Not, row)
	}
	return evalLeafRow(p, row)
}

// evalLeafRow evaluates a leaf predicate (comparison / set / between /
// null check) against the row.
func evalLeafRow(p *Predicate, row map[string]any) bool {
	left, present := row[p.Field]
	leftNull := !present || left == nil

	switch p.Op {
	case PredIsNull:
		return leftNull
	case PredNotNull:
		return !leftNull
	}

	// Every remaining op excludes a null left operand.
	if leftNull {
		return false
	}

	switch p.Op {
	case PredEq, PredNe, PredLt, PredLte, PredGt, PredGte:
		right, rightNull := comparisonRightRow(p, row)
		if rightNull {
			return false
		}
		cmp, comparable := compareRowValues(left, right)
		if !comparable {
			return false
		}
		return applyRowCmp(p.Op, cmp)
	case PredOneOf, PredNotOneOf:
		member := false
		for _, cand := range p.Values {
			if cmp, comparable := compareRowValues(left, cand); comparable && cmp == 0 {
				member = true
				break
			}
		}
		if p.Op == PredOneOf {
			return member
		}
		return !member
	case PredBetween:
		loCmp, loOK := compareRowValues(left, p.Lo)
		hiCmp, hiOK := compareRowValues(left, p.Hi)
		if !loOK || !hiOK {
			return false
		}
		return loCmp >= 0 && hiCmp <= 0
	}
	return false
}

// comparisonRightRow resolves the right operand of a comparison: either
// the value at the to_field column (returning its null state) or the
// literal value. A literal is never null.
func comparisonRightRow(p *Predicate, row map[string]any) (any, bool) {
	if p.ToField != "" {
		v, present := row[p.ToField]
		if !present || v == nil {
			return nil, true
		}
		return v, false
	}
	return p.Value, false
}

// applyRowCmp maps a comparison operator + a -1/0/1 ordering to a bool.
func applyRowCmp(op string, cmp int) bool {
	switch op {
	case PredEq:
		return cmp == 0
	case PredNe:
		return cmp != 0
	case PredLt:
		return cmp < 0
	case PredLte:
		return cmp <= 0
	case PredGt:
		return cmp > 0
	case PredGte:
		return cmp >= 0
	}
	return false
}

// compareRowValues compares two row values, inferring the comparison
// domain from the left operand's Go type. Returns (-1|0|1, true) when the
// values are comparable, or (0, false) when the right operand cannot be
// coerced into the left's domain. Mirrors compareFilterValues in
// compile/inmem/filter.go.
func compareRowValues(left, right any) (int, bool) {
	if lf, lok := rowFloat(left); lok {
		rf, rok := rowFloat(right)
		if !rok {
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
	}
	if ls, lok := left.(string); lok {
		rs, rok := right.(string)
		if !rok {
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
	}
	if lb, lok := left.(bool); lok {
		rb, rok := right.(bool)
		if !rok {
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

func rowFloat(v any) (float64, bool) {
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
