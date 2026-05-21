package inmem

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/compile"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeGroupAggregate partitions the input by groupby fields and
// emits one row per partition with each requested aggregate. Output
// schema is groupby fields (with their original types) followed by
// one F64 column per AggOp named by `op.As`.
//
// Aggregate semantics follow the alias map (compile/aggregates.go).
// Pulse-backed aliases (mean/sum/count/etc.) use the same numerical
// approach Pulse does (Welford for online moments; sort+interpolate
// for percentiles). Cohort-analytics aliases (wmean/ratio/lift/share/
// ci0/ci1) use the documented per-alias formulae over the partition.
//
// Aliases not in the map raise PRISM_COMPILE_003.
func executeGroupAggregate(_ context.Context, n *nodes.GroupAggregateNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	groups, order, err := partitionByGroupby(in, n.Groupby())
	if err != nil {
		return nil, err
	}

	// Build output schema: groupby fields verbatim + one F64 per agg op.
	srcSchema := in.Schema()
	gbFields := make([]encoding.Field, 0, len(n.Groupby()))
	gbCols := map[string]int{}
	for i, f := range srcSchema.Fields {
		gbCols[f.Name] = i
	}
	for _, name := range n.Groupby() {
		pos, ok := gbCols[name]
		if !ok {
			return nil, prismerrors.New(
				"PRISM_PLAN_003",
				fmt.Sprintf("Groupby field %q not in source schema.", name),
				map[string]any{"Dataset": name, "Available": strings.Join(in.FieldNames(), ", ")},
			)
		}
		gbFields = append(gbFields, srcSchema.Fields[pos])
	}

	outSchema := &encoding.Schema{Fields: make([]encoding.Field, 0, len(gbFields)+len(n.Aggs()))}
	outSchema.Fields = append(outSchema.Fields, gbFields...)
	for _, op := range n.Aggs() {
		if op.As == "" {
			return nil, fmt.Errorf("GroupAggregateNode: aggregate %s missing 'as' name", op.Op)
		}
		outSchema.Fields = append(outSchema.Fields, encoding.Field{Name: op.As, Type: encoding.FieldTypeF64})
	}

	// Allocate output columns for groupby fields with the right kind.
	cols := map[string]table.Column{}
	for _, name := range n.Groupby() {
		f := srcSchema.Field(name)
		kind := table.KindFromPulseFieldType(f.Type)
		switch kind {
		case table.KindString:
			cols[name] = make(table.StringColumn, 0, len(order))
		case table.KindInt:
			cols[name] = make(table.IntColumn, 0, len(order))
		case table.KindFloat:
			cols[name] = make(table.FloatColumn, 0, len(order))
		case table.KindBool:
			cols[name] = make(table.BoolColumn, 0, len(order))
		case table.KindDate:
			cols[name] = make(table.DateColumn, 0, len(order))
		}
	}
	for _, op := range n.Aggs() {
		cols[op.As] = make(table.FloatColumn, 0, len(order))
	}

	// Pre-compute share denominators per agg (sum over whole table).
	shareTotals := map[string]float64{}
	for _, op := range n.Aggs() {
		if strings.ToLower(op.Op) == "share" {
			col, ok := in.Column(op.Field)
			if !ok {
				return nil, prismerrors.New("PRISM_PLAN_003",
					fmt.Sprintf("share aggregate references missing field %q.", op.Field),
					map[string]any{"Dataset": op.Field, "Available": strings.Join(in.FieldNames(), ", ")},
				)
			}
			total := 0.0
			for _, v := range floatColumnValues(col) {
				total += v
			}
			shareTotals[op.Field] = total
		}
	}

	for _, key := range order {
		idx := groups[key]
		// Append groupby key values from the first row of the group.
		if len(idx) > 0 {
			for _, name := range n.Groupby() {
				src, _ := in.Column(name)
				cols[name] = appendOneCell(cols[name], src, idx[0])
			}
		}

		for _, op := range n.Aggs() {
			val, err := computeAggregate(in, op, idx, shareTotals)
			if err != nil {
				return nil, err
			}
			fc := cols[op.As].(table.FloatColumn)
			fc = append(fc, val)
			cols[op.As] = fc
		}
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(outSchema, cols, len(order), hash)
}

// partitionByGroupby builds a map of group-key → row indices. When
// groupby is empty, every row lands in one group keyed by "_all".
// order preserves first-appearance ordering so output rows are
// deterministic.
func partitionByGroupby(tbl *table.Table, groupby []string) (map[string][]int, []string, error) {
	groups := map[string][]int{}
	order := []string{}
	rows := tbl.NumRows()

	if len(groupby) == 0 {
		idx := make([]int, rows)
		for i := range idx {
			idx[i] = i
		}
		groups["_all"] = idx
		order = append(order, "_all")
		return groups, order, nil
	}

	for i := 0; i < rows; i++ {
		parts := make([]string, len(groupby))
		for j, name := range groupby {
			col, ok := tbl.Column(name)
			if !ok {
				return nil, nil, prismerrors.New("PRISM_PLAN_003",
					fmt.Sprintf("Groupby field %q not present at execute time.", name),
					map[string]any{"Dataset": name, "Available": strings.Join(tbl.FieldNames(), ", ")},
				)
			}
			parts[j] = keyFor(col.ValueAt(i))
		}
		key := strings.Join(parts, "\x00")
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], i)
	}
	return groups, order, nil
}

