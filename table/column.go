// Package table holds Prism's columnar in-memory table type (`Table`),
// shared by every DAG node from Source through Encode. Columns are
// typed by a small `Kind` enum that buckets Pulse's 13 storage types
// down to five categories sufficient for spec validation and rendering.
//
// D015 establishes that Table is materialised (not streaming); D016
// establishes that storage is columnar with deferred bit-packing.
// D024 (queued for P02) records why `table/` is its own package rather
// than nested under `compile/` or `plan/`.
package table

import (
	"github.com/frankbardon/pulse/encoding"
)

// Kind buckets Pulse storage types into Prism's columnar categories.
// The mapping is intentionally lossy: every Pulse type folds into
// exactly one Kind, so downstream code (scales, encodings, format
// strings) reasons about one of five shapes instead of seventeen.
type Kind int

const (
	// KindUnknown is the zero value; a Column must never report it.
	KindUnknown Kind = iota
	// KindInt covers unsigned + bit-packed integer Pulse types.
	KindInt
	// KindFloat covers f32, f64, and decimal128 variants.
	KindFloat
	// KindString covers categorical types (rendered by dictionary).
	KindString
	// KindBool covers packed_bool. Null state lives in the per-record
	// null bitmap when Field.Nullable is set.
	KindBool
	// KindDate covers the date Pulse type (days-since-epoch).
	KindDate
)

// String returns the snake-case name (used in error context and tests).
func (k Kind) String() string {
	switch k {
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindString:
		return "string"
	case KindBool:
		return "bool"
	case KindDate:
		return "date"
	default:
		return "unknown"
	}
}

// KindFromPulseFieldType folds the 13 Pulse FieldType variants into the
// five Prism Kinds. Decimal types route to KindFloat in v1 because we
// surface them as numeric scalars at the encoding layer; revisit when a
// dedicated decimal renderer lands. Nullability is orthogonal to Kind —
// callers consult Field.Nullable separately.
func KindFromPulseFieldType(ft encoding.FieldType) Kind {
	switch {
	case ft.IsDecimal():
		return KindFloat
	case ft == encoding.FieldTypeF32, ft == encoding.FieldTypeF64:
		return KindFloat
	case ft == encoding.FieldTypeDate:
		return KindDate
	case ft == encoding.FieldTypePackedBool:
		return KindBool
	case ft.IsCategorical():
		return KindString
	case ft == encoding.FieldTypeU8, ft == encoding.FieldTypeU16,
		ft == encoding.FieldTypeU32, ft == encoding.FieldTypeU64,
		ft == encoding.FieldTypeU4:
		return KindInt
	}
	return KindUnknown
}

// Column is the read interface for one materialised column. Concrete
// impls are typed Go slices (no bit-packing in v1, per D016).
type Column interface {
	// Kind reports the Prism Kind bucket this column belongs to.
	Kind() Kind
	// Len returns the row count.
	Len() int
	// ValueAt returns the i-th value as an interface{} (any). Returns
	// nil if the column carries an explicit null sentinel at i.
	ValueAt(i int) any
	// IsNull reports whether row i carries an explicit null marker.
	// Implementations that don't track nullability return false for
	// every i (the safe backward-compatible answer).
	IsNull(i int) bool
	// NullCount returns the number of nulls in the column. Plain
	// slice-backed columns return 0; nullable wrappers consult their
	// bitmap. -1 is reserved for future implementations that defer
	// the count until requested.
	NullCount() int
}


// IntColumn is the storage for KindInt columns. int64 is wide enough
// to hold u64 values up to math.MaxInt64; values above that are
// truncated at decode time and a warning surfaces in Source telemetry.
type IntColumn []int64

// Kind implements Column.
func (IntColumn) Kind() Kind { return KindInt }

// Len implements Column.
func (c IntColumn) Len() int { return len(c) }

// ValueAt implements Column.
func (c IntColumn) ValueAt(i int) any { return c[i] }

// IsNull implements Column. Plain slice columns never report nulls;
// nullable variants live behind NullableColumn.
func (IntColumn) IsNull(int) bool { return false }

