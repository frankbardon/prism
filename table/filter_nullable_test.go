package table

import "testing"

// TestPrismFilterReindexesNullableColumns: before the encoder's null
// policy used Filter, no caller ever handed it a NullableColumn — the
// wrapper fell through filterColumn's identity return, kept its full
// length, and NewTable rejected the result. Both halves of the
// wrapper (values and bitmap) must be re-indexed together.
func TestPrismFilterReindexesNullableColumns(t *testing.T) {
	schema := &Schema{Fields: []Field{
		{Name: "label", Type: FieldTypeCategoricalU8},
		{Name: "score", Type: FieldTypeF64, Nullable: true},
	}}
	nulls := NewNullBitmap(4)
	nulls.Set(1)
	nulls.Set(2)
	cols := map[string]Column{
		"label": StringColumn{"w", "x", "y", "z"},
		"score": NullableColumn{Inner: FloatColumn{1, 2, 3, 4}, Nulls: nulls},
	}
	src, err := NewTable(schema, cols, 4, "xxh64:000000000000000f")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	// Keep rows 0 (non-null), 2 (null) and 3 (non-null).
	out, err := Filter(src, []bool{true, false, true, true}, "test")
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if out.NumRows() != 3 {
		t.Fatalf("NumRows = %d, want 3", out.NumRows())
	}
	score, ok := out.Column("score")
	if !ok {
		t.Fatal("score column missing")
	}
	if score.Len() != 3 {
		t.Errorf("score.Len = %d, want 3", score.Len())
	}
	// Source row 2 was the only surviving null; it now sits at index 1.
	wantNull := []bool{false, true, false}
	for i, want := range wantNull {
		if got := score.IsNull(i); got != want {
			t.Errorf("IsNull(%d) = %v, want %v", i, got, want)
		}
	}
	if score.NullCount() != 1 {
		t.Errorf("NullCount = %d, want 1", score.NullCount())
	}
	if score.ValueAt(0) != 1.0 || score.ValueAt(2) != 4.0 {
		t.Errorf("values = %v/%v, want 1/4", score.ValueAt(0), score.ValueAt(2))
	}
}
