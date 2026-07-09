package inmem

import (
	"context"
	"testing"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// calcTestTable builds a small table with one nullable numeric column
// (`bonus`, null at row 1) so the null/coalesce/div paths are covered.
func calcTestTable(t *testing.T) *table.Table {
	t.Helper()
	nb := table.NewNullBitmap(3)
	nb.Set(1) // bonus is null at row 1
	cols := map[string]table.Column{
		"region":  table.StringColumn{"NA", "EU", "APAC"},
		"revenue": table.FloatColumn{100, 200, 300},
		"users":   table.FloatColumn{10, 0, 30}, // users==0 at row 1
		"bonus":   table.NullableColumn{Inner: table.FloatColumn{5, 0, 7}, Nulls: nb},
	}
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "revenue", Type: table.FieldTypeF64},
		{Name: "users", Type: table.FieldTypeF64},
		{Name: "bonus", Type: table.FieldTypeF64, Nullable: true},
	}}
	tbl, err := table.NewTable(schema, cols, 3, "xxh64:test")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// runCalc executes a calc expression against calcTestTable and returns
// the produced output column.
func runCalc(t *testing.T, e spec.CalcExpr, as string) table.Column {
	t.Helper()
	in := calcTestTable(t)
	n := nodes.NewCalculate("c:1", "src", e, as)
	out, err := New().Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	col, ok := out.Column(as)
	if !ok {
		t.Fatalf("missing output column %q", as)
	}
	return col
}

func lit(v any) spec.CalcExpr            { return spec.CalcExpr{Literal: v} }
func fld(f string) spec.CalcExpr         { return spec.CalcExpr{Field: f} }
func ptr(e spec.CalcExpr) *spec.CalcExpr { return &e }

func TestCalcArithmetic(t *testing.T) {
	cases := []struct {
		name string
		expr spec.CalcExpr
		want []float64
	}{
		{"add", spec.CalcExpr{Op: spec.CalcAdd, Operands: []spec.CalcExpr{fld("revenue"), lit(float64(1))}}, []float64{101, 201, 301}},
		{"sub", spec.CalcExpr{Op: spec.CalcSub, Operands: []spec.CalcExpr{fld("revenue"), fld("users")}}, []float64{90, 200, 270}},
		{"mul", spec.CalcExpr{Op: spec.CalcMul, Operands: []spec.CalcExpr{fld("users"), lit(float64(2))}}, []float64{20, 0, 60}},
		{"div", spec.CalcExpr{Op: spec.CalcDiv, Operands: []spec.CalcExpr{fld("revenue"), fld("users")}}, []float64{10, 0, 10}},
		{"mod", spec.CalcExpr{Op: spec.CalcMod, Operands: []spec.CalcExpr{fld("revenue"), lit(float64(7))}}, []float64{2, 4, 6}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			col := runCalc(t, c.expr, "out")
			for i, w := range c.want {
				if c.name == "div" && i == 1 {
					// users==0 → null.
					if !col.IsNull(1) {
						t.Fatalf("div by zero row 1 should be null, got %v", col.ValueAt(1))
					}
					continue
				}
				if col.IsNull(i) {
					t.Fatalf("row %d unexpectedly null", i)
				}
				if got := col.ValueAt(i).(float64); got != w {
					t.Errorf("%s row %d = %v, want %v", c.name, i, got, w)
				}
			}
		})
	}
}

func TestCalcDivByZeroNull(t *testing.T) {
	// revenue / users, users==0 at row 1 → null; mod likewise.
	div := runCalc(t, spec.CalcExpr{Op: spec.CalcDiv, Operands: []spec.CalcExpr{fld("revenue"), fld("users")}}, "d")
	if !div.IsNull(1) {
		t.Errorf("div-by-zero should be null at row 1")
	}
	if div.IsNull(0) || div.ValueAt(0).(float64) != 10 {
		t.Errorf("div row 0 = %v, want 10", div.ValueAt(0))
	}
	mod := runCalc(t, spec.CalcExpr{Op: spec.CalcMod, Operands: []spec.CalcExpr{fld("revenue"), fld("users")}}, "m")
	if !mod.IsNull(1) {
		t.Errorf("mod-by-zero should be null at row 1")
	}
}

func TestCalcNullPropagation(t *testing.T) {
	// bonus is null at row 1; revenue + bonus should be null there.
	col := runCalc(t, spec.CalcExpr{Op: spec.CalcAdd, Operands: []spec.CalcExpr{fld("revenue"), fld("bonus")}}, "out")
	if !col.IsNull(1) {
		t.Errorf("null operand should propagate to null at row 1")
	}
	if col.IsNull(0) || col.ValueAt(0).(float64) != 105 {
		t.Errorf("row 0 = %v, want 105", col.ValueAt(0))
	}
}

