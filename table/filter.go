package table

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/frankbardon/pulse/encoding"
)

// Filter returns a new Table holding only the rows for which keep[i]
// is true. The schema is preserved verbatim (same field order, same
// types); the row count is len(keep_set). The returned table's hash
// is derived from the source table's hash plus a content-addressed
// suffix carrying the partition tag, so cached tables remain
// content-addressed.
//
// Filter is the partition primitive used by the encoder's facet
// fan-out (D054). It is intentionally narrow — no transform
// semantics, no column projection — so the hot path stays a single
// columnar walk.
//
// partitionTag is an opaque string the caller uses to differentiate
// multiple sibling filters of the same parent. The encoder's facet
// path passes "facet:<rowVal>:<colVal>" so two distinct partitions
// of the same parent produce different hashes.
func Filter(src *Table, keep []bool, partitionTag string) (*Table, error) {
	if src == nil {
		return nil, fmt.Errorf("table.Filter: nil source table")
	}
	if len(keep) != src.rowCount {
		return nil, fmt.Errorf("table.Filter: keep length %d != source rowCount %d", len(keep), src.rowCount)
	}
	keepCount := 0
	for _, k := range keep {
		if k {
			keepCount++
		}
	}

	// Preserve schema verbatim; reusing src.schema is safe because
	// schemas are immutable post-NewTable.
	cols := map[string]Column{}
	for _, name := range src.order {
		srcCol, _ := src.Column(name)
		cols[name] = filterColumn(srcCol, keep, keepCount)
	}

	hash := childHash(src.hash, partitionTag, keepCount)
	tbl, err := NewTable(src.schema, cols, keepCount, hash)
	if err != nil {
		return nil, fmt.Errorf("table.Filter: %w", err)
	}
	return tbl, nil
}

// filterColumn produces a new typed column holding only the rows
// where keep[i] is true. The destination column is pre-sized to
// keepCount so no reallocation happens during the walk.
func filterColumn(src Column, keep []bool, keepCount int) Column {
	switch c := src.(type) {
	case IntColumn:
		out := make(IntColumn, 0, keepCount)
		for i, k := range keep {
			if k {
				out = append(out, c[i])
			}
		}
		return out
	case FloatColumn:
		out := make(FloatColumn, 0, keepCount)
		for i, k := range keep {
			if k {
				out = append(out, c[i])
			}
		}
		return out
	case StringColumn:
		out := make(StringColumn, 0, keepCount)
		for i, k := range keep {
			if k {
				out = append(out, c[i])
			}
		}
		return out
	case BoolColumn:
		out := make(BoolColumn, 0, keepCount)
		for i, k := range keep {
			if k {
				out = append(out, c[i])
			}
		}
		return out
	case DateColumn:
		out := make(DateColumn, 0, keepCount)
		for i, k := range keep {
			if k {
				out = append(out, c[i])
			}
		}
		return out
	}
	return src
}

// childHash produces a content-addressed hash for the filtered
// table: sha256(parent_hash + "\x00" + partitionTag + "\x00" +
// keepCount). Short prefix returned for cache-key efficiency.
func childHash(parent, tag string, keepCount int) string {
	h := sha256.New()
	h.Write([]byte(parent))
	h.Write([]byte{0})
	h.Write([]byte(tag))
	h.Write([]byte{0})
	h.Write([]byte(fmt.Sprintf("%d", keepCount)))
	return "filter:" + hex.EncodeToString(h.Sum(nil)[:8])
}

// SchemaFields returns the schema's field slice for callers that
// want to walk fields without reaching through Schema().Fields. Kept
// as a tiny accessor to centralise the dereference and ease future
// schema refactors.
func SchemaFields(s *encoding.Schema) []encoding.Field {
	if s == nil {
		return nil
	}
	return s.Fields
}
