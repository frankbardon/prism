package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/table"
)

// executeJoin is the in-memory hash-join body invoked from JoinNode.Execute.
// D046 pins the algorithm: build hash table on the right input, scan
// the left input, emit per-kind. The build side is the right input
// unconditionally to keep fingerprints deterministic.
func (n *JoinNode) executeJoin(_ context.Context, in []*table.Table) (*table.Table, error) {
	if len(in) != 2 {
		return nil, fmt.Errorf("JoinNode: expected 2 inputs, got %d", len(in))
	}
	left, right := in[0], in[1]
	if left == nil || right == nil {
		return nil, fmt.Errorf("JoinNode: nil input table")
	}

	// 1. Validate join keys exist + types match on both sides.
	if err := validateJoinKeys(n.on, left.Schema(), right.Schema()); err != nil {
		return nil, err
	}

	// 2. Cardinality gate. n.maxRows == 0 → consult env.
	maxRows := n.maxRows
	if maxRows <= 0 {
		maxRows = limits.MustJoinMaxRows()
	}
	product := int64(left.NumRows()) * int64(right.NumRows())
	if product > int64(maxRows) {
		return nil, prismerrors.New(
			"PRISM_JOIN_003",
			fmt.Sprintf("Join would produce %d rows (left × right) and exceeds PRISM_JOIN_MAX_ROWS=%d.", product, maxRows),
			map[string]any{"Actual": product, "Limit": maxRows},
		)
	}

	// 3. Compute output schema based on kind.
	outSchema := joinOutputSchema(left.Schema(), right.Schema(), n.on, n.kind)

	// 4. Build hash on right; scan left.
	rightKeyCols, err := keyColumns(right, n.on)
	if err != nil {
		return nil, err
	}
	leftKeyCols, err := keyColumns(left, n.on)
	if err != nil {
		return nil, err
	}

	// Bucket right rows by encoded key.
	buckets := make(map[string][]int, right.NumRows())
	for i := 0; i < right.NumRows(); i++ {
		k := encodeJoinKey(rightKeyCols, i)
		buckets[k] = append(buckets[k], i)
	}
	matchedRight := make(map[int]struct{}) // for outer

	// Build per-column accumulators for the output schema.
	cols := newColumnBuildersFor(outSchema)

	switch n.kind {
	case JoinInner, JoinLeft, JoinOuter, "":
		for li := 0; li < left.NumRows(); li++ {
			k := encodeJoinKey(leftKeyCols, li)
			matches := buckets[k]
			if len(matches) == 0 {
				if n.kind == JoinLeft || n.kind == JoinOuter {
					appendLeftRow(cols, outSchema, left, li, right, -1)
				}
				continue
			}
			for _, ri := range matches {
				appendLeftRow(cols, outSchema, left, li, right, ri)
				matchedRight[ri] = struct{}{}
			}
		}
		if n.kind == JoinOuter {
			for ri := 0; ri < right.NumRows(); ri++ {
				if _, ok := matchedRight[ri]; ok {
					continue
				}
				// Emit a row with left fields zero, right fields populated,
				// plus the join-key fields from the right (because left
				// has no corresponding row).
				appendRightOnlyOuterRow(cols, outSchema, left, right, ri, n.on)
			}
		}
	case JoinAnti:
		for li := 0; li < left.NumRows(); li++ {
			k := encodeJoinKey(leftKeyCols, li)
			if len(buckets[k]) > 0 {
				continue
			}
			appendAntiRow(cols, outSchema, left, li)
		}
	default:
		return nil, fmt.Errorf("JoinNode: unknown kind %q", n.kind)
	}

	rowCount := outputRowCount(cols, outSchema)
	hash := joinResultHash(n, left, right)
	return table.NewTable(outSchema, finaliseColumnsFor(cols, outSchema), rowCount, hash)
}

// validateJoinKeys ensures every key in `on` is present in both schemas
// and that both sides expose the same Pulse Kind.
func validateJoinKeys(on []string, left, right *encoding.Schema) error {
	leftIdx := schemaIndex(left)
	rightIdx := schemaIndex(right)
	for _, key := range on {
		lf, lok := leftIdx[key]
		rf, rok := rightIdx[key]
		if !lok {
			return prismerrors.New(
				"PRISM_JOIN_002",
				fmt.Sprintf("Join key %q is missing on the left side (available: %s).",
					key, strings.Join(fieldNames(left), ", ")),
				map[string]any{"Key": key, "Side": "left",
					"Available": strings.Join(fieldNames(left), ", ")},
			)
		}
		if !rok {
			return prismerrors.New(
				"PRISM_JOIN_002",
				fmt.Sprintf("Join key %q is missing on the right side (available: %s).",
					key, strings.Join(fieldNames(right), ", ")),
				map[string]any{"Key": key, "Side": "right",
					"Available": strings.Join(fieldNames(right), ", ")},
			)
		}
		lk := table.KindFromPulseFieldType(lf.Type)
		rk := table.KindFromPulseFieldType(rf.Type)
		if lk != rk {
			return prismerrors.New(
				"PRISM_JOIN_001",
				fmt.Sprintf("Join key %q has incompatible kinds on the two sides (left=%s, right=%s).",
					key, lk, rk),
				map[string]any{"Key": key,
					"LeftKind": lk.String(), "RightKind": rk.String()},
			)
		}
	}
	return nil
}

