package table

import (
	"testing"

	"github.com/frankbardon/pulse/encoding"
)

// TestKindFromFieldType covers every native FieldType, including the ones
// that fold to KindUnknown (set types), so the mapping stays total.
func TestKindFromFieldType(t *testing.T) {
	cases := []struct {
		ft   FieldType
		want Kind
	}{
		{FieldTypeU8, KindInt},
		{FieldTypeU16, KindInt},
		{FieldTypeU32, KindInt},
		{FieldTypeU64, KindInt},
		{FieldTypeU4, KindInt},
		{FieldTypeF32, KindFloat},
		{FieldTypeF64, KindFloat},
		{FieldTypeDecimal128, KindFloat},
		{FieldTypeDate, KindDate},
		{FieldTypePackedBool, KindBool},
		{FieldTypeCategoricalU8, KindString},
		{FieldTypeCategoricalU16, KindString},
		{FieldTypeCategoricalU32, KindString},
		{FieldTypeSetU8, KindUnknown},
		{FieldTypeSetU16, KindUnknown},
		{FieldTypeSetU32, KindUnknown},
		{FieldTypeSetU64, KindUnknown},
	}
	for _, c := range cases {
		t.Run(c.ft.String(), func(t *testing.T) {
			if got := KindFromFieldType(c.ft); got != c.want {
				t.Fatalf("KindFromFieldType(%s) = %s, want %s", c.ft, got, c.want)
			}
		})
	}
}

// TestKindFromFieldTypeMatchesPulse asserts the native kind mapping agrees
// with the legacy Pulse-backed one for every type both understand. This is
// the invariant the E1-S2/E1-S3 migration relies on. Removed with the shim
// in E4.
func TestKindFromFieldTypeMatchesPulse(t *testing.T) {
	pulseTypes := []encoding.FieldType{
		encoding.FieldTypeU8, encoding.FieldTypeU16, encoding.FieldTypeU32,
		encoding.FieldTypeU64, encoding.FieldTypeF32, encoding.FieldTypeF64,
		encoding.FieldTypeU4, encoding.FieldTypeDate, encoding.FieldTypePackedBool,
		encoding.FieldTypeCategoricalU8, encoding.FieldTypeCategoricalU16,
		encoding.FieldTypeCategoricalU32, encoding.FieldTypeDecimal128,
	}
	for _, pt := range pulseTypes {
		nt := FromPulseFieldType(pt)
		if got, want := KindFromFieldType(nt), KindFromPulseFieldType(pt); got != want {
			t.Fatalf("kind mismatch for %s: native=%s pulse=%s", pt, got, want)
		}
		if nt.String() != pt.String() {
			t.Fatalf("String mismatch: native=%q pulse=%q", nt.String(), pt.String())
		}
	}
}

func TestFieldTypePredicates(t *testing.T) {
	if !FieldTypeF64.IsNumeric() || FieldTypeDate.IsNumeric() || FieldTypeCategoricalU8.IsNumeric() {
		t.Fatal("IsNumeric predicate wrong")
	}
	if !FieldTypeDecimal128.IsDecimal() || FieldTypeF64.IsDecimal() {
		t.Fatal("IsDecimal predicate wrong")
	}
	if !FieldTypeCategoricalU16.IsCategorical() || FieldTypeSetU8.IsCategorical() {
		t.Fatal("IsCategorical predicate wrong")
	}
	if !FieldTypePackedBool.IsBitPacked() || !FieldTypeU4.IsBitPacked() || FieldTypeU8.IsBitPacked() {
		t.Fatal("IsBitPacked predicate wrong")
	}
	if !FieldTypeSetU64.IsSet() || FieldTypeCategoricalU8.IsSet() {
		t.Fatal("IsSet predicate wrong")
	}
	if !FieldTypeCategoricalU8.HasDictionary() || !FieldTypeSetU8.HasDictionary() || FieldTypeF64.HasDictionary() {
		t.Fatal("HasDictionary predicate wrong")
	}
	if !FieldTypeSetU64.IsKnown() || fieldTypeCount.IsKnown() {
		t.Fatal("IsKnown predicate wrong")
	}
}