// appendOneCell widens out with src[i] and returns the new column.
// The output kind always equals src.Kind() (caller ensures the
// initial column allocation matches).
func appendOneCell(out, src table.Column, i int) table.Column {
	switch dst := out.(type) {
	case table.StringColumn:
		if s, ok := src.(table.StringColumn); ok {
			return append(dst, s[i])
		}
		return append(dst, keyFor(src.ValueAt(i)))
	case table.IntColumn:
		if s, ok := src.(table.IntColumn); ok {
			return append(dst, s[i])
		}
	case table.FloatColumn:
		if s, ok := src.(table.FloatColumn); ok {
			return append(dst, s[i])
		}
	case table.BoolColumn:
		if s, ok := src.(table.BoolColumn); ok {
			return append(dst, s[i])
		}
	case table.DateColumn:
		if s, ok := src.(table.DateColumn); ok {
			return append(dst, s[i])
		}
	}
	return out
}

// computeAggregate dispatches one AggOp over the row indices.
func computeAggregate(tbl *table.Table, op nodes.AggOp, idx []int, shareTotals map[string]float64) (float64, error) {
	alias := strings.ToLower(op.Op)
	mapping, known := compile.AliasToPulse[alias]
	if !known {
		return 0, prismerrors.New("PRISM_COMPILE_003",
			fmt.Sprintf("Aggregate alias %q is not in compile.AliasToPulse.", op.Op),
			map[string]any{"Alias": op.Op, "Backend": "inmem"},
		)
	}
	_ = mapping // alias is supported; below we compute it client-side.

	// "count" works even when Field is empty. count(*) counts every
	// row in the group; count(field) skips nulls (matches PostgreSQL
	// + pandas semantics). See tier1-02 PR3.
	if alias == "count" {
		if op.Field == "" {
			return float64(len(idx)), nil
		}
		col, ok := tbl.Column(op.Field)
		if !ok {
			return 0, prismerrors.New("PRISM_PLAN_003",
				fmt.Sprintf("Aggregate %q references missing field %q.", op.Op, op.Field),
				map[string]any{"Dataset": op.Field, "Available": strings.Join(tbl.FieldNames(), ", ")},
			)
		}
		n := 0
		for _, i := range idx {
			if !col.IsNull(i) {
				n++
			}
		}
		return float64(n), nil
	}

	col, ok := tbl.Column(op.Field)
	if !ok {
		return 0, prismerrors.New("PRISM_PLAN_003",
			fmt.Sprintf("Aggregate %q references missing field %q.", op.Op, op.Field),
			map[string]any{"Dataset": op.Field, "Available": strings.Join(tbl.FieldNames(), ", ")},
		)
	}

	vals := make([]float64, 0, len(idx))
	for _, i := range idx {
		v := col.ValueAt(i)
		switch x := v.(type) {
		case float64:
			vals = append(vals, x)
		case int64:
			vals = append(vals, float64(x))
		case bool:
			if x {
				vals = append(vals, 1)
			} else {
				vals = append(vals, 0)
			}
		}
	}

	switch alias {
	case "sum":
		return sum(vals), nil
	case "mean":
		return mean(vals), nil
	case "min":
		return minOf(vals), nil
	case "max":
		return maxOf(vals), nil
	case "stdev":
		return math.Sqrt(variance(vals)), nil
	case "variance":
		return variance(vals), nil
	case "median":
		return percentile(vals, 50), nil
	case "q1":
		return percentile(vals, 25), nil
	case "q3":
		return percentile(vals, 75), nil
	case "mode":
		return mode(vals), nil
	case "distinct":
		return distinct(idx, col), nil
	case "ci0":
		return ciBound(vals, -1), nil
	case "ci1":
		return ciBound(vals, +1), nil
	case "wmean":
		weights, err := siblingValues(tbl, op.Field, "_weight", idx)
		if err != nil {
			return 0, err
		}
		return weightedMean(vals, weights), nil
	case "ratio":
		denom, err := siblingValues(tbl, op.Field, "_denom", idx)
		if err != nil {
			return 0, err
		}
		if len(vals) == 0 || len(denom) == 0 || denom[0] == 0 {
			return 0, nil
		}
		return vals[0] / denom[0], nil
	case "lift":
		baseline, err := siblingValues(tbl, op.Field, "_baseline", idx)
		if err != nil {
			return 0, err
		}
		b := mean(baseline)
		if b == 0 {
			return 0, nil
		}
		return mean(vals) / b, nil
	case "share":
		total := shareTotals[op.Field]
		if total == 0 {
			return 0, nil
		}
		return sum(vals) / total, nil
	}

	return 0, prismerrors.New("PRISM_COMPILE_003",
		fmt.Sprintf("Aggregate alias %q has no in-memory implementation.", op.Op),
		map[string]any{"Alias": op.Op, "Backend": "inmem"},
	)
}

