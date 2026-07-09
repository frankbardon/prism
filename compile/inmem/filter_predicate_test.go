package inmem

import (
	"context"
	goerrors "errors"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// predicateTable returns a 4-row table exercising every column Kind the
// filter evaluator handles, including a nullable column (age at row 1).
//
//	row | brand_id | score | target | qty | age
//	 0  | alpha    | 0.3   | 0.5    | 2   | 21
//	 1  | alpha    | 0.7   | 0.7    | 5   | (null)
//	 2  | beta     | 0.4   | 0.2    | 1   | 41
//	 3  | beta     | 0.8   | 0.9    | 9   | 28
func predicateTable(t *testing.T) *table.Table {
	t.Helper()
	tbl, _, err := table.FromInline("pred",
		[]map[string]any{
			{"brand_id": "alpha", "score": 0.3, "target": 0.5, "qty": 2, "age": 21.0},
			{"brand_id": "alpha", "score": 0.7, "target": 0.7, "qty": 5}, // age omitted → null
			{"brand_id": "beta", "score": 0.4, "target": 0.2, "qty": 1, "age": 41.0},
			{"brand_id": "beta", "score": 0.8, "target": 0.9, "qty": 9, "age": 28.0},
		},
		[]spec.FieldSpec{
			{Name: "brand_id", Type: "string"},
			{Name: "score", Type: "float64"},
			{Name: "target", Type: "float64"},
			{Name: "qty", Type: "int"},
			{Name: "age", Type: "float64"},
		})
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	return tbl
}

// runFilter executes a predicate through the in-mem backend and returns
// the surviving row count.
func runFilter(t *testing.T, tbl *table.Table, pred spec.Predicate) int {
	t.Helper()
	n := nodes.NewFilter("f", "src", pred)
	out, err := New().Compile(context.Background(), n, []*table.Table{tbl})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return out.NumRows()
}

func TestPrismFilterOperators(t *testing.T) {
	tbl := predicateTable(t)
	cases := []struct {
		name string
		pred spec.Predicate
		want int
	}{
		{"eq_literal", spec.Predicate{Op: spec.PredEq, Field: "score", Value: 0.7}, 1},
		{"ne_literal", spec.Predicate{Op: spec.PredNe, Field: "brand_id", Value: "alpha"}, 2},
		{"lt_literal", spec.Predicate{Op: spec.PredLt, Field: "score", Value: 0.5}, 2},
		{"lte_literal", spec.Predicate{Op: spec.PredLte, Field: "qty", Value: 2}, 2},
		{"gt_literal", spec.Predicate{Op: spec.PredGt, Field: "score", Value: 0.5}, 2},
		{"gte_literal", spec.Predicate{Op: spec.PredGte, Field: "qty", Value: 5}, 2},
		{"eq_string", spec.Predicate{Op: spec.PredEq, Field: "brand_id", Value: "beta"}, 2},
		// int column vs float literal → numeric coercion.
		{"int_vs_float", spec.Predicate{Op: spec.PredGt, Field: "qty", Value: 4.5}, 2},

		// field-vs-field.
		{"gt_field", spec.Predicate{Op: spec.PredGt, Field: "score", ToField: "target"}, 1},
		{"eq_field", spec.Predicate{Op: spec.PredEq, Field: "score", ToField: "target"}, 1},

		// set membership.
		{"one_of", spec.Predicate{Op: spec.PredOneOf, Field: "brand_id", Values: []any{"alpha"}}, 2},
		{"not_one_of", spec.Predicate{Op: spec.PredNotOneOf, Field: "brand_id", Values: []any{"alpha"}}, 2},

		// inclusive range.
		{"between", spec.Predicate{Op: spec.PredBetween, Field: "score", Lo: 0.4, Hi: 0.7}, 2},

		// null checks.
		{"is_null", spec.Predicate{Op: spec.PredIsNull, Field: "age"}, 1},
		{"not_null", spec.Predicate{Op: spec.PredNotNull, Field: "age"}, 3},

		// null operand in a comparison → row excluded (age row 1 is null).
		{"null_excluded_from_cmp", spec.Predicate{Op: spec.PredGt, Field: "age", Value: 25.0}, 2},

		// boolean combinators.
		{"and", spec.Predicate{And: []spec.Predicate{
			{Op: spec.PredGt, Field: "score", Value: 0.5},
			{Op: spec.PredEq, Field: "brand_id", Value: "beta"},
		}}, 1},
		{"or", spec.Predicate{Or: []spec.Predicate{
			{Op: spec.PredLt, Field: "score", Value: 0.35},
			{Op: spec.PredGt, Field: "score", Value: 0.75},
		}}, 2},
		{"not", spec.Predicate{Not: &spec.Predicate{Op: spec.PredEq, Field: "brand_id", Value: "alpha"}}, 2},

		// nested: not(is_null) == not_null.
		{"not_is_null", spec.Predicate{Not: &spec.Predicate{Op: spec.PredIsNull, Field: "age"}}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := runFilter(t, tbl, tc.pred); got != tc.want {
				t.Errorf("survivors = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestPrismFilterUnknownFieldErrors confirms a predicate over a missing
// column surfaces PRISM_COMPILE_002 rather than silently passing.
func TestPrismFilterUnknownFieldErrors(t *testing.T) {
	tbl := predicateTable(t)
	n := nodes.NewFilter("f", "src", spec.Predicate{Op: spec.PredGt, Field: "missing", Value: 1})
	_, err := New().Compile(context.Background(), n, []*table.Table{tbl})
	if err == nil {
		t.Fatal("expected an error for an unknown field")
	}
	var ae *prismerrors.AppError
	if !goerrors.As(err, &ae) || ae.Code != "PRISM_COMPILE_002" {
		t.Fatalf("want PRISM_COMPILE_002, got %v", err)
	}
}