func schemaIndex(s *encoding.Schema) map[string]*encoding.Field {
	out := make(map[string]*encoding.Field, len(s.Fields))
	for i := range s.Fields {
		out[s.Fields[i].Name] = &s.Fields[i]
	}
	return out
}

func fieldNames(s *encoding.Schema) []string {
	out := make([]string, 0, len(s.Fields))
	for i := range s.Fields {
		out = append(out, s.Fields[i].Name)
	}
	return out
}

// joinOutputSchema constructs the output schema for the given kind.
// inner/left/outer: left columns first, then right columns excluding
// join keys. anti: left columns only.
func joinOutputSchema(left, right *encoding.Schema, on []string, kind JoinKind) *encoding.Schema {
	if kind == JoinAnti {
		return cloneSchema(left)
	}
	return joinedSchema(left, right, on)
}

// keyColumns resolves the *table.Column for each join key in t.
// Caller has already validated that every key exists.
func keyColumns(t *table.Table, on []string) ([]table.Column, error) {
	out := make([]table.Column, len(on))
	for i, k := range on {
		c, ok := t.Column(k)
		if !ok {
			return nil, fmt.Errorf("join: column %q absent from input", k)
		}
		out[i] = c
	}
	return out, nil
}

// encodeJoinKey packs the join-key values of row i across all key
// columns into a stable string. Uses byte 0x1F (Unit Separator) as a
// delimiter; this character is illegal in valid identifiers + path
// strings, eliminating ambiguity at the cost of one byte per column.
func encodeJoinKey(cols []table.Column, i int) string {
	var b strings.Builder
	for j, c := range cols {
		if j > 0 {
			b.WriteByte(0x1F)
		}
		writeColValue(&b, c, i)
	}
	return b.String()
}

func writeColValue(b *strings.Builder, c table.Column, i int) {
	switch v := c.ValueAt(i).(type) {
	case int64:
		fmt.Fprintf(b, "i:%d", v)
	case float64:
		// %g is round-trip stable; not strictly canonical but adequate
		// for join-key equality (caller validated kinds match).
		fmt.Fprintf(b, "f:%g", v)
	case string:
		b.WriteString("s:")
		b.WriteString(v)
	case bool:
		if v {
			b.WriteString("b:1")
		} else {
			b.WriteString("b:0")
		}
	default:
		fmt.Fprintf(b, "?:%v", v)
	}
}

// newColumnBuildersFor allocates a builder per field in outSchema with
// the appropriate slice type for the field's Kind.
func newColumnBuildersFor(s *encoding.Schema) map[string]*columnBuilder {
	out := make(map[string]*columnBuilder, len(s.Fields))
	for i := range s.Fields {
		f := &s.Fields[i]
		kind := table.KindFromPulseFieldType(f.Type)
		cb := &columnBuilder{kind: kind}
		switch kind {
		case table.KindInt:
			s := make([]int64, 0)
			cb.ints = &s
		case table.KindFloat:
			s := make([]float64, 0)
			cb.floats = &s
		case table.KindString:
			s := make([]string, 0)
			cb.strs = &s
		case table.KindBool:
			s := make([]bool, 0)
			cb.bools = &s
		case table.KindDate:
			s := make([]int64, 0)
			cb.dates = &s
		}
		out[f.Name] = cb
	}
	return out
}

// finaliseColumnsFor materialises every builder into a typed Column.
func finaliseColumnsFor(cols map[string]*columnBuilder, s *encoding.Schema) map[string]table.Column {
	out := make(map[string]table.Column, len(cols))
	for i := range s.Fields {
		name := s.Fields[i].Name
		cb := cols[name]
		switch cb.kind {
		case table.KindInt:
			out[name] = table.IntColumn(*cb.ints)
		case table.KindFloat:
			out[name] = table.FloatColumn(*cb.floats)
		case table.KindString:
			out[name] = table.StringColumn(*cb.strs)
		case table.KindBool:
			out[name] = table.BoolColumn(*cb.bools)
		case table.KindDate:
			out[name] = table.DateColumn(*cb.dates)
		}
	}
	return out
}

// outputRowCount infers the row count from any one column. All columns
// in cols share a row count by construction (we append in lockstep).
func outputRowCount(cols map[string]*columnBuilder, s *encoding.Schema) int {
	for i := range s.Fields {
		name := s.Fields[i].Name
		cb := cols[name]
		switch cb.kind {
		case table.KindInt:
			return len(*cb.ints)
		case table.KindFloat:
			return len(*cb.floats)
		case table.KindString:
			return len(*cb.strs)
		case table.KindBool:
			return len(*cb.bools)
		case table.KindDate:
			return len(*cb.dates)
		}
	}
	return 0
}

