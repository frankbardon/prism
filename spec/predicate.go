package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Predicate is the structured row-filter grammar that replaces the
// free-form expression string carried by FilterTransform (E2-S1). A
// predicate node is exactly one of:
//
//   - a leaf comparison ({op, field, value} or {op, field, to_field}),
//   - a set-membership test ({op: one_of|not_one_of, field, values}),
//   - an inclusive range ({op: between, field, lo, hi}),
//   - a null check ({op: is_null|not_null, field}),
//   - a boolean combinator ({and: [...]}, {or: [...]}, {not: {...}}).
//
// The grammar is intentionally minimal — no substring, regex, or date
// arithmetic. Anything richer is precomputed by the caller upstream.
//
// Literal operands (value / values / lo / hi) decode via encoding/json,
// so JSON numbers arrive as float64, strings as string, and booleans as
// bool. The in-memory evaluator (compile/inmem) coerces against the
// target column's Kind.
type Predicate struct {
	// Boolean combinators. Exactly one branch of the whole node is set;
	// combinator branches carry no leaf fields.
	And []Predicate `json:"and,omitempty"`
	Or  []Predicate `json:"or,omitempty"`
	Not *Predicate  `json:"not,omitempty"`

	// Op is the leaf operator: eq, ne, lt, lte, gt, gte, one_of,
	// not_one_of, between, is_null, not_null.
	Op string `json:"op,omitempty"`

	// Field is the left operand column name (every leaf op needs it).
	Field string `json:"field,omitempty"`

	// Value is the comparison right operand as a literal; ToField names
	// a second column instead (field-vs-field). Exactly one is set for
	// eq/ne/lt/lte/gt/gte.
	Value   any    `json:"value,omitempty"`
	ToField string `json:"to_field,omitempty"`

	// Values is the candidate set for one_of / not_one_of.
	Values []any `json:"values,omitempty"`

	// Lo and Hi are the inclusive bounds for between.
	Lo any `json:"lo,omitempty"`
	Hi any `json:"hi,omitempty"`
}

// Predicate operator constants.
const (
	PredEq       = "eq"
	PredNe       = "ne"
	PredLt       = "lt"
	PredLte      = "lte"
	PredGt       = "gt"
	PredGte      = "gte"
	PredOneOf    = "one_of"
	PredNotOneOf = "not_one_of"
	PredBetween  = "between"
	PredIsNull   = "is_null"
	PredNotNull  = "not_null"
)

// PredicateOps lists every valid leaf operator in schema order. Shared
// by the JSON Schema generator intent and the validate rule.
var PredicateOps = []string{
	PredEq, PredNe, PredLt, PredLte, PredGt, PredGte,
	PredOneOf, PredNotOneOf, PredBetween, PredIsNull, PredNotNull,
}

// IsComparison reports whether op is one of the binary comparison
// operators (eq/ne/lt/lte/gt/gte).
func IsComparison(op string) bool {
	switch op {
	case PredEq, PredNe, PredLt, PredLte, PredGt, PredGte:
		return true
	}
	return false
}

// isKnownOp reports whether op is a recognised leaf operator.
func isKnownOp(op string) bool {
	for _, o := range PredicateOps {
		if o == op {
			return true
		}
	}
	return false
}

// IsCombinator reports whether the node is a boolean combinator rather
// than a leaf.
func (p *Predicate) IsCombinator() bool {
	return p.And != nil || p.Or != nil || p.Not != nil
}

// UnmarshalJSON decodes a structured predicate and rejects the legacy
// free-form expression string outright (a raw JSON string where an
// object is expected). Unknown keys are rejected; nested predicates are
// validated recursively via this same method.
func (p *Predicate) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		return fmt.Errorf("filter predicate must be a structured object, not a string: " +
			"the expression grammar was replaced by {op, field, value} leaves and and/or/not combinators")
	}
	type shadow Predicate
	var s shadow
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return fmt.Errorf("filter predicate: %w", err)
	}
	*p = Predicate(s)
	return p.validateShape()
}

// validateShape enforces the structural invariants that make a decoded
// predicate interpretable: exactly one branch, non-empty combinators,
// a known leaf operator, and the operand fields the operator requires.
// Schema-aware checks (field existence, type compatibility, lo<=hi)
// live in the PRISM_SPEC_037 validate rule.
func (p *Predicate) validateShape() error {
	branches := 0
	if p.And != nil {
		branches++
	}
	if p.Or != nil {
		branches++
	}
	if p.Not != nil {
		branches++
	}
	if p.Op != "" {
		branches++
	}
	switch {
	case branches == 0:
		return fmt.Errorf("filter predicate: empty node (need one of op, and, or, not)")
	case branches > 1:
		return fmt.Errorf("filter predicate: exactly one of op/and/or/not allowed per node")
	}

	if p.IsCombinator() {
		if p.Op != "" || p.Field != "" || p.Value != nil || p.ToField != "" ||
			p.Values != nil || p.Lo != nil || p.Hi != nil {
			return fmt.Errorf("filter predicate: boolean combinator must not carry leaf operands")
		}
		switch {
		case p.And != nil && len(p.And) == 0:
			return fmt.Errorf("filter predicate: `and` must list at least one predicate")
		case p.Or != nil && len(p.Or) == 0:
			return fmt.Errorf("filter predicate: `or` must list at least one predicate")
		}
		return nil
	}

	// Leaf node.
	if !isKnownOp(p.Op) {
		return fmt.Errorf("filter predicate: unknown op %q (valid: %v)", p.Op, PredicateOps)
	}
	if p.Field == "" {
		return fmt.Errorf("filter predicate: op %q requires a `field`", p.Op)
	}
	switch {
	case IsComparison(p.Op):
		hasValue := p.Value != nil
		hasRef := p.ToField != ""
		if hasValue == hasRef {
			return fmt.Errorf("filter predicate: comparison %q requires exactly one of `value` or `to_field`", p.Op)
		}
	case p.Op == PredOneOf || p.Op == PredNotOneOf:
		if len(p.Values) == 0 {
			return fmt.Errorf("filter predicate: %q requires a non-empty `values` set", p.Op)
		}
	case p.Op == PredBetween:
		if p.Lo == nil || p.Hi == nil {
			return fmt.Errorf("filter predicate: `between` requires both `lo` and `hi`")
		}
	case p.Op == PredIsNull || p.Op == PredNotNull:
		if p.Value != nil || p.ToField != "" || p.Values != nil || p.Lo != nil || p.Hi != nil {
			return fmt.Errorf("filter predicate: %q takes only `field`", p.Op)
		}
	}
	return nil
}

// ReferencedFields returns the deduplicated set of column names the
// predicate reads (left operands plus any to_field right operands),
// walking combinators recursively. Order is deterministic (first-seen).
// Used by the filter-pushdown optimizer and the validate rule.
func (p *Predicate) ReferencedFields() []string {
	seen := map[string]bool{}
	var out []string
	var walk func(n *Predicate)
	walk = func(n *Predicate) {
		if n == nil {
			return
		}
		for i := range n.And {
			walk(&n.And[i])
		}
		for i := range n.Or {
			walk(&n.Or[i])
		}
		walk(n.Not)
		add := func(f string) {
			if f != "" && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
		add(n.Field)
		add(n.ToField)
	}
	walk(p)
	return out
}
