package table

import "fmt"

// This file defines Prism's native schema vocabulary — Schema, Field,
// FieldType, and Dictionary — the Pulse-free replacement for
// github.com/frankbardon/pulse/encoding.{Schema,Field,FieldType,Dictionary}.
//
// It carries no Pulse import: the type definitions here are the anchor of
// the Pulse-eviction effort (epic E1). The FieldType constant block mirrors
// Pulse's byte layout (identical iota order) so the temporary conversion
// shims in pulse_shim.go are a direct cast and so migrated call sites keep
// the same constant names (encoding.FieldTypeF64 -> table.FieldTypeF64).
//
// Only the predicate surface Prism actually consults is exposed
// (IsNumeric / IsCategorical / IsDecimal / IsBitPacked / IsSet / IsKnown /
// HasDictionary), plus the categorical Dictionary representation the
// inline loader and record-reader converters need.

// FieldType identifies the storage type of a schema field. The byte
// values match Pulse's encoding.FieldType one-for-one; do not reorder.
type FieldType byte

// All field types Prism understands. Nullability is orthogonal to type —
// a field is marked nullable via Field.Nullable and the null state lives
// in the per-record null bitmap (see null_bitmap.go).
const (
	FieldTypeU8             FieldType = iota // 0
	FieldTypeU16                             // 1
	FieldTypeU32                             // 2
	FieldTypeU64                             // 3
	FieldTypeF32                             // 4
	FieldTypeF64                             // 5
	FieldTypeU4                              // 6  (4-bit, bit-packed)
	FieldTypeDate                            // 7
	FieldTypePackedBool                      // 8  (1-bit, bit-packed)
	FieldTypeCategoricalU8                   // 9
	FieldTypeCategoricalU16                  // 10
	FieldTypeCategoricalU32                  // 11
	FieldTypeDecimal128                      // 12
	FieldTypeSetU8                           // 13 (bitmask over shared dict, ≤8 members)
	FieldTypeSetU16                          // 14 (≤16 members)
	FieldTypeSetU32                          // 15 (≤32 members)
	FieldTypeSetU64                          // 16 (≤64 members)

	fieldTypeCount // sentinel; keep last
)

// String returns the snake-case name of the field type. Matches Pulse's
// encoding.FieldType.String so cross-boundary error context stays stable.
func (ft FieldType) String() string {
	switch ft {
	case FieldTypeU8:
		return "u8"
	case FieldTypeU16:
		return "u16"
	case FieldTypeU32:
		return "u32"
	case FieldTypeU64:
		return "u64"
	case FieldTypeF32:
		return "f32"
	case FieldTypeF64:
		return "f64"
	case FieldTypeU4:
		return "u4"
	case FieldTypeDate:
		return "date"
	case FieldTypePackedBool:
		return "packed_bool"
	case FieldTypeCategoricalU8:
		return "categorical_u8"
	case FieldTypeCategoricalU16:
		return "categorical_u16"
	case FieldTypeCategoricalU32:
		return "categorical_u32"
	case FieldTypeDecimal128:
		return "decimal128"
	case FieldTypeSetU8:
		return "set_u8"
	case FieldTypeSetU16:
		return "set_u16"
	case FieldTypeSetU32:
		return "set_u32"
	case FieldTypeSetU64:
		return "set_u64"
	default:
		return fmt.Sprintf("unknown(%d)", ft)
	}
}

// IsNumeric reports whether the field type is a strict scalar number: the
// unsigned-integer family (u8/u16/u32/u64), the float family (f32/f64),
// and decimal128. Date and bit-packed encodings are excluded. Mirrors
// encoding.FieldType.IsNumeric; consulted by validate/measureTypeFor and
// scale/aggregate selection.
func (ft FieldType) IsNumeric() bool {
	if ft.IsDecimal() {
		return true
	}
	switch ft {
	case FieldTypeU8, FieldTypeU16, FieldTypeU32, FieldTypeU64,
		FieldTypeF32, FieldTypeF64:
		return true
	}
	return false
}