// NullCount implements Column.
func (IntColumn) NullCount() int { return 0 }

// FloatColumn is the storage for KindFloat columns.
type FloatColumn []float64

// Kind implements Column.
func (FloatColumn) Kind() Kind { return KindFloat }

// Len implements Column.
func (c FloatColumn) Len() int { return len(c) }

// ValueAt implements Column.
func (c FloatColumn) ValueAt(i int) any { return c[i] }

// IsNull implements Column.
func (FloatColumn) IsNull(int) bool { return false }

// NullCount implements Column.
func (FloatColumn) NullCount() int { return 0 }

// StringColumn is the storage for KindString columns (categorical
// values decoded against their Pulse dictionary at materialisation time).
type StringColumn []string

// Kind implements Column.
func (StringColumn) Kind() Kind { return KindString }

// Len implements Column.
func (c StringColumn) Len() int { return len(c) }

// ValueAt implements Column.
func (c StringColumn) ValueAt(i int) any { return c[i] }

// IsNull implements Column.
func (StringColumn) IsNull(int) bool { return false }

// NullCount implements Column.
func (StringColumn) NullCount() int { return 0 }

// BoolColumn is the storage for KindBool columns.
type BoolColumn []bool

// Kind implements Column.
func (BoolColumn) Kind() Kind { return KindBool }

// Len implements Column.
func (c BoolColumn) Len() int { return len(c) }

// ValueAt implements Column.
func (c BoolColumn) ValueAt(i int) any { return c[i] }

// IsNull implements Column.
func (BoolColumn) IsNull(int) bool { return false }

// NullCount implements Column.
func (BoolColumn) NullCount() int { return 0 }

// DateColumn is the storage for KindDate columns. Values are stored as
// int64 "days since epoch" using Pulse's wire format; scales/encoders
// convert to time.Time at the render boundary.
type DateColumn []int64

// Kind implements Column.
func (DateColumn) Kind() Kind { return KindDate }

// Len implements Column.
func (c DateColumn) Len() int { return len(c) }

// ValueAt implements Column.
func (c DateColumn) ValueAt(i int) any { return c[i] }

// IsNull implements Column.
func (DateColumn) IsNull(int) bool { return false }

// NullCount implements Column.
func (DateColumn) NullCount() int { return 0 }

// NullableColumn wraps any Column with an optional null bitmap. When
// the bitmap is nil, the wrapper is transparent. When set, ValueAt
// returns nil for marked positions and IsNull / NullCount consult the
// bitmap.
//
// The plan-stage hash join allocates a NullableColumn per output
// column that may receive unmatched rows; the resolver and inline
// loader wrap nullable Pulse fields the same way.
type NullableColumn struct {
	Inner Column
	Nulls *NullBitmap
}

// Kind implements Column by delegating to Inner.
func (n NullableColumn) Kind() Kind {
	if n.Inner == nil {
		return KindUnknown
	}
	return n.Inner.Kind()
}

// Len implements Column.
func (n NullableColumn) Len() int {
	if n.Inner == nil {
		return 0
	}
	return n.Inner.Len()
}

// ValueAt returns nil for null rows; otherwise delegates to Inner.
func (n NullableColumn) ValueAt(i int) any {
	if n.Inner == nil {
		return nil
	}
	if n.Nulls != nil && n.Nulls.IsNull(i) {
		return nil
	}
	return n.Inner.ValueAt(i)
}

// IsNull implements Column.
func (n NullableColumn) IsNull(i int) bool {
	if n.Nulls == nil {
		return false
	}
	return n.Nulls.IsNull(i)
}

// NullCount implements Column.
func (n NullableColumn) NullCount() int {
	if n.Nulls == nil {
		return 0
	}
	return n.Nulls.Count()
}

// Unwrap returns the underlying column. Callers that need to feed a
// slice-backed reader (e.g. scale extent computation) can read the
// inner column directly, then consult IsNull per row.
func (n NullableColumn) Unwrap() Column { return n.Inner }

