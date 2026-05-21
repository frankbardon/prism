package table

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

// FromInline turns inline `data.values` rows (and optional `data.fields`
// declarations) into a *Table backed by a synthetic *encoding.Schema.
//
// Type resolution:
//   - If fields is non-empty, every declared field is honoured verbatim;
//     unknown type tokens fall back to KindString (categorical_u8).
//   - Otherwise the first row's JSON kinds drive inference:
//     string → categorical_u8 / KindString
//     float64 / json.Number → f64 / KindFloat
//     bool → packed_bool / KindBool
//     other (nil, nested arrays/maps) → categorical_u8 / KindString
//     so downstream rules see a usable measure type.
//
// Subsequent rows are validated against the resolved schema; a row whose
// JSON kind for a given field disagrees with the schema returns
// PRISM_RESOLVE_INLINE_TYPE_MISMATCH with row index and field name.
//
// Hash is xxhash64 over a canonical JSON encoding of values (rows sorted
// by key per row, fields written in schema declaration order). Identical
// inputs map to identical hashes regardless of map iteration order.
func FromInline(name string, values []map[string]any, fields []spec.FieldSpec) (*Table, *encoding.Schema, error) {
	if len(values) == 0 && len(fields) == 0 {
		return nil, nil, fmt.Errorf("table: FromInline requires non-empty values or fields")
	}

	schema, fieldOrder, err := inferInlineSchema(values, fields)
	if err != nil {
		return nil, nil, err
	}

	cols, err := buildInlineColumns(schema, fieldOrder, values)
	if err != nil {
		return nil, nil, err
	}

	hash := hashInline(fieldOrder, values)

	tbl, err := NewTable(schema, cols, len(values), hash)
	if err != nil {
		return nil, nil, err
	}
	_ = name // reserved for future telemetry; keeps the signature stable.
	return tbl, schema, nil
}

// inferInlineSchema resolves field types from declarations or the first row.
func inferInlineSchema(values []map[string]any, fields []spec.FieldSpec) (*encoding.Schema, []string, error) {
	s := &encoding.Schema{}
	order := []string{}

	if len(fields) > 0 {
		for _, f := range fields {
			ft := pulseTypeFromToken(f.Type)
			fld := encoding.Field{Name: f.Name, Type: ft}
			if ft.IsCategorical() {
				fld.Dictionary = encoding.NewDictionary()
			}
			s.Fields = append(s.Fields, fld)
			order = append(order, f.Name)
		}
		return s, order, nil
	}

	if len(values) == 0 {
		return nil, nil, fmt.Errorf("table: FromInline cannot infer schema from empty values without declared fields")
	}

	// Use the first row's key set; preserve a deterministic order
	// (alphabetical) so identical specs produce identical schemas.
	first := values[0]
	keys := make([]string, 0, len(first))
	for k := range first {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		ft := pulseTypeFromJSONValue(first[k])
		fld := encoding.Field{Name: k, Type: ft}
		if ft.IsCategorical() {
			fld.Dictionary = encoding.NewDictionary()
		}
		s.Fields = append(s.Fields, fld)
		order = append(order, k)
	}
	return s, order, nil
}

// buildInlineColumns walks values once and emits typed columns. Each
// row is validated against the schema's field kinds; mismatches return
// PRISM_RESOLVE_INLINE_TYPE_MISMATCH. Per-field null bitmaps track JSON
// `null` values; the result wraps the underlying slice in a
// NullableColumn whenever any nulls appeared so downstream IsNull
// reports reflect the missing data.
func buildInlineColumns(schema *encoding.Schema, fieldOrder []string, values []map[string]any) (map[string]Column, error) {
	n := len(values)
	inner := map[string]Column{}
	nulls := map[string]*NullBitmap{}

	for i := range schema.Fields {
		f := &schema.Fields[i]
		kind := KindFromPulseFieldType(f.Type)
		switch kind {
		case KindInt:
			inner[f.Name] = make(IntColumn, n)
		case KindFloat:
			inner[f.Name] = make(FloatColumn, n)
		case KindString:
			inner[f.Name] = make(StringColumn, n)
		case KindBool:
			inner[f.Name] = make(BoolColumn, n)
		case KindDate:
			inner[f.Name] = make(DateColumn, n)
		default:
			return nil, fmt.Errorf("table: inline field %q has unknown kind for type %s", f.Name, f.Type)
		}
	}

	for rowIdx, row := range values {
		for _, name := range fieldOrder {
			val, present := row[name]
			if !present {
				// Missing keys: treat as null (the row supplied no value).
				ensureNullBitmap(nulls, name, n).Set(rowIdx)
				continue
			}
			if val == nil {
				ensureNullBitmap(nulls, name, n).Set(rowIdx)
				continue
			}
			f := schema.Field(name)
			kind := KindFromPulseFieldType(f.Type)
			gotKind, ok := classifyJSONValue(val)
			if ok && !inlineKindCompatible(kind, gotKind) {
				return nil, prismerrors.New(
					"PRISM_RESOLVE_INLINE_TYPE_MISMATCH",
					fmt.Sprintf("Inline row %d field %q has type %s but the schema declared %s.", rowIdx, name, gotKind, kind),
					map[string]any{
						"Row":      rowIdx,
						"Field":    name,
						"GotType":  gotKind.String(),
						"WantType": kind.String(),
					},
				)
			}
			if err := assignInlineValue(inner[name], rowIdx, kind, val, f); err != nil {
				return nil, err
			}
		}
	}

	// Wrap fields that observed at least one null.
	out := make(map[string]Column, len(inner))
	for name, col := range inner {
		nb := nulls[name]
		if nb != nil && nb.Count() > 0 {
			out[name] = NullableColumn{Inner: col, Nulls: nb}
		} else {
			out[name] = col
		}
	}
	return out, nil
}

