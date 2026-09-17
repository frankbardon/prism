package inmem

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
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
// Offset "center" (E5-S3) slides each partition's bounds so the
// partition's own midpoint lands on zero — the streamgraph
// silhouette. Every stack is then symmetric about the same baseline,
// so the union of all of them is symmetric too and the measure axis
// resolves a signed domain centred on zero.
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
	ranks := stackOrderRanks(in, valCol, n)

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
		switch n.Offset() {
		case spec.StackOffsetNormalize:
			normalizeStack(partition, starts, ends)
		case spec.StackOffsetCenter:
			centerStack(partition, starts, ends)
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

// centerStack slides one partition's bounds so the partition's own
// midpoint sits at zero — d3's silhouette offset, the streamgraph
// layout. A stack spanning [lo, hi] becomes [-(hi-lo)/2, (hi-lo)/2],
// so every stack is symmetric about the same line no matter how much
// total it carries, and the widest stack alone sets the axis extent.
func centerStack(partition []int, starts, ends table.FloatColumn) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, idx := range partition {
		lo = math.Min(lo, math.Min(starts[idx], ends[idx]))
		hi = math.Max(hi, math.Max(starts[idx], ends[idx]))
	}
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return
	}
	shift := -(lo + hi) / 2
	for _, idx := range partition {
		starts[idx] += shift
		ends[idx] += shift
	}
}

// stackOrderRanks returns the per-row segment rank the node's ordering
// asks for: the inside-out layout when it selects one, otherwise the
// first-appearance rank of the stack-by tuple. Falling back on the
// first-appearance rank (rather than nil) keeps a degenerate
// inside-out request — no stack-by field, one series — on the
// documented default instead of dropping ordering entirely.
func stackOrderRanks(tbl *table.Table, valCol table.Column, n *nodes.StackNode) []int {
	base := stackByRanks(tbl, n.StackBy())
	if n.Ordering() != spec.StackOrderInsideOut || base == nil {
		return base
	}
	if r := insideOutRanks(tbl, valCol, base, n.Groupby()); r != nil {
		return r
	}
	return base
}

// insideOutRanks re-ranks the stack-by series into d3's
// stackOrderInsideOut layout, the ordering a streamgraph wants.
//
// Series are visited in order of where they peak — the stack in which
// each reaches its largest value — and each is placed on whichever
// side of the stack currently carries less total magnitude. The two
// sides are then joined low-side-reversed first, so the
// earliest-peaking series end up adjacent to the centre line and the
// later ones fray at the edges. The running magnitudes are what keep
// the two sides balanced, which is why a heavy series can be pushed
// outward even though it peaks early.
//
// "Where a series peaks" is measured against stack order, which is
// first-appearance order of the groupby tuple — the same order
// buildPartitions walks. A spec whose rows arrive out of dimension
// order shapes that with a preceding `sort` transform, exactly as the
// segment-order contract already documents.
//
// Returns nil when there is nothing to reorder (fewer than two
// series), leaving the caller on the first-appearance rank.
func insideOutRanks(tbl *table.Table, valCol table.Column, base []int, groupby []string) []int {
	series := 0
	for _, r := range base {
		if r+1 > series {
			series = r + 1
		}
	}
	if series < 2 {
		return nil
	}

	stackIdx := stackByRanks(tbl, groupby)
	stacks := 1
	for _, s := range stackIdx {
		if s+1 > stacks {
			stacks = s + 1
		}
	}

	// totals[series][stack] is the signed magnitude the series carries
	// in that stack; sums[series] its absolute total across all of
	// them.
	totals := make([][]float64, series)
	for i := range totals {
		totals[i] = make([]float64, stacks)
	}
	sums := make([]float64, series)
	for row, s := range base {
		v, ok := numericCell(valCol, row)
		if !ok {
			continue
		}
		col := 0
		if stackIdx != nil {
			col = stackIdx[row]
		}
		totals[s][col] += v
		sums[s] += math.Abs(v)
	}

	peaks := make([]int, series)
	for s := range totals {
		best, at := math.Inf(-1), 0
		for col, v := range totals[s] {
			if v > best {
				best, at = v, col
			}
		}
		peaks[s] = at
	}

	order := make([]int, series)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return peaks[order[a]] < peaks[order[b]] })

	var lowTotal, highTotal float64
	low := make([]int, 0, series)
	high := make([]int, 0, series)
	for _, s := range order {
		if highTotal < lowTotal {
			highTotal += sums[s]
			high = append(high, s)
		} else {
			lowTotal += sums[s]
			low = append(low, s)
		}
	}

	rankOf := make([]int, series)
	pos := 0
	for i := len(low) - 1; i >= 0; i-- {
		rankOf[low[i]] = pos
		pos++
	}
	for _, s := range high {
		rankOf[s] = pos
		pos++
	}

	out := make([]int, len(base))
	for row, s := range base {
		out[row] = rankOf[s]
	}
	return out
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
