package table

import "testing"

// TestFromInlineJSONNullSetsBitmap — when a row carries `null` for a
// field, FromInline should wrap the resulting column in a
// NullableColumn whose bitmap marks the row. ValueAt then returns nil
// for the marked row while plain rows return their parsed value.
func TestFromInlineJSONNullSetsBitmap(t *testing.T) {
	rows := []map[string]any{
		{"region": "west", "score": 0.42},
		{"region": "east", "score": nil},
		{"region": "north", "score": 0.91},
	}
	tbl, _, err := FromInline("scores", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	col, ok := tbl.Column("score")
	if !ok {
		t.Fatal("score column missing")
	}
	if col.NullCount() != 1 {
		t.Errorf("NullCount = %d, want 1", col.NullCount())
	}
	if !col.IsNull(1) {
		t.Error("row 1 IsNull = false; want true")
	}
	if col.IsNull(0) || col.IsNull(2) {
		t.Error("non-null rows reported as null")
	}
	if col.ValueAt(1) != nil {
		t.Errorf("ValueAt(1) = %v, want nil", col.ValueAt(1))
	}
	if col.ValueAt(0) != 0.42 {
		t.Errorf("ValueAt(0) = %v, want 0.42", col.ValueAt(0))
	}
}

// TestFromInlineMissingKeyTreatedAsNull — when a row omits a field that
// other rows supply, the missing position should be marked null.
func TestFromInlineMissingKeyTreatedAsNull(t *testing.T) {
	rows := []map[string]any{
		{"region": "west", "score": 0.42},
		{"region": "east"},
	}
	tbl, _, err := FromInline("scores", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	col, _ := tbl.Column("score")
	if !col.IsNull(1) {
		t.Error("missing key row not marked null")
	}
}

// TestFromInlineNoNullsLeavesPlainColumn — a fully-populated table
// should not be wrapped; plain slice-backed columns minimise the
// overhead path.
func TestFromInlineNoNullsLeavesPlainColumn(t *testing.T) {
	rows := []map[string]any{
		{"region": "west", "score": 0.42},
		{"region": "east", "score": 0.91},
	}
	tbl, _, err := FromInline("scores", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	col, _ := tbl.Column("score")
	if _, isNullable := col.(NullableColumn); isNullable {
		t.Error("fully-populated column unnecessarily wrapped in NullableColumn")
	}
}
