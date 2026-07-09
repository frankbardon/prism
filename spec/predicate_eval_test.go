package spec

import "testing"

func TestPredicateEvalRow(t *testing.T) {
	cases := []struct {
		name string
		pred Predicate
		row  map[string]any
		want bool
	}{
		{
			name: "eq numeric true",
			pred: Predicate{Op: PredEq, Field: "n", Value: 3.0},
			row:  map[string]any{"n": 3.0},
			want: true,
		},
		{
			name: "eq numeric int coercion",
			pred: Predicate{Op: PredEq, Field: "n", Value: 3.0},
			row:  map[string]any{"n": 3},
			want: true,
		},
		{
			name: "gte true",
			pred: Predicate{Op: PredGte, Field: "n", Value: 0.5},
			row:  map[string]any{"n": 0.78},
			want: true,
		},
		{
			name: "lt negative true",
			pred: Predicate{Op: PredLt, Field: "n", Value: 0.0},
			row:  map[string]any{"n": -0.12},
			want: true,
		},
		{
			name: "lt false",
			pred: Predicate{Op: PredLt, Field: "n", Value: 0.0},
			row:  map[string]any{"n": 0.34},
			want: false,
		},
		{
			name: "string eq",
			pred: Predicate{Op: PredEq, Field: "s", Value: "west"},
			row:  map[string]any{"s": "west"},
			want: true,
		},
		{
			name: "string ne",
			pred: Predicate{Op: PredNe, Field: "s", Value: "west"},
			row:  map[string]any{"s": "east"},
			want: true,
		},
		{
			name: "bool eq",
			pred: Predicate{Op: PredEq, Field: "b", Value: true},
			row:  map[string]any{"b": true},
			want: true,
		},
		{
			name: "to_field comparison",
			pred: Predicate{Op: PredGt, Field: "a", ToField: "b"},
			row:  map[string]any{"a": 5.0, "b": 2.0},
			want: true,
		},
		{
			name: "one_of member",
			pred: Predicate{Op: PredOneOf, Field: "s", Values: []any{"x", "y", "z"}},
			row:  map[string]any{"s": "y"},
			want: true,
		},
		{
			name: "not_one_of non-member",
			pred: Predicate{Op: PredNotOneOf, Field: "s", Values: []any{"x", "y"}},
			row:  map[string]any{"s": "z"},
			want: true,
		},
		{
			name: "between inclusive",
			pred: Predicate{Op: PredBetween, Field: "n", Lo: 1.0, Hi: 10.0},
			row:  map[string]any{"n": 10.0},
			want: true,
		},
		{
			name: "between out of range",
			pred: Predicate{Op: PredBetween, Field: "n", Lo: 1.0, Hi: 10.0},
			row:  map[string]any{"n": 11.0},
			want: false,
		},
		{
			name: "is_null on missing key",
			pred: Predicate{Op: PredIsNull, Field: "missing"},
			row:  map[string]any{"n": 1.0},
			want: true,
		},
		{
			name: "is_null on nil value",
			pred: Predicate{Op: PredIsNull, Field: "n"},
			row:  map[string]any{"n": nil},
			want: true,
		},
		{
			name: "not_null true",
			pred: Predicate{Op: PredNotNull, Field: "n"},
			row:  map[string]any{"n": 1.0},
			want: true,
		},
		{
			name: "null left excludes comparison",
			pred: Predicate{Op: PredEq, Field: "n", Value: 1.0},
			row:  map[string]any{"n": nil},
			want: false,
		},
		{
			name: "incomparable domains false",
			pred: Predicate{Op: PredEq, Field: "n", Value: "1"},
			row:  map[string]any{"n": 1.0},
			want: false,
		},
		{
			name: "and combinator",
			pred: Predicate{And: []Predicate{
				{Op: PredGt, Field: "n", Value: 0.0},
				{Op: PredLt, Field: "n", Value: 10.0},
			}},
			row:  map[string]any{"n": 5.0},
			want: true,
		},
		{
			name: "or combinator",
			pred: Predicate{Or: []Predicate{
				{Op: PredLt, Field: "n", Value: 0.0},
				{Op: PredGt, Field: "n", Value: 100.0},
			}},
			row:  map[string]any{"n": 150.0},
			want: true,
		},
		{
			name: "not combinator rescues null",
			pred: Predicate{Not: &Predicate{Op: PredEq, Field: "n", Value: 1.0}},
			row:  map[string]any{"n": nil},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pred.EvalRow(tc.row); got != tc.want {
				t.Errorf("EvalRow(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