// appendLeftRow emits one output row whose left fields come from
// left[leftIdx] and whose right (non-key) fields come from
// right[rightIdx] when rightIdx >= 0 (matched) or zero values when
// rightIdx == -1 (unmatched on a left/outer join).
func appendLeftRow(
	cols map[string]*columnBuilder,
	outSchema *encoding.Schema,
	left *table.Table, leftIdx int,
	right *table.Table, rightIdx int,
) {
	matched := rightIdx >= 0
	leftIndex := schemaIndex(left.Schema())
	for i := range outSchema.Fields {
		f := &outSchema.Fields[i]
		if _, onLeft := leftIndex[f.Name]; onLeft {
			c, _ := left.Column(f.Name)
			appendColValue(cols[f.Name], c, leftIdx)
			continue
		}
		// Right-side non-key field.
		if matched {
			c, _ := right.Column(f.Name)
			appendColValue(cols[f.Name], c, rightIdx)
		} else {
			appendZeroValue(cols[f.Name])
		}
	}
}

// appendRightOnlyOuterRow emits a row where right fields come from
// right[rightIdx] (including join keys, since left has no row) and
// left non-key fields are zero values.
func appendRightOnlyOuterRow(
	cols map[string]*columnBuilder,
	outSchema *encoding.Schema,
	left *table.Table,
	right *table.Table, rightIdx int,
	on []string,
) {
	keySet := make(map[string]struct{}, len(on))
	for _, k := range on {
		keySet[k] = struct{}{}
	}
	leftIndex := schemaIndex(left.Schema())
	for i := range outSchema.Fields {
		f := &outSchema.Fields[i]
		_, isKey := keySet[f.Name]
		_, onLeft := leftIndex[f.Name]
		switch {
		case isKey:
			// Join keys live in the left schema slot in the output but
			// take their value from the right input here.
			c, _ := right.Column(f.Name)
			appendColValue(cols[f.Name], c, rightIdx)
		case onLeft:
			// Non-key left column → zero.
			appendZeroValue(cols[f.Name])
		default:
			// Right-only column.
			c, _ := right.Column(f.Name)
			appendColValue(cols[f.Name], c, rightIdx)
		}
	}
}

// appendAntiRow emits a row with only left columns (anti join output
// schema equals the left schema verbatim).
func appendAntiRow(
	cols map[string]*columnBuilder,
	outSchema *encoding.Schema,
	left *table.Table, leftIdx int,
) {
	for i := range outSchema.Fields {
		f := &outSchema.Fields[i]
		c, _ := left.Column(f.Name)
		appendColValue(cols[f.Name], c, leftIdx)
	}
}

// appendColValue copies the i-th value of src into the builder dst.
// Caller has already established that src.Kind() == dst.kind.
func appendColValue(dst *columnBuilder, src table.Column, i int) {
	switch dst.kind {
	case table.KindInt:
		v := src.ValueAt(i).(int64)
		*dst.ints = append(*dst.ints, v)
	case table.KindFloat:
		v := src.ValueAt(i).(float64)
		*dst.floats = append(*dst.floats, v)
	case table.KindString:
		v := src.ValueAt(i).(string)
		*dst.strs = append(*dst.strs, v)
	case table.KindBool:
		v := src.ValueAt(i).(bool)
		*dst.bools = append(*dst.bools, v)
	case table.KindDate:
		v := src.ValueAt(i).(int64)
		*dst.dates = append(*dst.dates, v)
	}
}

// appendZeroValue appends the zero value of the builder's Kind. Used
// for left-join / outer-join unmatched right-side fields. Future work
// (null bitmap) replaces this with a proper null sentinel.
func appendZeroValue(dst *columnBuilder) {
	switch dst.kind {
	case table.KindInt:
		*dst.ints = append(*dst.ints, 0)
	case table.KindFloat:
		*dst.floats = append(*dst.floats, 0)
	case table.KindString:
		*dst.strs = append(*dst.strs, "")
	case table.KindBool:
		*dst.bools = append(*dst.bools, false)
	case table.KindDate:
		*dst.dates = append(*dst.dates, 0)
	}
}

// joinResultHash builds a deterministic content hash for the joined
// table from the node descriptor + both input hashes. Format mirrors
// table.NewTable's convention: `xxh64:<hex>`.
func joinResultHash(n *JoinNode, left, right *table.Table) string {
	h := xxhash.New()
	_, _ = h.Write([]byte("JoinNode|"))
	_, _ = h.Write([]byte(n.Fingerprint()))
	_, _ = h.Write([]byte{0x1F})
	_, _ = h.Write([]byte(left.Hash()))
	_, _ = h.Write([]byte{0x1F})
	_, _ = h.Write([]byte(right.Hash()))
	return fmt.Sprintf("xxh64:%016x", h.Sum64())
}