func TestDictionaryRoundTrip(t *testing.T) {
	d := NewDictionary()
	a, _ := d.Add("alpha")
	b, _ := d.Add("beta")
	again, _ := d.Add("alpha")
	if a != 0 || b != 1 || again != 0 {
		t.Fatalf("Add IDs wrong: alpha=%d beta=%d alpha2=%d", a, b, again)
	}
	if d.Count() != 2 {
		t.Fatalf("Count = %d, want 2", d.Count())
	}
	if d.Resolve(1) != "beta" || d.Resolve(99) != "" {
		t.Fatalf("Resolve wrong: 1=%q 99=%q", d.Resolve(1), d.Resolve(99))
	}
	if id, ok := d.IDFor("beta"); !ok || id != 1 {
		t.Fatalf("IDFor(beta) = %d,%v", id, ok)
	}
	if _, ok := d.IDFor("missing"); ok {
		t.Fatal("IDFor(missing) reported present")
	}
	if got := d.Values(); len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("Values = %v", got)
	}
}

func TestSchemaFieldLookup(t *testing.T) {
	s := &Schema{Fields: []Field{
		{Name: "id", Type: FieldTypeU64},
		{Name: "country", Type: FieldTypeCategoricalU8, Dictionary: NewDictionary()},
	}}
	if f := s.Field("country"); f == nil || f.Type != FieldTypeCategoricalU8 {
		t.Fatalf("Field(country) = %+v", f)
	}
	if f := s.Field("missing"); f != nil {
		t.Fatalf("Field(missing) = %+v, want nil", f)
	}
	if _, ok := s.Categorical("country"); !ok {
		t.Fatal("Categorical(country) not found")
	}
	if _, ok := s.Categorical("id"); ok {
		t.Fatal("Categorical(id) should be false for a numeric field")
	}
	var nilSchema *Schema
	if nilSchema.Field("x") != nil {
		t.Fatal("nil-schema Field should return nil")
	}
}

// TestSchemaRoundTrip exercises the temporary Pulse conversion shims: a
// native schema survives native -> Pulse -> native unchanged, including the
// categorical dictionary. Removed with the shim in E4.
func TestSchemaRoundTrip(t *testing.T) {
	dict := NewDictionary()
	_, _ = dict.Add("US")
	_, _ = dict.Add("CA")

	orig := &Schema{Fields: []Field{
		{Name: "id", Type: FieldTypeU64, Nullable: false},
		{Name: "amount", Type: FieldTypeDecimal128, Precision: 18, Scale: 2, Nullable: true},
		{Name: "country", Type: FieldTypeCategoricalU8, Dictionary: dict, Description: "iso code"},
	}}

	got := FromPulseSchema(ToPulseSchema(orig))
	if len(got.Fields) != len(orig.Fields) {
		t.Fatalf("field count %d, want %d", len(got.Fields), len(orig.Fields))
	}
	for i := range orig.Fields {
		of, gf := orig.Fields[i], got.Fields[i]
		if of.Name != gf.Name || of.Type != gf.Type || of.Nullable != gf.Nullable ||
			of.Precision != gf.Precision || of.Scale != gf.Scale || of.Description != gf.Description {
			t.Fatalf("field %d mismatch: orig=%+v got=%+v", i, of, gf)
		}
	}
	rt, ok := got.Categorical("country")
	if !ok {
		t.Fatal("round-tripped country lost its dictionary")
	}
	if v := rt.Values(); len(v) != 2 || v[0] != "US" || v[1] != "CA" {
		t.Fatalf("round-tripped dictionary values = %v", v)
	}

	if FromPulseSchema(nil) != nil || ToPulseSchema(nil) != nil {
		t.Fatal("nil schema conversion should stay nil")
	}
}