func ensureNullBitmap(nulls map[string]*NullBitmap, name string, n int) *NullBitmap {
	if nb, ok := nulls[name]; ok {
		return nb
	}
	nb := NewNullBitmap(n)
	nulls[name] = nb
	return nb
}

// assignInlineValue places val into col[rowIdx] under the given kind.
func assignInlineValue(col Column, rowIdx int, kind Kind, val any, f *encoding.Field) error {
	switch kind {
	case KindInt:
		c := col.(IntColumn)
		switch v := val.(type) {
		case float64:
			c[rowIdx] = int64(v)
		case int:
			c[rowIdx] = int64(v)
		case int64:
			c[rowIdx] = v
		case nil:
			// zero value
		default:
			return prismerrors.New(
				"PRISM_RESOLVE_INLINE_TYPE_MISMATCH",
				fmt.Sprintf("Inline row %d field %q has type %T but the schema declared int.", rowIdx, f.Name, val),
				map[string]any{"Row": rowIdx, "Field": f.Name, "GotType": fmt.Sprintf("%T", val), "WantType": "int"},
			)
		}
	case KindFloat:
		c := col.(FloatColumn)
		switch v := val.(type) {
		case float64:
			c[rowIdx] = v
		case int:
			c[rowIdx] = float64(v)
		case int64:
			c[rowIdx] = float64(v)
		case nil:
		default:
			return prismerrors.New(
				"PRISM_RESOLVE_INLINE_TYPE_MISMATCH",
				fmt.Sprintf("Inline row %d field %q has type %T but the schema declared float.", rowIdx, f.Name, val),
				map[string]any{"Row": rowIdx, "Field": f.Name, "GotType": fmt.Sprintf("%T", val), "WantType": "float"},
			)
		}
	case KindString:
		c := col.(StringColumn)
		s := stringify(val)
		c[rowIdx] = s
		if f.Dictionary != nil {
			_, _ = f.Dictionary.Add(s) // ignore overflow at inline scale
		}
	case KindBool:
		c := col.(BoolColumn)
		b, _ := val.(bool)
		c[rowIdx] = b
	case KindDate:
		c := col.(DateColumn)
		if v, ok := val.(float64); ok {
			c[rowIdx] = int64(v)
		}
	}
	return nil
}

// classifyJSONValue maps a json-decoded value to its expected Kind.
// Returns ok=false when the value is nil or unclassifiable; callers
// then accept the row as a zero-value placeholder. Numeric values
// always report KindFloat (JSON has no integer/float distinction);
// inlineKindCompatible widens the check so an integer-declared column
// accepts numeric input without firing PRISM_RESOLVE_INLINE_TYPE_MISMATCH.
func classifyJSONValue(v any) (Kind, bool) {
	switch v.(type) {
	case string:
		return KindString, true
	case float64, int, int64, float32, int32:
		return KindFloat, true
	case bool:
		return KindBool, true
	case nil:
		return KindUnknown, false
	default:
		return KindUnknown, false
	}
}

// inlineKindCompatible reports whether a got value can populate a column
// declared as want. KindInt/KindFloat/KindDate all accept numeric JSON
// input because JSON does not distinguish integer/float.
func inlineKindCompatible(want, got Kind) bool {
	if want == got {
		return true
	}
	if got == KindFloat && (want == KindInt || want == KindDate) {
		return true
	}
	return false
}

// pulseTypeFromJSONValue picks a Pulse FieldType for an inline first-row value.
func pulseTypeFromJSONValue(v any) encoding.FieldType {
	switch v.(type) {
	case string:
		return encoding.FieldTypeCategoricalU8
	case float64, int, int64, float32, int32:
		return encoding.FieldTypeF64
	case bool:
		return encoding.FieldTypePackedBool
	default:
		return encoding.FieldTypeCategoricalU8
	}
}

// pulseTypeFromToken maps spec.FieldSpec.Type tokens (the same set used
// by validate/buildLookup) to Pulse FieldType variants.
func pulseTypeFromToken(token string) encoding.FieldType {
	switch strings.ToLower(token) {
	case "int", "int8", "int16", "int32", "int64", "u8", "u16", "u32", "u64":
		return encoding.FieldTypeU64
	case "float", "f32", "f64", "float32", "float64":
		return encoding.FieldTypeF64
	case "bool", "boolean":
		return encoding.FieldTypePackedBool
	case "date", "datetime":
		return encoding.FieldTypeDate
	default:
		return encoding.FieldTypeCategoricalU8
	}
}

// hashInline computes a stable xxhash64 over the inline rows. Each row
// is rendered in fieldOrder; the resulting byte stream is fed to xxhash.
func hashInline(fieldOrder []string, values []map[string]any) string {
	h := xxhash.New()
	for _, row := range values {
		// Iterate by fieldOrder so iteration order is independent of
		// map randomisation.
		obj := make([]json.RawMessage, 0, len(fieldOrder))
		for _, name := range fieldOrder {
			val, ok := row[name]
			if !ok {
				obj = append(obj, json.RawMessage("null"))
				continue
			}
			b, err := json.Marshal(val)
			if err != nil {
				b = []byte("null")
			}
			obj = append(obj, b)
		}
		rowBytes, _ := json.Marshal(obj)
		_, _ = h.Write(rowBytes)
		_, _ = h.Write([]byte{'\n'})
	}
	return fmt.Sprintf("xxh64:%016x", h.Sum64())
}

// stringify renders an arbitrary value as a string for the inline
// categorical column. Mirrors json.Marshal for non-strings.
func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
