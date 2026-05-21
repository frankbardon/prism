package table

import "testing"

func TestNullBitmapSetIsNullCount(t *testing.T) {
	b := NewNullBitmap(8)
	if b.Count() != 0 {
		t.Fatalf("initial count = %d, want 0", b.Count())
	}
	b.Set(0)
	b.Set(3)
	b.Set(7)
	if !b.IsNull(0) || !b.IsNull(3) || !b.IsNull(7) {
		t.Fatalf("Set positions not marked: %+v", b)
	}
	if b.IsNull(1) || b.IsNull(2) {
		t.Fatalf("unset positions reported null")
	}
	if b.Count() != 3 {
		t.Fatalf("Count = %d, want 3", b.Count())
	}
	// Idempotent set
	b.Set(3)
	if b.Count() != 3 {
		t.Fatalf("re-Set bumped Count to %d", b.Count())
	}
}

func TestNullBitmapGrows(t *testing.T) {
	b := NewNullBitmap(0)
	b.Set(127)
	if !b.IsNull(127) {
		t.Fatal("Set(127) not marked after growth")
	}
	if b.Capacity() < 128 {
		t.Fatalf("Capacity = %d, want >= 128", b.Capacity())
	}
}

func TestNilBitmapSafe(t *testing.T) {
	var b *NullBitmap
	if b.IsNull(0) {
		t.Error("nil bitmap should not report null")
	}
	if b.Count() != 0 {
		t.Error("nil bitmap Count != 0")
	}
	if b.Capacity() != 0 {
		t.Error("nil bitmap Capacity != 0")
	}
	b.Set(0) // no-op, no panic
}

func TestNullableColumnTransparentWhenNoBitmap(t *testing.T) {
	inner := IntColumn{1, 2, 3}
	var col Column = NullableColumn{Inner: inner}
	if col.Len() != 3 {
		t.Errorf("Len = %d", col.Len())
	}
	if col.Kind() != KindInt {
		t.Errorf("Kind = %s", col.Kind())
	}
	if v := col.ValueAt(1); v != int64(2) {
		t.Errorf("ValueAt(1) = %v", v)
	}
	if col.IsNull(1) {
		t.Error("IsNull(1) = true for non-nullable column")
	}
	if col.NullCount() != 0 {
		t.Errorf("NullCount = %d", col.NullCount())
	}
}

func TestNullableColumnHidesNullValues(t *testing.T) {
	inner := IntColumn{10, 20, 30}
	nulls := NewNullBitmap(3)
	nulls.Set(1)
	var col Column = NullableColumn{Inner: inner, Nulls: nulls}
	if !col.IsNull(1) {
		t.Error("IsNull(1) = false; want true")
	}
	if col.ValueAt(1) != nil {
		t.Errorf("ValueAt(1) = %v, want nil", col.ValueAt(1))
	}
	if col.ValueAt(0) != int64(10) {
		t.Errorf("ValueAt(0) = %v, want 10", col.ValueAt(0))
	}
	if col.NullCount() != 1 {
		t.Errorf("NullCount = %d, want 1", col.NullCount())
	}
}

func TestPlainColumnsReportNoNulls(t *testing.T) {
	cols := []Column{
		IntColumn{1, 2, 3},
		FloatColumn{1.0, 2.0},
		StringColumn{"a", "b"},
		BoolColumn{true, false},
		DateColumn{0, 1, 2},
	}
	for _, c := range cols {
		if c.NullCount() != 0 {
			t.Errorf("%s: NullCount = %d, want 0", c.Kind(), c.NullCount())
		}
		for i := 0; i < c.Len(); i++ {
			if c.IsNull(i) {
				t.Errorf("%s row %d: IsNull = true", c.Kind(), i)
			}
		}
	}
}
