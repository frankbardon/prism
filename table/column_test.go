package table

import (
	"testing"

	"github.com/frankbardon/pulse/encoding"
)

func TestKindFromPulseFieldType(t *testing.T) {
	cases := []struct {
		ft   encoding.FieldType
		want Kind
	}{
		{encoding.FieldTypeU8, KindInt},
		{encoding.FieldTypeU64, KindInt},
		{encoding.FieldTypeU4, KindInt},
		{encoding.FieldTypeF32, KindFloat},
		{encoding.FieldTypeF64, KindFloat},
		{encoding.FieldTypeDecimal128, KindFloat},
		{encoding.FieldTypePackedBool, KindBool},
		{encoding.FieldTypeDate, KindDate},
		{encoding.FieldTypeCategoricalU8, KindString},
		{encoding.FieldTypeCategoricalU16, KindString},
		{encoding.FieldTypeCategoricalU32, KindString},
	}
	for _, c := range cases {
		t.Run(c.ft.String(), func(t *testing.T) {
			got := KindFromPulseFieldType(c.ft)
			if got != c.want {
				t.Fatalf("KindFromPulseFieldType(%s) = %s, want %s", c.ft, got, c.want)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	for _, c := range []struct {
		k    Kind
		want string
	}{
		{KindInt, "int"},
		{KindFloat, "float"},
		{KindString, "string"},
		{KindBool, "bool"},
		{KindDate, "date"},
		{KindUnknown, "unknown"},
	} {
		if got := c.k.String(); got != c.want {
			t.Fatalf("Kind(%d).String() = %q, want %q", c.k, got, c.want)
		}
	}
}

func TestColumnRoundTrip(t *testing.T) {
	t.Run("IntColumn", func(t *testing.T) {
		var c Column = IntColumn{1, 2, 3}
		if c.Kind() != KindInt || c.Len() != 3 || c.ValueAt(1).(int64) != 2 {
			t.Fatalf("IntColumn round trip failed: kind=%s len=%d v[1]=%v", c.Kind(), c.Len(), c.ValueAt(1))
		}
	})
	t.Run("FloatColumn", func(t *testing.T) {
		var c Column = FloatColumn{1.5, 2.5}
		if c.Kind() != KindFloat || c.Len() != 2 || c.ValueAt(0).(float64) != 1.5 {
			t.Fatalf("FloatColumn round trip failed")
		}
	})
	t.Run("StringColumn", func(t *testing.T) {
		var c Column = StringColumn{"a", "b"}
		if c.Kind() != KindString || c.ValueAt(1).(string) != "b" {
			t.Fatalf("StringColumn round trip failed")
		}
	})
	t.Run("BoolColumn", func(t *testing.T) {
		var c Column = BoolColumn{true, false}
		if c.Kind() != KindBool || c.ValueAt(0).(bool) != true {
			t.Fatalf("BoolColumn round trip failed")
		}
	})
	t.Run("DateColumn", func(t *testing.T) {
		var c Column = DateColumn{19700, 19701}
		if c.Kind() != KindDate || c.ValueAt(0).(int64) != 19700 {
			t.Fatalf("DateColumn round trip failed")
		}
	})
}