func TestCalcFunctions(t *testing.T) {
	neg := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnNeg, Args: []spec.CalcExpr{fld("revenue")}}, "o")
	if neg.ValueAt(0).(float64) != -100 {
		t.Errorf("neg = %v, want -100", neg.ValueAt(0))
	}
	abs := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnAbs, Args: []spec.CalcExpr{
		spec.CalcExpr{Op: spec.CalcSub, Operands: []spec.CalcExpr{fld("users"), fld("revenue")}},
	}}, "o")
	if abs.ValueAt(0).(float64) != 90 {
		t.Errorf("abs = %v, want 90", abs.ValueAt(0))
	}
	round := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnRound, Args: []spec.CalcExpr{lit(float64(2.6))}}, "o")
	if round.ValueAt(0).(float64) != 3 {
		t.Errorf("round = %v, want 3", round.ValueAt(0))
	}
	floor := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnFloor, Args: []spec.CalcExpr{lit(float64(2.9))}}, "o")
	if floor.ValueAt(0).(float64) != 2 {
		t.Errorf("floor = %v, want 2", floor.ValueAt(0))
	}
	ceil := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnCeil, Args: []spec.CalcExpr{lit(float64(2.1))}}, "o")
	if ceil.ValueAt(0).(float64) != 3 {
		t.Errorf("ceil = %v, want 3", ceil.ValueAt(0))
	}
	mn := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnMin, Args: []spec.CalcExpr{fld("revenue"), fld("users")}}, "o")
	if mn.ValueAt(0).(float64) != 10 {
		t.Errorf("min = %v, want 10", mn.ValueAt(0))
	}
	mx := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnMax, Args: []spec.CalcExpr{fld("revenue"), fld("users")}}, "o")
	if mx.ValueAt(0).(float64) != 100 {
		t.Errorf("max = %v, want 100", mx.ValueAt(0))
	}
}

func TestCalcCoalesceNull(t *testing.T) {
	// coalesce(bonus, -1): bonus null at row 1 → falls back to -1.
	col := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnCoalesce, Args: []spec.CalcExpr{fld("bonus"), lit(float64(-1))}}, "o")
	if col.IsNull(1) {
		t.Fatalf("coalesce row 1 should be non-null (-1 fallback)")
	}
	if got := col.ValueAt(1).(float64); got != -1 {
		t.Errorf("coalesce row 1 = %v, want -1", got)
	}
	if col.ValueAt(0).(float64) != 5 {
		t.Errorf("coalesce row 0 = %v, want 5 (bonus)", col.ValueAt(0))
	}
}

func TestCalcMinMaxSkipsNull(t *testing.T) {
	// min(bonus, revenue): row 1 bonus is null → min over remaining
	// non-null operand (revenue==200).
	col := runCalc(t, spec.CalcExpr{Fn: spec.CalcFnMin, Args: []spec.CalcExpr{fld("bonus"), fld("revenue")}}, "o")
	if col.IsNull(1) {
		t.Fatalf("min row 1 should be non-null (skips null bonus)")
	}
	if got := col.ValueAt(1).(float64); got != 200 {
		t.Errorf("min row 1 = %v, want 200", got)
	}
}

func TestCalcConcat(t *testing.T) {
	// concat(region, "-", users) → "NA-10" etc.
	col := runCalc(t, spec.CalcExpr{Concat: []spec.CalcExpr{fld("region"), lit("-"), fld("users")}}, "label")
	if col.Kind() != table.KindString {
		t.Fatalf("concat kind = %v, want string", col.Kind())
	}
	if got := col.ValueAt(0).(string); got != "NA-10" {
		t.Errorf("concat row 0 = %q, want NA-10", got)
	}
	// null operand (bonus at row 1) renders as empty string.
	col2 := runCalc(t, spec.CalcExpr{Concat: []spec.CalcExpr{fld("region"), fld("bonus")}}, "label2")
	if got := col2.ValueAt(1).(string); got != "EU" {
		t.Errorf("concat with null row 1 = %q, want EU", got)
	}
}

func TestCalcCaseFallthrough(t *testing.T) {
	// case: revenue > 150 → 1, revenue > 50 → 2, else 3.
	expr := spec.CalcExpr{
		Case: []spec.CalcBranch{
			{When: spec.Predicate{Op: spec.PredGt, Field: "revenue", Value: float64(150)}, Then: lit(float64(1))},
			{When: spec.Predicate{Op: spec.PredGt, Field: "revenue", Value: float64(50)}, Then: lit(float64(2))},
		},
		Else: ptr(lit(float64(3))),
	}
	col := runCalc(t, expr, "bucket")
	want := []float64{2, 1, 1} // revenue 100,200,300
	for i, w := range want {
		if got := col.ValueAt(i).(float64); got != w {
			t.Errorf("case row %d = %v, want %v", i, got, w)
		}
	}
}

func TestCalcCaseElse(t *testing.T) {
	// No branch matches row 0 → else fires.
	expr := spec.CalcExpr{
		Case: []spec.CalcBranch{
			{When: spec.Predicate{Op: spec.PredGt, Field: "revenue", Value: float64(1000)}, Then: lit(float64(1))},
		},
		Else: ptr(lit(float64(9))),
	}
	col := runCalc(t, expr, "b")
	if col.ValueAt(0).(float64) != 9 {
		t.Errorf("case else row 0 = %v, want 9", col.ValueAt(0))
	}
}

func TestCalcStringCaseOutput(t *testing.T) {
	// A case whose branches yield strings produces a string column.
	expr := spec.CalcExpr{
		Case: []spec.CalcBranch{
			{When: spec.Predicate{Op: spec.PredGt, Field: "revenue", Value: float64(150)}, Then: lit("hi")},
		},
		Else: ptr(lit("lo")),
	}
	col := runCalc(t, expr, "tier")
	if col.Kind() != table.KindString {
		t.Fatalf("case string kind = %v, want string", col.Kind())
	}
	if got := col.ValueAt(0).(string); got != "lo" {
		t.Errorf("row 0 = %q, want lo", got)
	}
	if got := col.ValueAt(1).(string); got != "hi" {
		t.Errorf("row 1 = %q, want hi", got)
	}
}
