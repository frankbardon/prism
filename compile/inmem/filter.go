package inmem

import (
	"context"

	"github.com/frankbardon/prism/compile"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeFilter compiles the FilterNode's Pulse expression once and
// evaluates it per row. Surviving rows are picked from every input
// column; the schema is preserved verbatim.
//
// Compile errors surface as PRISM_COMPILE_002 via compile.CompileExpression;
// per-row evaluation errors short-circuit with PRISM_COMPILE_002 too
// (carrying the row index in the Site context).
func executeFilter(_ context.Context, n *nodes.FilterNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	prog, err := compile.CompileExpression(n.Expr())
	if err != nil {
		return nil, err
	}

	rows := in.NumRows()
	mask := make([]bool, rows)
	keep := 0
	for i := 0; i < rows; i++ {
		env := buildEnv(in, i)
		ok, err := prog.EvalBool(env)
		if err != nil {
			return nil, err
		}
		if ok {
			mask[i] = true
			keep++
		}
	}

	out := make(map[string]table.Column, len(in.FieldNames()))
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		out[name] = pickRowsByMask(col, mask)
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(in.Schema(), out, keep, hash)
}
