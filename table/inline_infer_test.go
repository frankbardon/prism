package table

import (
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

// TestPrismInlineInferSkipsLeadingNull — a leading gap in a series must
// not decide the column's kind. `v` is null in row 0 and numeric after,
// so the column is a float column with row 0 flagged null.
func TestPrismInlineInferSkipsLeadingNull(t *testing.T) {
	rows := []map[string]any{
		{"t": 1.0, "v": nil},
		{"t": 2.0, "v": 110.0},
		{"t": 3.0, "v": 120.0},
	}
	tbl, schema, err := FromInline("series", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	if got := schema.Field("v").Type; got != FieldTypeF64 {
		t.Fatalf("v FieldType = %s, want %s", got, FieldTypeF64)
	}
	col, ok := tbl.Column("v")
	if !ok {
		t.Fatal("v column missing")
	}
	if col.Kind() != KindFloat {
		t.Fatalf("v Kind = %v, want KindFloat", col.Kind())
	}
	if col.NullCount() != 1 || !col.IsNull(0) {
		t.Fatalf("NullCount = %d IsNull(0) = %v, want 1 / true", col.NullCount(), col.IsNull(0))
	}
	if col.ValueAt(1) != 110.0 || col.ValueAt(2) != 120.0 {
		t.Fatalf("values = %v/%v, want 110/120", col.ValueAt(1), col.ValueAt(2))
	}
}

// TestPrismInlineInferSkipsLeadingNullRun — the scan continues past a
// run of nulls, not just a single leading one, and past a row that omits
// the key entirely.
func TestPrismInlineInferSkipsLeadingNullRun(t *testing.T) {
	rows := []map[string]any{
		{"v": nil},
		{},
		{"v": nil},
		{"v": true},
	}
	tbl, _, err := FromInline("flags", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	col, _ := tbl.Column("v")
	if col.Kind() != KindBool {
		t.Fatalf("v Kind = %v, want KindBool", col.Kind())
	}
	if col.NullCount() != 3 {
		t.Fatalf("NullCount = %d, want 3", col.NullCount())
	}
	if col.ValueAt(3) != true {
		t.Fatalf("ValueAt(3) = %v, want true", col.ValueAt(3))
	}
}

// TestPrismInlineInferAllNullColumn — a column with no non-null value
// anywhere has nothing to infer from. It resolves to the categorical
// fallback rather than erroring, and every row is flagged null so the
// chosen kind never has to hold a value.
func TestPrismInlineInferAllNullColumn(t *testing.T) {
	rows := []map[string]any{
		{"t": 1.0, "v": nil},
		{"t": 2.0, "v": nil},
		{"t": 3.0},
	}
	tbl, schema, err := FromInline("series", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	if got := schema.Field("v").Type; got != FieldTypeCategoricalU8 {
		t.Fatalf("v FieldType = %s, want %s", got, FieldTypeCategoricalU8)
	}
	col, ok := tbl.Column("v")
	if !ok {
		t.Fatal("v column missing")
	}
	if col.NullCount() != 3 {
		t.Fatalf("NullCount = %d, want 3", col.NullCount())
	}
	for i := range rows {
		if !col.IsNull(i) {
			t.Fatalf("row %d IsNull = false, want true", i)
		}
		if col.ValueAt(i) != nil {
			t.Fatalf("ValueAt(%d) = %v, want nil", i, col.ValueAt(i))
		}
	}
}

// TestPrismInlineInferFieldMissingFromFirstRow — a field introduced by a
// later row still becomes a column, in the same alphabetical slot it
// would have occupied had row 0 carried it.
func TestPrismInlineInferFieldMissingFromFirstRow(t *testing.T) {
	rows := []map[string]any{
		{"a": 1.0, "c": "x"},
		{"a": 2.0, "b": 9.0, "c": "y"},
	}
	tbl, _, err := FromInline("ds", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	got := tbl.FieldNames()
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("FieldNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FieldNames = %v, want %v", got, want)
		}
	}
	col, ok := tbl.Column("b")
	if !ok {
		t.Fatal("b column missing")
	}
	if col.Kind() != KindFloat {
		t.Fatalf("b Kind = %v, want KindFloat", col.Kind())
	}
	if !col.IsNull(0) {
		t.Error("b row 0 should be null — the row did not supply the key")
	}
	if col.ValueAt(1) != 9.0 {
		t.Fatalf("b ValueAt(1) = %v, want 9", col.ValueAt(1))
	}
}

// TestPrismInlineInferOrderIndependentOfIntroducingRow — the column
// order must not depend on which row first mentioned a key.
func TestPrismInlineInferOrderIndependentOfIntroducingRow(t *testing.T) {
	forward := []map[string]any{
		{"a": 1.0},
		{"z": 2.0, "m": 3.0},
	}
	reverse := []map[string]any{
		{"z": 2.0},
		{"m": 3.0, "a": 1.0},
	}
	fwd, _, err := FromInline("ds", forward, nil)
	if err != nil {
		t.Fatalf("FromInline forward: %v", err)
	}
	rev, _, err := FromInline("ds", reverse, nil)
	if err != nil {
		t.Fatalf("FromInline reverse: %v", err)
	}
	for i, name := range []string{"a", "m", "z"} {
		if fwd.FieldNames()[i] != name || rev.FieldNames()[i] != name {
			t.Fatalf("field order %v / %v, want [a m z]", fwd.FieldNames(), rev.FieldNames())
		}
	}
}

// TestPrismInlineInferGenuineMismatchStillErrors — narrowing the
// false positive must not disable the check: a real string arriving in a
// column inferred as float still raises
// PRISM_RESOLVE_INLINE_TYPE_MISMATCH, including when the inference
// itself had to scan past a leading null to reach the float.
func TestPrismInlineInferGenuineMismatchStillErrors(t *testing.T) {
	cases := map[string][]map[string]any{
		"leading null then float then string": {
			{"v": nil},
			{"v": 110.0},
			{"v": "not a number"},
		},
		"float then string": {
			{"v": 110.0},
			{"v": "not a number"},
		},
		"leading null then string then float": {
			{"v": nil},
			{"v": "alpha"},
			{"v": 42.0},
		},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := FromInline("ds", rows, nil)
			if err == nil {
				t.Fatal("expected PRISM_RESOLVE_INLINE_TYPE_MISMATCH, got nil")
			}
			ae, ok := err.(*prismerrors.AppError)
			if !ok {
				t.Fatalf("expected *AppError, got %T", err)
			}
			if ae.Code != "PRISM_RESOLVE_INLINE_TYPE_MISMATCH" {
				t.Fatalf("Code = %s, want PRISM_RESOLVE_INLINE_TYPE_MISMATCH", ae.Code)
			}
		})
	}
}

// TestPrismInlineInferDeclarationWinsOverNulls — an explicit
// `data.fields` declaration bypasses inference entirely, nulls or not.
func TestPrismInlineInferDeclarationWinsOverNulls(t *testing.T) {
	rows := []map[string]any{
		{"v": nil},
		{"v": 110.0},
	}
	fields := []spec.FieldSpec{{Name: "v", Type: "float"}}
	tbl, schema, err := FromInline("ds", rows, fields)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	if got := schema.Field("v").Type; got != FieldTypeF64 {
		t.Fatalf("v FieldType = %s, want %s", got, FieldTypeF64)
	}
	col, _ := tbl.Column("v")
	if col.Kind() != KindFloat {
		t.Fatalf("v Kind = %v, want KindFloat", col.Kind())
	}

	// A string declaration still wins even though every value is numeric.
	strFields := []spec.FieldSpec{{Name: "v", Type: "string"}}
	_, strSchema, err := FromInline("ds", []map[string]any{{"v": nil}, {"v": "a"}}, strFields)
	if err != nil {
		t.Fatalf("FromInline string decl: %v", err)
	}
	if got := strSchema.Field("v").Type; got != FieldTypeCategoricalU8 {
		t.Fatalf("v FieldType = %s, want %s", got, FieldTypeCategoricalU8)
	}
}
