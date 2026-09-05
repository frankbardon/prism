package table

// Row is a materialized, row-oriented snapshot of one Table row, keyed
// by field name. Table itself stays columnar internally (see
// Table/Column) — Row exists purely as an ergonomic view for
// consumers that want row-oriented access instead, chiefly a `custom`
// mark's CustomRenderer (E2), which receives whole rows rather than
// column slices.
type Row map[string]any

// Rows materializes every row of t as a []Row, in row order. Intended
// for callers that need row-oriented access over a modest number of
// rows (e.g. a CustomRenderer); large tables should prefer the
// columnar Table/Column API directly to avoid the per-row allocation.
func (t *Table) Rows() []Row {
	if t == nil {
		return nil
	}
	n := t.NumRows()
	fields := t.FieldNames()
	out := make([]Row, n)
	for i := 0; i < n; i++ {
		row := make(Row, len(fields))
		for _, name := range fields {
			col, ok := t.Column(name)
			if !ok {
				continue
			}
			row[name] = col.ValueAt(i)
		}
		out[i] = row
	}
	return out
}
