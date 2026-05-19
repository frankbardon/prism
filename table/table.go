package table

import (
	"fmt"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
)

// Table is the columnar in-memory result of one DAG node. Tables are
// immutable once constructed (callers receive aliased columns; never
// mutate them in place). Hash is computed by the producer (Resolver,
// SourceNode, inline converter) and propagated as the cache key.
type Table struct {
	schema   *encoding.Schema
	columns  map[string]Column
	order    []string
	rowCount int
	hash     string
}

// NewTable builds and validates a Table.
//
// Validation:
//   - schema must be non-nil and have at least one field.
//   - columns must contain exactly one entry per schema field (no extras,
//     no missing).
//   - every column's Len() must equal rowCount.
//   - every column's Kind() must match KindFromPulseFieldType for its
//     schema field's Type.
//   - rowCount must be in [0, limits.TableMaxRows()]. Exceeding the cap
//     returns PRISM_RESOLVE_007.
//
// hash is propagated verbatim; the producer owns hashing strategy.
func NewTable(schema *encoding.Schema, columns map[string]Column, rowCount int, hash string) (*Table, error) {
	if schema == nil {
		return nil, fmt.Errorf("table: schema is nil")
	}
	if len(schema.Fields) == 0 {
		return nil, fmt.Errorf("table: schema has no fields")
	}
	if rowCount < 0 {
		return nil, fmt.Errorf("table: negative rowCount %d", rowCount)
	}
	cap, _ := limits.TableMaxRows()
	if rowCount > cap {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_007",
			fmt.Sprintf("Materialisation refused: %d rows would exceed PRISM_TABLE_MAX_ROWS=%d.", rowCount, cap),
			map[string]any{"Actual": rowCount, "Limit": cap},
		)
	}
	if got, want := len(columns), len(schema.Fields); got != want {
		return nil, fmt.Errorf("table: have %d columns, schema declares %d", got, want)
	}

	order := make([]string, 0, len(schema.Fields))
	for i := range schema.Fields {
		f := &schema.Fields[i]
		col, ok := columns[f.Name]
		if !ok {
			return nil, fmt.Errorf("table: schema field %q has no column", f.Name)
		}
		if col.Len() != rowCount {
			return nil, fmt.Errorf("table: column %q has length %d, want %d", f.Name, col.Len(), rowCount)
		}
		wantKind := KindFromPulseFieldType(f.Type)
		if wantKind == KindUnknown {
			return nil, fmt.Errorf("table: schema field %q has unknown Pulse type %s", f.Name, f.Type)
		}
		if col.Kind() != wantKind {
			return nil, fmt.Errorf("table: column %q kind %s does not match schema field type %s (want %s)",
				f.Name, col.Kind(), f.Type, wantKind)
		}
		order = append(order, f.Name)
	}

	return &Table{
		schema:   schema,
		columns:  columns,
		order:    order,
		rowCount: rowCount,
		hash:     hash,
	}, nil
}

// Schema returns the table's Pulse schema.
func (t *Table) Schema() *encoding.Schema { return t.schema }

// NumRows returns the row count.
func (t *Table) NumRows() int { return t.rowCount }

// Hash returns the producer-supplied content hash. Cache keys combine
// this with the parent node's fingerprint per design/05-dag-executor.md.
func (t *Table) Hash() string { return t.hash }

// Column looks up a column by field name.
func (t *Table) Column(name string) (Column, bool) {
	c, ok := t.columns[name]
	return c, ok
}

// Columns returns the columns in schema declaration order.
func (t *Table) Columns() []Column {
	out := make([]Column, 0, len(t.order))
	for _, name := range t.order {
		out = append(out, t.columns[name])
	}
	return out
}

// FieldNames returns the field names in schema declaration order.
func (t *Table) FieldNames() []string {
	out := make([]string, len(t.order))
	copy(out, t.order)
	return out
}
