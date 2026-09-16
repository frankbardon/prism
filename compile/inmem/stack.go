package inmem

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeStack appends the two F64 bound columns a StackNode declares.
//
// Rows are partitioned by n.Groupby() — one partition per stack — and
// ordered inside each partition by orderStackPartition. Within a
// partition the values accumulate from zero: positives run upward and
// negatives downward, tracked on independent cursors so a mixed-sign
// stack grows in both directions instead of cancelling.
//
// Offset "normalize" rescales each partition's bounds so the stack
// spans exactly 0..1 (Vega's normalize: (v - lo) / (hi - lo) over the
// partition's own extent, which reduces to v / total for all-positive
// data). A degenerate partition — every value zero, or unreadable —
// normalises to 0 rather than dividing by zero.
//
// A null or non-numeric cell contributes nothing: its segment gets a
// zero-height [cursor, cursor] bound and the cursor does not move.
func executeStack(_ context.Context, n *nodes.StackNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	valCol, ok := in.Column(n.Field())
	if !ok {
		return nil, prismerrors.New(
			"PRISM_PLAN_STACK_FIELD_MISSING",
			fmt.Sprintf("Stack field %q is not present in the upstream table.", n.Field()),
			map[string]any{"Field": n.Field(), "Available": strings.Join(in.FieldNames(), ", ")},
		)
	}

	rows := in.NumRows()
	starts := make(table.FloatColumn, rows)
	ends := make(table.FloatColumn, rows)

	// Partition by the stack-delimiting fields. Ordering inside a
	// partition is orderStackPartition's job, never buildPartitions'.
	partitions := buildPartitions(in, n.Groupby(), nil)
	ranks := stackByRanks(in, n.StackBy())

	for _, partition := range partitions {
		orderStackPartition(partition, ranks)
		var pos, neg float64
		for _, idx := range partition {
			v, okv := numericCell(valCol, idx)
			switch {
			case !okv || v == 0:
				starts[idx], ends[idx] = pos, pos
			case v > 0:
				starts[idx] = pos
				pos += v
				ends[idx] = pos
			default:
				starts[idx] = neg
				neg += v
				ends[idx] = neg
			}
		}
		if n.Offset() == "normalize" {
			normalizeStack(partition, starts, ends)
		}
	}

	cols := make(map[string]table.Column, len(in.FieldNames())+2)
	for _, name := range in.FieldNames() {
		c, _ := in.Column(name)
		cols[name] = c
	}
	cols[n.StartAs()] = starts
	cols[n.EndAs()] = ends

	schema := cloneSchemaShallow(in.Schema())
	schema.Fields = append(schema.Fields,
		table.Field{Name: n.StartAs(), Type: table.FieldTypeF64},
		table.Field{Name: n.EndAs(), Type: table.FieldTypeF64},
	)
	return table.NewTable(schema, cols, rows, hashChain(in.Hash(), n.Fingerprint()))
}

// stackByRanks assigns each row the first-appearance rank of its
// stack-by tuple across the whole table, mirroring the ordering
// contract encode/marks/group.go's groupRows publishes: the tuple that
// appears first upstream sorts first in every stack, so a segment
// occupies the same slot in each stack and the emission order lines up
// with the legend. Returns nil when no stack-by field is bound.
func stackByRanks(tbl *table.Table, fields []string) []int {
	if len(fields) == 0 {
		return nil
	}
	cols := make([]table.Column, 0, len(fields))
	for _, f := range fields {
		col, ok := tbl.Column(f)
		if !ok {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return nil
	}
	rows := tbl.NumRows()
	out := make([]int, rows)
	seen := map[string]int{}
	for i := 0; i < rows; i++ {
		parts := make([]string, len(cols))
		for j, col := range cols {
			parts[j] = keyFor(col.ValueAt(i))
		}
		key := strings.Join(parts, "\x00")
		rank, ok := seen[key]
		if !ok {
			rank = len(seen)
			seen[key] = rank
		}
		out[i] = rank
	}
	return out
}

// orderStackPartition sorts one partition's row indices in place by
// the stack-by first-appearance rank. With no stack-by field bound the
// partition keeps upstream row order, which an author shapes with a
// preceding `sort` transform.
func orderStackPartition(partition []int, ranks []int) {
	if ranks == nil {
		return
	}
	sort.SliceStable(partition, func(a, b int) bool {
		return ranks[partition[a]] < ranks[partition[b]]
	})
}

// normalizeStack rescales one partition's bounds onto 0..1 over the
// partition's own [lo, hi] extent.
func normalizeStack(partition []int, starts, ends table.FloatColumn) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, idx := range partition {
		lo = math.Min(lo, math.Min(starts[idx], ends[idx]))
		hi = math.Max(hi, math.Max(starts[idx], ends[idx]))
	}
	span := hi - lo
	if math.IsInf(span, 0) || span == 0 {
		for _, idx := range partition {
			starts[idx], ends[idx] = 0, 0
		}
		return
	}
	for _, idx := range partition {
		starts[idx] = (starts[idx] - lo) / span
		ends[idx] = (ends[idx] - lo) / span
	}
}

// numericCell coerces one cell to float64. Reports false for nulls and
// for anything that is not a number.
func numericCell(col table.Column, idx int) (float64, bool) {
	switch v := col.ValueAt(idx).(type) {
	case nil:
		return 0, false
	case float64:
		if math.IsNaN(v) {
			return 0, false
		}
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
