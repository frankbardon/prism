package nodes_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

func unionLeftSchema() *encoding.Schema {
	return mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "score", Type: encoding.FieldTypeF64},
	)
}

func TestPrismUnionConcat(t *testing.T) {
	s := unionLeftSchema()
	t1 := mkTableFor(t, s, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "beta"),
		"score":    mkFloatCol(0.5, 0.6),
	}, 2, "xxh64:union-1aaaaaaaa")
	t2 := mkTableFor(t, s, map[string]table.Column{
		"brand_id": mkStrCol("gamma", "delta", "epsilon"),
		"score":    mkFloatCol(0.7, 0.8, 0.9),
	}, 3, "xxh64:union-2aaaaaaaa")
	t3 := mkTableFor(t, s, map[string]table.Column{
		"brand_id": mkStrCol("zeta"),
		"score":    mkFloatCol(1.0),
	}, 1, "xxh64:union-3aaaaaaaa")

	n := nodes.NewUnion("u1", []plan.NodeID{"a", "b", "c"})
	out, err := n.Execute(context.Background(), []*table.Table{t1, t2, t3})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.NumRows() != 6 {
		t.Errorf("rows=%d; want 6", out.NumRows())
	}
	brandCol, _ := out.Column("brand_id")
	want := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	for i, w := range want {
		got := brandCol.ValueAt(i).(string)
		if got != w {
			t.Errorf("row %d brand_id=%q; want %q", i, got, w)
		}
	}
}

func TestPrismUnionSchemaMismatch(t *testing.T) {
	a := unionLeftSchema()
	b := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "weight", Type: encoding.FieldTypeF64}, // different name
	)
	t1 := mkTableFor(t, a, map[string]table.Column{
		"brand_id": mkStrCol("x"),
		"score":    mkFloatCol(1.0),
	}, 1, "xxh64:union-mm1aaaaaaa")
	t2 := mkTableFor(t, b, map[string]table.Column{
		"brand_id": mkStrCol("y"),
		"weight":   mkFloatCol(2.0),
	}, 1, "xxh64:union-mm2aaaaaaa")

	n := nodes.NewUnion("um", []plan.NodeID{"a", "b"})
	_, err := n.Execute(context.Background(), []*table.Table{t1, t2})
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_004" {
		t.Fatalf("got %v; want PRISM_PLAN_004", err)
	}
	diff, _ := ae.Context["Diff"].(string)
	if !strings.Contains(diff, "input[1]") {
		t.Errorf("Diff %q should mention input[1]", diff)
	}
	if !strings.Contains(diff, "weight") {
		t.Errorf("Diff %q should mention the differing field name", diff)
	}
}

func TestPrismUnionFieldCountMismatch(t *testing.T) {
	a := unionLeftSchema()
	b := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
	)
	t1 := mkTableFor(t, a, map[string]table.Column{
		"brand_id": mkStrCol("x"),
		"score":    mkFloatCol(1.0),
	}, 1, "xxh64:union-fc1aaaaaaa")
	t2 := mkTableFor(t, b, map[string]table.Column{
		"brand_id": mkStrCol("y"),
	}, 1, "xxh64:union-fc2aaaaaaa")

	n := nodes.NewUnion("uf", []plan.NodeID{"a", "b"})
	_, err := n.Execute(context.Background(), []*table.Table{t1, t2})
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_004" {
		t.Fatalf("got %v; want PRISM_PLAN_004", err)
	}
	if !strings.Contains(ae.Context["Diff"].(string), "field-count") {
		t.Errorf("Diff should mention field-count: %v", ae.Context["Diff"])
	}
}

func TestPrismUnionSingleInput(t *testing.T) {
	s := unionLeftSchema()
	t1 := mkTableFor(t, s, map[string]table.Column{
		"brand_id": mkStrCol("alpha"),
		"score":    mkFloatCol(0.5),
	}, 1, "xxh64:union-single-aaaa")
	n := nodes.NewUnion("us", []plan.NodeID{"only"})
	out, err := n.Execute(context.Background(), []*table.Table{t1})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.NumRows() != 1 {
		t.Errorf("rows=%d; want 1", out.NumRows())
	}
}
