package inmem

import (
	"context"
	"fmt"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/compile"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeCalculate appends one F64 column derived from a Pulse
// expression. Compile errors surface as PRISM_COMPILE_002 via
// compile.CompileExpression; per-row evaluation errors short-circuit
// with PRISM_COMPILE_002 too. Non-numeric results error out so the
// downstream F64 column is well-typed.
func executeCalculate(_ context.Context, n *nodes.CalculateNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	if n.As() == "" {
		return nil, fmt.Errorf("CalculateNode: missing 'as' name")
	}

	prog, err := compile.CompileExpression(n.Expr())
	if err != nil {
		return nil, err
	}

	rows := in.NumRows()
	values := make(table.FloatColumn, rows)
	nulls := table.NewNullBitmap(rows)
	for i := 0; i < rows; i++ {
		env := buildEnv(in, i)
		// Pulse expressions are strict on nulls: any null input
		// propagates to a null output (matches PostgreSQL / pandas
		// conventions). Caller picks this up via IsNull.
		if envHasNull(env) {
			nulls.Set(i)
			continue
		}
		v, err := prog.EvalFloat(env)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}

	schema := cloneSchemaShallow(in.Schema())
	schema.Fields = append(schema.Fields, encoding.Field{Name: n.As(), Type: encoding.FieldTypeF64, Nullable: nulls.Count() > 0})

	cols := make(map[string]table.Column, len(in.FieldNames())+1)
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		cols[name] = col
	}
	if nulls.Count() > 0 {
		cols[n.As()] = table.NullableColumn{Inner: values, Nulls: nulls}
	} else {
		cols[n.As()] = values
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(schema, cols, rows, hash)
}
