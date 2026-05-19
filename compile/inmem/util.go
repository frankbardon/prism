package inmem

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/table"
)

// rowValueAt fetches column[i] as a normalized any. Used by Filter /
// Calculate to populate per-row expression environments. Returns nil
// when the column is absent.
func rowValueAt(tbl *table.Table, name string, i int) any {
	col, ok := tbl.Column(name)
	if !ok {
		return nil
	}
	return col.ValueAt(i)
}

// buildEnv returns the per-row environment map every expression
// evaluation uses. Field values come from the input table; an
// `__row__` sentinel carries the row index for error context.
func buildEnv(tbl *table.Table, i int) map[string]any {
	out := make(map[string]any, len(tbl.FieldNames())+1)
	for _, name := range tbl.FieldNames() {
		out[name] = rowValueAt(tbl, name, i)
	}
	out["__row__"] = i
	return out
}

// cloneSchemaShallow returns a shallow copy of s so callers can
// extend Fields without mutating the input schema.
func cloneSchemaShallow(s *encoding.Schema) *encoding.Schema {
	out := &encoding.Schema{Fields: make([]encoding.Field, len(s.Fields))}
	copy(out.Fields, s.Fields)
	return out
}

// hashChain combines a parent table hash with an op fingerprint to
// produce a deterministic child hash. Same shape as the rest of the
// codebase: `xxh64:<hex>` (matches D025).
func hashChain(parentHash, opFingerprint string) string {
	h := xxhash.New()
	_, _ = h.WriteString(parentHash)
	_, _ = h.WriteString("|")
	_, _ = h.WriteString(opFingerprint)
	return fmt.Sprintf("xxh64:%016x", h.Sum64())
}

// pickRowsByMask emits a new column containing only rows where
// mask[i] is true. Preserves the source column's Kind.
func pickRowsByMask(col table.Column, mask []bool) table.Column {
	keep := 0
	for _, m := range mask {
		if m {
			keep++
		}
	}
	switch c := col.(type) {
	case table.IntColumn:
		out := make(table.IntColumn, 0, keep)
		for i, m := range mask {
			if m {
				out = append(out, c[i])
			}
		}
		return out
	case table.FloatColumn:
		out := make(table.FloatColumn, 0, keep)
		for i, m := range mask {
			if m {
				out = append(out, c[i])
			}
		}
		return out
	case table.StringColumn:
		out := make(table.StringColumn, 0, keep)
		for i, m := range mask {
			if m {
				out = append(out, c[i])
			}
		}
		return out
	case table.BoolColumn:
		out := make(table.BoolColumn, 0, keep)
		for i, m := range mask {
			if m {
				out = append(out, c[i])
			}
		}
		return out
	case table.DateColumn:
		out := make(table.DateColumn, 0, keep)
		for i, m := range mask {
			if m {
				out = append(out, c[i])
			}
		}
		return out
	}
	return col
}

// pickRowsByIndex emits a new column containing rows in the supplied
// order (no de-duplication; same row can appear twice). Used by Sort
// + Limit + Sample to physically reorder/subset columns.
func pickRowsByIndex(col table.Column, idx []int) table.Column {
	switch c := col.(type) {
	case table.IntColumn:
		out := make(table.IntColumn, len(idx))
		for i, k := range idx {
			out[i] = c[k]
		}
		return out
	case table.FloatColumn:
		out := make(table.FloatColumn, len(idx))
		for i, k := range idx {
			out[i] = c[k]
		}
		return out
	case table.StringColumn:
		out := make(table.StringColumn, len(idx))
		for i, k := range idx {
			out[i] = c[k]
		}
		return out
	case table.BoolColumn:
		out := make(table.BoolColumn, len(idx))
		for i, k := range idx {
			out[i] = c[k]
		}
		return out
	case table.DateColumn:
		out := make(table.DateColumn, len(idx))
		for i, k := range idx {
			out[i] = c[k]
		}
		return out
	}
	return col
}

// floatColumnValues materialises a column's values as a []float64 for
// numeric ops. Non-numeric columns return an empty slice.
func floatColumnValues(col table.Column) []float64 {
	switch c := col.(type) {
	case table.FloatColumn:
		out := make([]float64, len(c))
		copy(out, c)
		return out
	case table.IntColumn:
		out := make([]float64, len(c))
		for i, v := range c {
			out[i] = float64(v)
		}
		return out
	case table.DateColumn:
		out := make([]float64, len(c))
		for i, v := range c {
			out[i] = float64(v)
		}
		return out
	case table.BoolColumn:
		out := make([]float64, len(c))
		for i, v := range c {
			if v {
				out[i] = 1
			}
		}
		return out
	}
	return nil
}

// keyFor renders a value as a deterministic string for partition /
// group-by keys. Stable across runs (no map iteration).
func keyFor(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch x := v.(type) {
	case string:
		return "s:" + x
	case int64:
		return fmt.Sprintf("i:%d", x)
	case float64:
		return fmt.Sprintf("f:%g", x)
	case bool:
		if x {
			return "b:1"
		}
		return "b:0"
	}
	return fmt.Sprintf("v:%v", v)
}
