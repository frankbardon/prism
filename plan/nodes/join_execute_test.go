package nodes_test

import (
	"context"
	"errors"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

func mkSchema(fields ...encoding.Field) *encoding.Schema {
	return &encoding.Schema{Fields: append([]encoding.Field(nil), fields...)}
}

func mkStrCol(vals ...string) table.Column { return table.StringColumn(vals) }
func mkFloatCol(vals ...float64) table.Column {
	return table.FloatColumn(vals)
}
func mkIntCol(vals ...int64) table.Column { return table.IntColumn(vals) }

func mkTableFor(t *testing.T, schema *encoding.Schema, cols map[string]table.Column, n int, hash string) *table.Table {
	t.Helper()
	tbl, err := table.NewTable(schema, cols, n, hash)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// brandSide produces a 3-row left table with brand_id + score columns.
func brandSide(t *testing.T) *table.Table {
	t.Helper()
	// Use a real categorical type so KindFromPulseFieldType resolves
	// to KindString without dictionary plumbing.
	s := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "score", Type: encoding.FieldTypeF64},
	)
	return mkTableFor(t, s, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "beta", "gamma"),
		"score":    mkFloatCol(0.5, 0.6, 0.7),
	}, 3, "xxh64:leftleftleftleft")
}

// labelSide produces a right table with brand_id + label, missing gamma
// and including a delta with no match on the left.
func labelSide(t *testing.T) *table.Table {
	t.Helper()
	s := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	)
	return mkTableFor(t, s, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "beta", "delta"),
		"label":    mkStrCol("Alpha Inc", "Beta LLC", "Delta Co"),
	}, 3, "xxh64:rightrightrightrigh")
}

func TestPrismJoinTypes(t *testing.T) {
	left := brandSide(t)
	right := labelSide(t)

	cases := []struct {
		kind         nodes.JoinKind
		wantRows     int
		wantBrandSet []string // unique brand_id values, order-independent
		wantLabels   []string // matched labels expected in output
	}{
		{nodes.JoinInner, 2, []string{"alpha", "beta"}, []string{"Alpha Inc", "Beta LLC"}},
		{nodes.JoinLeft, 3, []string{"alpha", "beta", "gamma"}, []string{"Alpha Inc", "Beta LLC"}},
		{nodes.JoinOuter, 4, []string{"alpha", "beta", "gamma", "delta"}, []string{"Alpha Inc", "Beta LLC", "Delta Co"}},
		{nodes.JoinAnti, 1, []string{"gamma"}, nil},
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			n := nodes.NewJoin("join-1", "L", "R", []string{"brand_id"}, tc.kind, 0)
			out, err := n.Execute(context.Background(), []*table.Table{left, right})
			if err != nil {
				t.Fatalf("Execute(%s): %v", tc.kind, err)
			}
			if out.NumRows() != tc.wantRows {
				t.Errorf("rows=%d; want %d", out.NumRows(), tc.wantRows)
			}
			brandCol, ok := out.Column("brand_id")
			if !ok {
				t.Fatalf("output missing brand_id")
			}
			seen := map[string]bool{}
			for i := 0; i < brandCol.Len(); i++ {
				seen[brandCol.ValueAt(i).(string)] = true
			}
			for _, want := range tc.wantBrandSet {
				if !seen[want] {
					t.Errorf("brand_id %q missing; got %v", want, seen)
				}
			}
			if tc.kind != nodes.JoinAnti {
				labelCol, ok := out.Column("label")
				if !ok {
					t.Fatalf("output missing label column")
				}
				seenLabels := map[string]bool{}
				for i := 0; i < labelCol.Len(); i++ {
					seenLabels[labelCol.ValueAt(i).(string)] = true
				}
				for _, want := range tc.wantLabels {
					if !seenLabels[want] {
						t.Errorf("label %q missing; got %v", want, seenLabels)
					}
				}
			} else {
				// Anti join → output schema = left schema only.
				if _, ok := out.Column("label"); ok {
					t.Errorf("anti join output should not include right-only columns")
				}
			}
		})
	}
}

func TestPrismJoinMultiColumnKey(t *testing.T) {
	lSchema := mkSchema(
		encoding.Field{Name: "k1", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "k2", Type: encoding.FieldTypeU8},
		encoding.Field{Name: "lv", Type: encoding.FieldTypeF64},
	)
	rSchema := mkSchema(
		encoding.Field{Name: "k1", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "k2", Type: encoding.FieldTypeU8},
		encoding.Field{Name: "rv", Type: encoding.FieldTypeF64},
	)
	left := mkTableFor(t, lSchema, map[string]table.Column{
		"k1": mkStrCol("a", "a", "b"),
		"k2": mkIntCol(1, 2, 1),
		"lv": mkFloatCol(10, 20, 30),
	}, 3, "xxh64:multi-left-aaaaa")
	right := mkTableFor(t, rSchema, map[string]table.Column{
		"k1": mkStrCol("a", "b", "b"),
		"k2": mkIntCol(2, 1, 2),
		"rv": mkFloatCol(100, 200, 300),
	}, 3, "xxh64:multi-right-aaaa")

	n := nodes.NewJoin("mj", "L", "R", []string{"k1", "k2"}, nodes.JoinInner, 0)
	out, err := n.Execute(context.Background(), []*table.Table{left, right})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.NumRows() != 2 {
		t.Fatalf("rows=%d; want 2 (a,2 and b,1)", out.NumRows())
	}
	// Verify each output row's (lv, rv) pair belongs to the expected set.
	lvCol, _ := out.Column("lv")
	rvCol, _ := out.Column("rv")
	type pair struct{ lv, rv float64 }
	wantPairs := map[pair]bool{
		{20, 100}: false,
		{30, 200}: false,
	}
	for i := 0; i < out.NumRows(); i++ {
		p := pair{lvCol.ValueAt(i).(float64), rvCol.ValueAt(i).(float64)}
		if _, ok := wantPairs[p]; !ok {
			t.Errorf("unexpected pair %+v at row %d", p, i)
			continue
		}
		wantPairs[p] = true
	}
	for p, seen := range wantPairs {
		if !seen {
			t.Errorf("pair %+v missing from output", p)
		}
	}
}

