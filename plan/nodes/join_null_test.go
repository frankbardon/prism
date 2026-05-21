package nodes_test

import (
	"context"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// TestJoinLeftEmitsNullsForUnmatched — a left join should mark the
// unmatched right-side cells as null (not zero), so callers can
// distinguish "no match" from "matched with a zero value".
func TestJoinLeftEmitsNullsForUnmatched(t *testing.T) {
	leftSchema := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "score", Type: encoding.FieldTypeF64},
	)
	left := mkTableFor(t, leftSchema, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "beta", "gamma"),
		"score":    mkFloatCol(0.5, 0.6, 0.7),
	}, 3, "xxh64:leftleftleftleft")

	rightSchema := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	)
	right := mkTableFor(t, rightSchema, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "beta"),
		"label":    mkStrCol("Alpha Inc", "Beta LLC"),
	}, 2, "xxh64:rightrightrightrigh")

	n := nodes.NewJoin("join-null", "L", "R", []string{"brand_id"}, nodes.JoinLeft, 0)
	out, err := n.Execute(context.Background(), []*table.Table{left, right})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	labelCol, ok := out.Column("label")
	if !ok {
		t.Fatal("output missing label column")
	}
	if labelCol.NullCount() != 1 {
		t.Errorf("NullCount(label) = %d, want 1", labelCol.NullCount())
	}
	// Identify gamma's row by brand_id and assert label is null there.
	brand, _ := out.Column("brand_id")
	for i := 0; i < out.NumRows(); i++ {
		if brand.ValueAt(i) == "gamma" {
			if !labelCol.IsNull(i) {
				t.Errorf("row %d (gamma): label IsNull = false; want true", i)
			}
			if labelCol.ValueAt(i) != nil {
				t.Errorf("row %d (gamma): label ValueAt = %v; want nil", i, labelCol.ValueAt(i))
			}
		} else {
			if labelCol.IsNull(i) {
				t.Errorf("row %d (%v): label unexpectedly null", i, brand.ValueAt(i))
			}
		}
	}
}

// TestJoinOuterEmitsNullsBothSides — outer join should mark both
// left-side and right-side cells as null for unmatched rows.
func TestJoinOuterEmitsNullsBothSides(t *testing.T) {
	leftSchema := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "score", Type: encoding.FieldTypeF64},
	)
	left := mkTableFor(t, leftSchema, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "beta"),
		"score":    mkFloatCol(0.5, 0.6),
	}, 2, "xxh64:leftleftleftleft")

	rightSchema := mkSchema(
		encoding.Field{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	)
	right := mkTableFor(t, rightSchema, map[string]table.Column{
		"brand_id": mkStrCol("alpha", "gamma"),
		"label":    mkStrCol("Alpha Inc", "Gamma Co"),
	}, 2, "xxh64:rightrightrightrigh")

	n := nodes.NewJoin("join-outer", "L", "R", []string{"brand_id"}, nodes.JoinOuter, 0)
	out, err := n.Execute(context.Background(), []*table.Table{left, right})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	scoreCol, _ := out.Column("score")
	labelCol, _ := out.Column("label")
	if scoreCol.NullCount() != 1 {
		t.Errorf("score NullCount = %d, want 1 (for gamma row)", scoreCol.NullCount())
	}
	if labelCol.NullCount() != 1 {
		t.Errorf("label NullCount = %d, want 1 (for beta row)", labelCol.NullCount())
	}
}
