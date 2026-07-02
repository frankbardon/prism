package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCalcExprDecode(t *testing.T) {
	cases := map[string]struct {
		json   string
		check  func(*testing.T, *CalcExpr)
		errSub string
	}{
		"field": {
			json: `{"field":"Horsepower"}`,
			check: func(t *testing.T, e *CalcExpr) {
				if e.Form() != "field" || e.Field != "Horsepower" {
					t.Errorf("field not parsed: %+v", e)
				}
			},
		},
		"literal-number": {
			json: `{"literal":5}`,
			check: func(t *testing.T, e *CalcExpr) {
				if e.Form() != "literal" {
					t.Errorf("literal not parsed: %+v", e)
				}
			},
		},
		"literal-zero": {
			json: `{"literal":0}`,
			check: func(t *testing.T, e *CalcExpr) {
				if e.Form() != "literal" {
					t.Errorf("literal zero not parsed: %+v", e)
				}
			},
		},
		"arithmetic": {
			json: `{"op":"div","operands":[{"field":"a"},{"field":"b"}]}`,
			check: func(t *testing.T, e *CalcExpr) {
				if e.Op != CalcDiv || len(e.Operands) != 2 {
					t.Errorf("arithmetic not parsed: %+v", e)
				}
			},
		},
		"function": {
			json: `{"fn":"coalesce","args":[{"field":"a"},{"literal":0}]}`,
			check: func(t *testing.T, e *CalcExpr) {
				if e.Fn != CalcFnCoalesce || len(e.Args) != 2 {
					t.Errorf("function not parsed: %+v", e)
				}
			},
		},
		"concat": {
			json: `{"concat":[{"field":"a"},{"literal":"x"}]}`,
			check: func(t *testing.T, e *CalcExpr) {
				if len(e.Concat) != 2 {
					t.Errorf("concat not parsed: %+v", e)
				}
			},
		},
		"case": {
			json: `{"case":[{"when":{"op":"gt","field":"a","value":1},"then":{"literal":1}}],"else":{"literal":0}}`,
			check: func(t *testing.T, e *CalcExpr) {
				if len(e.Case) != 1 || e.Else == nil {
					t.Errorf("case not parsed: %+v", e)
				}
			},
		},
		"if-alias": {
			json: `{"if":[{"when":{"op":"gt","field":"a","value":1},"then":{"literal":1}}],"else":{"literal":0}}`,
			check: func(t *testing.T, e *CalcExpr) {
				if len(e.Case) != 1 || e.If != nil {
					t.Errorf("if-alias not normalised to case: %+v", e)
				}
			},
		},
		"reject-string": {
			json:   `"Horsepower / Weight"`,
			errSub: "structured object",
		},
		"reject-empty": {
			json:   `{}`,
			errSub: "empty node",
		},
		"reject-multiple-forms": {
			json:   `{"field":"a","literal":1}`,
			errSub: "exactly one",
		},
		"reject-unknown-key": {
			json:   `{"field":"a","wat":1}`,
			errSub: "unknown field",
		},
		"reject-null-literal": {
			json:   `{"literal":null}`,
			errSub: "empty node",
		},
		"reject-unknown-op": {
			json:   `{"op":"pow","operands":[{"field":"a"},{"field":"b"}]}`,
			errSub: "unknown op",
		},
		"reject-sub-arity": {
			json:   `{"op":"sub","operands":[{"field":"a"}]}`,
			errSub: "exactly 2 operands",
		},
		"reject-abs-arity": {
			json:   `{"fn":"abs","args":[{"field":"a"},{"field":"b"}]}`,
			errSub: "exactly 1 argument",
		},
		"reject-case-no-else": {
			json:   `{"case":[{"when":{"op":"gt","field":"a","value":1},"then":{"literal":1}}]}`,
			errSub: "requires an `else`",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var e CalcExpr
			err := json.Unmarshal([]byte(tc.json), &e)
			if tc.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("expected error containing %q, got %v", tc.errSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, &e)
		})
	}
}

func TestCalcExprReferencedFields(t *testing.T) {
	e := CalcExpr{
		Case: []CalcBranch{
			{
				When: Predicate{Op: PredGt, Field: "revenue", Value: float64(1)},
				Then: CalcExpr{Op: CalcDiv, Operands: []CalcExpr{{Field: "revenue"}, {Field: "users"}}},
			},
		},
		Else: &CalcExpr{Field: "bonus"},
	}
	got := e.ReferencedFields()
	want := map[string]bool{"revenue": true, "users": true, "bonus": true}
	if len(got) != len(want) {
		t.Fatalf("ReferencedFields = %v, want keys %v", got, want)
	}
	for _, f := range got {
		if !want[f] {
			t.Errorf("unexpected referenced field %q", f)
		}
	}
}

func TestCalculateTransformRejectsString(t *testing.T) {
	var tr Transform
	err := json.Unmarshal([]byte(`{"calculate":"a / b","as":"ratio"}`), &tr)
	if err == nil || !strings.Contains(err.Error(), "structured object") {
		t.Fatalf("expected string-form rejection, got %v", err)
	}
}

func TestCalculateTransformDecodesStructured(t *testing.T) {
	var tr Transform
	body := `{"calculate":{"op":"div","operands":[{"field":"a"},{"field":"b"}]},"as":"ratio"}`
	if err := json.Unmarshal([]byte(body), &tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.Calculate == nil || tr.Calculate.As != "ratio" || tr.Calculate.Calculate.Op != CalcDiv {
		t.Fatalf("structured calculate not parsed: %+v", tr.Calculate)
	}
}
