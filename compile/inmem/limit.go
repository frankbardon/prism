package inmem

import (
	"context"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeLimit emits rows [offset, offset+limit), clipped to the
// input table's length. limit <= 0 keeps zero rows.
func executeLimit(_ context.Context, n *nodes.LimitNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	start := n.Offset()
	if start < 0 {
		start = 0
	}
	if start > in.NumRows() {
		start = in.NumRows()
	}
	end := start + n.Limit()
	if n.Limit() <= 0 {
		end = start
	}
	if end > in.NumRows() {
		end = in.NumRows()
	}

	count := end - start
	idx := make([]int, count)
	for i := 0; i < count; i++ {
		idx[i] = start + i
	}

	cols := make(map[string]table.Column, len(in.FieldNames()))
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		cols[name] = pickRowsByIndex(col, idx)
	}
	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(in.Schema(), cols, count, hash)
}
