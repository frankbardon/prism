package inmem

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeWindow appends one F64 column per WindowOp. Rows are
// partitioned by n.Partitionby(); within each partition we sort by
// n.Sort() (stable; nulls last) before applying the op. Frame is
// honoured for running/moving variants; rejected for lag/lead/ranks.
//
// Supported ops (P04): row_number, rank, dense_rank, lag, lead,
// running_sum, running_avg, moving_avg, pct_change, ewma. Unknown
// ops raise PRISM_COMPILE_003.
func executeWindow(_ context.Context, n *nodes.WindowNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	rows := in.NumRows()
	partitions := buildPartitions(in, n.Partitionby(), n.Sort())

	// Allocate output columns: input + one F64 per op.
	cols := make(map[string]table.Column, len(in.FieldNames())+len(n.Ops()))
	for _, name := range in.FieldNames() {
		c, _ := in.Column(name)
		cols[name] = c
	}
	for _, op := range n.Ops() {
		if op.As == "" {
			return nil, fmt.Errorf("WindowNode: op %s missing 'as' name", op.Op)
		}
		cols[op.As] = make(table.FloatColumn, rows)
	}

	for _, partition := range partitions {
		for _, op := range n.Ops() {
			out, err := applyWindowOp(in, op, partition)
			if err != nil {
				return nil, err
			}
			fc := cols[op.As].(table.FloatColumn)
			for i, idx := range partition {
				fc[idx] = out[i]
			}
		}
	}

	schema := cloneSchemaShallow(in.Schema())
	for _, op := range n.Ops() {
		schema.Fields = append(schema.Fields, encoding.Field{Name: op.As, Type: encoding.FieldTypeF64})
	}
	return table.NewTable(schema, cols, rows, hashChain(in.Hash(), n.Fingerprint()))
}

// buildPartitions returns a slice of sorted row-index slices, one
// per partition. Partition key uses keyFor over the partitionby
// columns; rows within each partition are sorted by the sort keys.
func buildPartitions(tbl *table.Table, partitionBy []string, sortKeys []nodes.SortKey) [][]int {
	rows := tbl.NumRows()
	idx := make([]int, rows)
	for i := range idx {
		idx[i] = i
	}

	if len(partitionBy) == 0 {
		sortInPlace(tbl, idx, sortKeys)
		return [][]int{idx}
	}

	groups := map[string][]int{}
	order := []string{}
	for _, i := range idx {
		parts := make([]string, len(partitionBy))
		for j, name := range partitionBy {
			col, _ := tbl.Column(name)
			parts[j] = keyFor(col.ValueAt(i))
		}
		key := strings.Join(parts, "\x00")
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}

	out := make([][]int, 0, len(order))
	for _, k := range order {
		p := groups[k]
		sortInPlace(tbl, p, sortKeys)
		out = append(out, p)
	}
	return out
}

func sortInPlace(tbl *table.Table, idx []int, keys []nodes.SortKey) {
	if len(keys) == 0 {
		return
	}
	sort.SliceStable(idx, func(a, b int) bool {
		for _, k := range keys {
			col, ok := tbl.Column(k.Field)
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
}

// applyWindowOp returns the per-partition output slice. partition is
// the indexed view; the returned slice is in partition order.
func applyWindowOp(tbl *table.Table, op nodes.WindowOp, partition []int) ([]float64, error) {
	n := len(partition)
	out := make([]float64, n)
	opName := strings.ToLower(op.Op)

	switch opName {
	case "row_number":
		for i := range partition {
			out[i] = float64(i + 1)
		}
		return out, nil
	case "rank", "dense_rank":
		col, _ := tbl.Column(op.Field) // may be nil; rank uses sort order
		if col == nil && op.Field != "" {
			return nil, fmt.Errorf("WindowNode: rank field %q missing", op.Field)
		}
		// Ranks are 1-based against the sort order; equal values share rank.
		// dense_rank does not skip; rank skips by tie group size.
		dense := opName == "dense_rank"
		currentRank := 1
		nextRank := 1
		for i := 0; i < n; i++ {
			if i > 0 {
				prev := partition[i-1]
				curr := partition[i]
				equal := col != nil && compareValues(col.ValueAt(prev), col.ValueAt(curr)) == 0
				if !equal {
					if dense {
						currentRank++
					} else {
						currentRank = nextRank
					}
				}
			}
			out[i] = float64(currentRank)
			nextRank++
		}
		return out, nil
	case "lag", "lead":
		col, ok := tbl.Column(op.Field)
		if !ok {
			return nil, fmt.Errorf("WindowNode: %s field %q missing", op.Op, op.Field)
		}
		offset := 1
		if op.Param != nil {
			offset = int(*op.Param)
		}
		for i := 0; i < n; i++ {
			src := i - offset
			if opName == "lead" {
				src = i + offset
			}
			if src < 0 || src >= n {
				out[i] = math.NaN()
				continue
			}
			switch v := col.ValueAt(partition[src]).(type) {
			case float64:
				out[i] = v
			case int64:
				out[i] = float64(v)
			}
		}
		return out, nil
	case "running_sum":
		vals := partitionFloats(tbl, op.Field, partition)
		s := 0.0
		for i, v := range vals {
			s += v
			out[i] = s
		}
		return out, nil
	case "running_avg":
		vals := partitionFloats(tbl, op.Field, partition)
		s := 0.0
		for i, v := range vals {
			s += v
			out[i] = s / float64(i+1)
		}
		return out, nil
	case "moving_avg":
		vals := partitionFloats(tbl, op.Field, partition)
		win := 3
		if op.Param != nil {
			win = int(*op.Param)
		}
		if win <= 0 {
			win = 1
		}
		for i := 0; i < n; i++ {
			lo := i - win + 1
			if lo < 0 {
				lo = 0
			}
			s := 0.0
			for j := lo; j <= i; j++ {
				s += vals[j]
			}
			out[i] = s / float64(i-lo+1)
		}
		return out, nil
	case "ewma":
		vals := partitionFloats(tbl, op.Field, partition)
		alpha := 0.5
		if op.Param != nil {
			alpha = *op.Param
		}
		if alpha <= 0 || alpha > 1 {
			alpha = 0.5
		}
		if n > 0 {
			out[0] = vals[0]
		}
		for i := 1; i < n; i++ {
			out[i] = alpha*vals[i] + (1-alpha)*out[i-1]
		}
		return out, nil
	case "pct_change":
		vals := partitionFloats(tbl, op.Field, partition)
		for i := 0; i < n; i++ {
			if i == 0 || vals[i-1] == 0 {
				out[i] = math.NaN()
				continue
			}
			out[i] = (vals[i] - vals[i-1]) / vals[i-1]
		}
		return out, nil
	}

	return nil, prismerrors.New("PRISM_COMPILE_003",
		fmt.Sprintf("Window op %q has no in-memory implementation (supported: row_number, rank, dense_rank, lag, lead, running_sum, running_avg, moving_avg, pct_change, ewma).", op.Op),
		map[string]any{"Alias": op.Op, "Backend": "inmem"},
	)
}

func partitionFloats(tbl *table.Table, field string, partition []int) []float64 {
	out := make([]float64, len(partition))
	col, ok := tbl.Column(field)
	if !ok {
		return out
	}
	for i, idx := range partition {
		switch v := col.ValueAt(idx).(type) {
		case float64:
			out[i] = v
		case int64:
			out[i] = float64(v)
		case bool:
			if v {
				out[i] = 1
			}
		}
	}
	return out
}
