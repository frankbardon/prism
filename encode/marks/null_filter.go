package marks

import "github.com/frankbardon/prism/table"

// SkipNullRows returns the indices of rows in tbl where every named
// field is non-null. Encoders that consume `fields` per-row use this
// to drop rows where any required channel is null so geometries don't
// render at zero positions or default colors.
//
// The skipped count + offending field names are reported back so the
// caller can surface PRISM_WARN_NULL_DROPPED. See
// `.planning/tier1-02-hash-join-null-bitmap-plan.md`.
func SkipNullRows(tbl *table.Table, fields ...string) (kept []int, dropped int, offending []string) {
	if tbl == nil {
		return nil, 0, nil
	}
	n := tbl.NumRows()
	kept = make([]int, 0, n)
	offendingSet := map[string]struct{}{}
	for i := 0; i < n; i++ {
		row := true
		for _, f := range fields {
			if f == "" {
				continue
			}
			col, ok := tbl.Column(f)
			if !ok {
				continue
			}
			if col.IsNull(i) {
				row = false
				offendingSet[f] = struct{}{}
			}
		}
		if row {
			kept = append(kept, i)
		} else {
			dropped++
		}
	}
	if dropped == 0 {
		return kept, 0, nil
	}
	for f := range offendingSet {
		offending = append(offending, f)
	}
	return kept, dropped, offending
}
