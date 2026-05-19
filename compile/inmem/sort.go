package inmem

import (
	"context"
	"sort"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeSort returns a new table whose rows are reordered by the
// configured sort keys. Stable sort; nulls/zero-values last per key
// regardless of direction (matches pulse.OrderKey docs).
func executeSort(_ context.Context, n *nodes.SortNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	rows := in.NumRows()
	idx := make([]int, rows)
	for i := range idx {
		idx[i] = i
	}

	keys := n.Sort()
	sort.SliceStable(idx, func(a, b int) bool {
		for _, k := range keys {
			col, ok := in.Column(k.Field)
			if !ok {
				continue
			}
			va := col.ValueAt(idx[a])
			vb := col.ValueAt(idx[b])
			cmp := compareValues(va, vb)
			if cmp == 0 {
				continue
			}
			if k.Order == "desc" {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})

	cols := make(map[string]table.Column, len(in.FieldNames()))
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		cols[name] = pickRowsByIndex(col, idx)
	}
	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(in.Schema(), cols, rows, hash)
}

// compareValues returns -1 / 0 / +1 like strings.Compare. Mixed
// types fall back to a deterministic but arbitrary order keyed on
// the formatted form so sort is stable.
func compareValues(a, b any) int {
	switch av := a.(type) {
	case string:
		if bv, ok := b.(string); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case int64:
		if bv, ok := b.(int64); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case float64:
		if bv, ok := b.(float64); ok {
			switch {
			case av < bv:
				return -1
			case av > bv:
				return 1
			default:
				return 0
			}
		}
	case bool:
		if bv, ok := b.(bool); ok {
			switch {
			case !av && bv:
				return -1
			case av && !bv:
				return 1
			default:
				return 0
			}
		}
	}
	// Mixed-type fallback: compare formatted strings.
	as := keyFor(a)
	bs := keyFor(b)
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	}
	return 0
}