func TestPrismJoinCardinalityLimit(t *testing.T) {
	t.Setenv("PRISM_JOIN_MAX_ROWS", "10")
	lSchema := mkSchema(
		encoding.Field{Name: "k", Type: encoding.FieldTypeU8},
	)
	rSchema := mkSchema(
		encoding.Field{Name: "k", Type: encoding.FieldTypeU8},
	)
	left := mkTableFor(t, lSchema, map[string]table.Column{
		"k": mkIntCol(1, 2, 3),
	}, 3, "xxh64:left-card-aaaaaaa")
	right := mkTableFor(t, rSchema, map[string]table.Column{
		"k": mkIntCol(1, 1, 1, 1),
	}, 4, "xxh64:right-card-aaaaaa")

	n := nodes.NewJoin("card", "L", "R", []string{"k"}, nodes.JoinInner, 0)
	_, err := n.Execute(context.Background(), []*table.Table{left, right})
	if err == nil {
		t.Fatal("expected PRISM_JOIN_003, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_JOIN_003" {
		t.Fatalf("got %v; want PRISM_JOIN_003", err)
	}
	if _, ok := ae.Context["Actual"]; !ok {
		t.Errorf("error context missing Actual: %v", ae.Context)
	}
	if _, ok := ae.Context["Limit"]; !ok {
		t.Errorf("error context missing Limit: %v", ae.Context)
	}
}

func TestPrismJoinKeyTypeMismatch(t *testing.T) {
	lSchema := mkSchema(
		encoding.Field{Name: "k", Type: encoding.FieldTypeF64},
	)
	rSchema := mkSchema(
		encoding.Field{Name: "k", Type: encoding.FieldTypeCategoricalU8},
	)
	left := mkTableFor(t, lSchema, map[string]table.Column{
		"k": mkFloatCol(1, 2),
	}, 2, "xxh64:tm-left-aaaaaaaaa")
	right := mkTableFor(t, rSchema, map[string]table.Column{
		"k": mkStrCol("a", "b"),
	}, 2, "xxh64:tm-right-aaaaaaaa")

	n := nodes.NewJoin("tm", "L", "R", []string{"k"}, nodes.JoinInner, 0)
	_, err := n.Execute(context.Background(), []*table.Table{left, right})
	if err == nil {
		t.Fatal("expected PRISM_JOIN_001, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_JOIN_001" {
		t.Fatalf("got %v; want PRISM_JOIN_001", err)
	}
}

func TestPrismJoinKeyAbsent(t *testing.T) {
	lSchema := mkSchema(
		encoding.Field{Name: "k", Type: encoding.FieldTypeU8},
	)
	rSchema := mkSchema(
		encoding.Field{Name: "j", Type: encoding.FieldTypeU8},
	)
	left := mkTableFor(t, lSchema, map[string]table.Column{
		"k": mkIntCol(1),
	}, 1, "xxh64:ka-left-aaaaaaaaa")
	right := mkTableFor(t, rSchema, map[string]table.Column{
		"j": mkIntCol(2),
	}, 1, "xxh64:ka-right-aaaaaaaa")

	n := nodes.NewJoin("ka", "L", "R", []string{"k"}, nodes.JoinInner, 0)
	_, err := n.Execute(context.Background(), []*table.Table{left, right})
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_JOIN_002" {
		t.Fatalf("got %v; want PRISM_JOIN_002", err)
	}
	if side, _ := ae.Context["Side"].(string); side != "right" {
		t.Errorf("Side context=%v; want 'right'", ae.Context["Side"])
	}
}

func TestPrismJoinOutputSchemaOrder(t *testing.T) {
	left := brandSide(t)
	right := labelSide(t)

	n := nodes.NewJoin("os", "L", "R", []string{"brand_id"}, nodes.JoinInner, 0)
	out, err := n.Execute(context.Background(), []*table.Table{left, right})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []string{"brand_id", "score", "label"}
	got := out.FieldNames()
	if len(got) != len(want) {
		t.Fatalf("field count=%d; want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field[%d]=%q; want %q", i, got[i], want[i])
		}
	}
}