// IsCategorical reports whether the field type is one of the dictionary-
// backed categorical types.
func (ft FieldType) IsCategorical() bool {
	return ft == FieldTypeCategoricalU8 ||
		ft == FieldTypeCategoricalU16 ||
		ft == FieldTypeCategoricalU32
}

// IsDecimal reports whether the field type is decimal128.
func (ft FieldType) IsDecimal() bool {
	return ft == FieldTypeDecimal128
}

// IsBitPacked reports whether the field type shares its on-wire byte with
// adjacent fields via bit packing (u4, packed_bool).
func (ft FieldType) IsBitPacked() bool {
	return ft == FieldTypeU4 || ft == FieldTypePackedBool
}

// IsSet reports whether the field type is a bitmask-over-dictionary set
// (multi-select) type.
func (ft FieldType) IsSet() bool {
	switch ft {
	case FieldTypeSetU8, FieldTypeSetU16, FieldTypeSetU32, FieldTypeSetU64:
		return true
	}
	return false
}

// HasDictionary reports whether the field type carries a categorical
// dictionary (categorical or set types).
func (ft FieldType) HasDictionary() bool {
	return ft.IsCategorical() || ft.IsSet()
}

// IsKnown reports whether the byte value corresponds to a registered type.
func (ft FieldType) IsKnown() bool {
	return ft < fieldTypeCount
}

// Field describes a single column in a Prism schema.
type Field struct {
	// Name is the column name (spec field key).
	Name string
	// Type is the storage type bucketed to a Kind by KindFromFieldType.
	Type FieldType
	// Nullable is true when the field participates in the per-record null
	// bitmap.
	Nullable bool
	// Dictionary is non-nil only for categorical / set types; it maps
	// string categories to compact IDs.
	Dictionary *Dictionary
	// Precision is the decimal128 precision (1-38). Meaningful only when
	// Type is FieldTypeDecimal128.
	Precision uint8
	// Scale is the decimal128 scale (0-Precision). Meaningful only when
	// Type is FieldTypeDecimal128.
	Scale uint8
	// Description is optional human-facing documentation; empty means
	// none was supplied.
	Description string
}

// Schema holds the ordered field descriptors for a table.
type Schema struct {
	Fields []Field
}

// Field returns a pointer to the field with the given name, or nil when no
// such field exists. The pointer aliases the backing slice element, so
// callers may read (but must not reorder) it.
func (s *Schema) Field(name string) *Field {
	if s == nil {
		return nil
	}
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return &s.Fields[i]
		}
	}
	return nil
}

// Categorical returns the dictionary for a categorical field, or
// (nil, false) when the field is absent or non-categorical.
func (s *Schema) Categorical(name string) (*Dictionary, bool) {
	f := s.Field(name)
	if f == nil || f.Dictionary == nil || !f.Type.HasDictionary() {
		return nil, false
	}
	return f.Dictionary, true
}

// KindFromFieldType folds a native FieldType into one of the five Prism
// Kinds. It is the Pulse-free replacement for KindFromPulseFieldType and
// preserves the same lossy mapping: decimal128 -> KindFloat (surfaced as a
// numeric scalar in v1); categorical -> KindString; date -> KindDate;
// packed_bool -> KindBool; the unsigned-integer family (incl. u4) -> KindInt.
// Nullability is orthogonal — callers consult Field.Nullable separately.
// Set and unknown types return KindUnknown.
func KindFromFieldType(ft FieldType) Kind {
	switch {
	case ft.IsDecimal():
		return KindFloat
	case ft == FieldTypeF32, ft == FieldTypeF64:
		return KindFloat
	case ft == FieldTypeDate:
		return KindDate
	case ft == FieldTypePackedBool:
		return KindBool
	case ft.IsCategorical():
		return KindString
	case ft == FieldTypeU8, ft == FieldTypeU16,
		ft == FieldTypeU32, ft == FieldTypeU64,
		ft == FieldTypeU4:
		return KindInt
	}
	return KindUnknown
}