// siblingValues looks up a "<field><suffix>" sibling column and
// returns its values for the given row indices. Returns a structured
// error when the sibling is missing.
func siblingValues(tbl *table.Table, field, suffix string, idx []int) ([]float64, error) {
	name := field + suffix
	col, ok := tbl.Column(name)
	if !ok {
		return nil, prismerrors.New("PRISM_PLAN_003",
			fmt.Sprintf("Aggregate sibling field %q not present (convention: <field>%s).", name, suffix),
			map[string]any{"Dataset": name, "Available": strings.Join(tbl.FieldNames(), ", ")},
		)
	}
	out := make([]float64, 0, len(idx))
	for _, i := range idx {
		switch v := col.ValueAt(i).(type) {
		case float64:
			out = append(out, v)
		case int64:
			out = append(out, float64(v))
		}
	}
	return out, nil
}

// --- per-op numerical helpers ---

func sum(vals []float64) float64 {
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	return sum(vals) / float64(len(vals))
}

func minOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// variance returns the population variance (matches Pulse's
// AGG_VARIANCE which uses the population formula). Welford folded
// over the values; identical to Pulse's numerical approach (see
// processing/aggregator.go welfordAggregator).
func variance(vals []float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	m := 0.0
	m2 := 0.0
	for i, v := range vals {
		delta := v - m
		m += delta / float64(i+1)
		m2 += delta * (v - m)
	}
	return m2 / float64(n)
}

// percentile uses linear interpolation between the two nearest ranks
// (NIST recommended method). Matches Pulse's percentileAggregator
// (see processing/aggregator.go line 626).
func percentile(vals []float64, p float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	sort.Float64s(cp)
	rank := p / 100.0 * float64(len(cp)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		return cp[low]
	}
	return cp[low] + (rank-float64(low))*(cp[high]-cp[low])
}

func mode(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	count := map[float64]int{}
	for _, v := range vals {
		count[v]++
	}
	best := vals[0]
	bestN := 0
	for v, c := range count {
		if c > bestN || (c == bestN && v < best) {
			best = v
			bestN = c
		}
	}
	return best
}

func distinct(idx []int, col table.Column) float64 {
	seen := map[string]struct{}{}
	for _, i := range idx {
		if col.IsNull(i) {
			continue
		}
		seen[keyFor(col.ValueAt(i))] = struct{}{}
	}
	return float64(len(seen))
}

func weightedMean(vals, weights []float64) float64 {
	n := len(vals)
	if n == 0 || len(weights) != n {
		return 0
	}
	num := 0.0
	den := 0.0
	for i := 0; i < n; i++ {
		num += vals[i] * weights[i]
		den += weights[i]
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// ciBound mirrors Pulse's AGG_CI_LOWER / AGG_CI_UPPER bound2() math
// for default 0.95-confidence normal-method intervals: sample variance,
// Beasley-Springer-Moro inverse-normal quantile. Sign +1 = upper,
// -1 = lower. Returns NaN when n < 2 (matches Pulse).
func ciBound(vals []float64, sign float64) float64 {
	n := len(vals)
	if n < 2 {
		return math.NaN()
	}
	m := mean(vals)
	sampleVar := variance(vals) * float64(n) / float64(n-1)
	if sampleVar < 0 {
		sampleVar = 0
	}
	stderr := math.Sqrt(sampleVar) / math.Sqrt(float64(n))
	z := normalQuantile((1 + 0.95) / 2)
	return m + sign*z*stderr
}

// normalQuantile is the inverse CDF of the standard normal. Pulse
// v0.10.0 uses Beasley-Springer-Moro (~1e-9 accuracy); stdlib
// math.Erfinv via `z = sqrt(2) * erfinv(2p - 1)` is mathematically
// equivalent and matches Pulse output well within the parity
// tolerance.
func normalQuantile(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}
	return math.Sqrt2 * math.Erfinv(2*p-1)
}
