package inmem

import (
	"context"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeProject emits a new table containing only the requested
// fields, in the requested order. Missing field names raise
// PRISM_PLAN_003 — same code the Schema() validation path uses, so
// the diagnostic is consistent across plan and execute.
func executeProject(_ context.Context, n *nodes.ProjectNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	schema := &encoding.Schema{Fields: make([]encoding.Field, 0, len(n.Fields()))}
	cols := make(map[string]table.Column, len(n.Fields()))

	available := make([]string, 0, len(in.FieldNames()))
	idx := map[string]int{}
	for i, name := range in.FieldNames() {
		available = append(available, name)
		idx[name] = i
	}

	srcSchema := in.Schema()
	for _, want := range n.Fields() {
		pos, ok := idx[want]
		if !ok {
			return nil, prismerrors.New(
				"PRISM_PLAN_003",
				fmt.Sprintf("Field %q not in source schema (available: %s).", want, strings.Join(available, ", ")),
				map[string]any{"Dataset": want, "Available": strings.Join(available, ", ")},
			)
		}
		schema.Fields = append(schema.Fields, srcSchema.Fields[pos])
		col, _ := in.Column(want)
		cols[want] = col
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(schema, cols, in.NumRows(), hash)
}
