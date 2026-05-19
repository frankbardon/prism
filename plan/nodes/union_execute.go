package nodes

import (
	"context"
	"fmt"

	"github.com/cespare/xxhash/v2"
	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/table"
)

// executeUnion concatenates N inputs row-wise. Every input must declare
// the same schema in the same field order, with identical Pulse types.
// Mismatches surface as PRISM_PLAN_004 with diff context naming the
// offending input index and field.
func (n *UnionNode) executeUnion(_ context.Context, in []*table.Table) (*table.Table, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("UnionNode: no inputs")
	}
	for i, t := range in {
		if t == nil {
			return nil, fmt.Errorf("UnionNode: input[%d] is nil", i)
		}
	}
	base := in[0].Schema()
	for i := 1; i < len(in); i++ {
		if err := schemasMatch(base, in[i].Schema(), i); err != nil {
			return nil, err
		}
	}

	// Cardinality gate.
	total := 0
	for _, t := range in {
		total += t.NumRows()
	}
	cap := limits.MustTableMaxRows()
	if total > cap {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_007",
			fmt.Sprintf("Materialisation refused: %d rows would exceed PRISM_TABLE_MAX_ROWS=%d.", total, cap),
			map[string]any{"Actual": total, "Limit": cap},
		)
	}

	// Build per-column accumulators with combined capacity.
	cols := newColumnBuildersFor(base)
	for _, t := range in {
		for ri := 0; ri < t.NumRows(); ri++ {
			for fi := range base.Fields {
				name := base.Fields[fi].Name
				c, _ := t.Column(name)
				appendColValue(cols[name], c, ri)
			}
		}
	}

	hash := unionResultHash(n, in)
	return table.NewTable(base, finaliseColumnsFor(cols, base), total, hash)
}

// schemasMatch reports the first per-field discrepancy between left and
// right as a PRISM_PLAN_004. ix is the input index (right-side); the
// diff string names that index plus the offending field.
func schemasMatch(left, right *encoding.Schema, ix int) error {
	if len(left.Fields) != len(right.Fields) {
		return prismerrors.New(
			"PRISM_PLAN_004",
			fmt.Sprintf("Union input schemas disagree: input[%d] declares %d fields; input[0] declares %d.",
				ix, len(right.Fields), len(left.Fields)),
			map[string]any{"Diff": fmt.Sprintf("input[%d] field-count=%d; input[0] field-count=%d",
				ix, len(right.Fields), len(left.Fields))},
		)
	}
	for k := range left.Fields {
		lf, rf := &left.Fields[k], &right.Fields[k]
		if lf.Name != rf.Name || lf.Type != rf.Type {
			return prismerrors.New(
				"PRISM_PLAN_004",
				fmt.Sprintf("Union input schemas disagree: input[%d].field[%d]={%s,%s} != input[0].field[%d]={%s,%s}.",
					ix, k, rf.Name, rf.Type, k, lf.Name, lf.Type),
				map[string]any{
					"Diff": fmt.Sprintf("input[%d].field[%d]={%s,%s} != input[0].field[%d]={%s,%s}",
						ix, k, rf.Name, rf.Type, k, lf.Name, lf.Type),
				},
			)
		}
	}
	return nil
}

// unionResultHash builds a deterministic content hash for the
// concatenated table.
func unionResultHash(n *UnionNode, in []*table.Table) string {
	h := xxhash.New()
	_, _ = h.Write([]byte("UnionNode|"))
	_, _ = h.Write([]byte(n.Fingerprint()))
	_, _ = h.Write([]byte{0x1F})
	for _, t := range in {
		_, _ = h.Write([]byte(t.Hash()))
		_, _ = h.Write([]byte{0x1F})
	}
	return fmt.Sprintf("xxh64:%016x", h.Sum64())
}
