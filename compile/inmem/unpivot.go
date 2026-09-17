package inmem

import (
	"context"
	"fmt"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeUnpivot reshapes wide → long over the single materialised
// input: one input row carrying N unpivoted measure columns becomes N
// output rows, each repeating the untouched remaining columns, naming
// the measure in the key column and carrying its value in the value
// column.
//
// Output shape is taken straight from nodes.UnpivotNode.Schema() — the
// input schema minus the unpivoted fields, plus a categorical key
// column (as[0], default "key") and a numeric value column (as[1],
// default "value"), in that order. The names are read back off that
// schema rather than re-derived here so the executed table and the
// planned schema cannot disagree; a divergence would surface far from
// its cause as a PRISM_COMPILE_001 column-count mismatch.
//
// Row order is ROW-MAJOR and deterministic: every measure of input row
// 0 in `unpivot` declaration order, then every measure of input row 1,
// and so on. A source row's outputs stay adjacent, which is what makes
// a downstream sort / stack over the key column behave predictably.
//
// Nulls: a null in a source measure cell yields an output row whose
// value column is null (flagged in the column's null bitmap). The row
// is not dropped and the value is never coerced to zero — the null
// travels on for the encode-time null policy to act on.
//
// Types: the value column is f64, so an unpivoted source column must be
// numeric (KindInt / KindFloat). A categorical, boolean or date source
// column is refused with PRISM_COMPILE_002 rather than silently
// producing zeros or NaN.
func executeUnpivot(_ context.Context, n *nodes.UnpivotNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	fields := n.Unpivot()
	if len(fields) == 0 {
		return nil, fmt.Errorf("UnpivotNode: no fields to unpivot")
	}

	srcs := make([]table.Column, len(fields))
	for i, name := range fields {
		col, ok := in.Column(name)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_PLAN_003",
				fmt.Sprintf("unpivot references missing field %q.", name),
				map[string]any{"Dataset": name, "Available": strings.Join(in.FieldNames(), ", ")},
			)
		}
		switch col.Kind() {
		case table.KindInt, table.KindFloat:
		default:
			return nil, prismerrors.New(
				"PRISM_COMPILE_002",
				fmt.Sprintf("Transform evaluation failed at runtime: unpivot field %q is a %s column but the value column is numeric (f64).",
					name, col.Kind()),
				map[string]any{
					"Reason": fmt.Sprintf("unpivot field %q is a %s column but the value column is numeric (f64)", name, col.Kind()),
					"Site":   "unpivot",
				},
			)
		}
		srcs[i] = col
	}

	outSchema, err := n.Schema([]*table.Schema{in.Schema()})
	if err != nil {
		return nil, err
	}
	nOut := len(outSchema.Fields)
	if nOut < 2 {
		return nil, fmt.Errorf("UnpivotNode: output schema has %d fields, want key + value", nOut)
	}
	keyField := &outSchema.Fields[nOut-2]
	valField := &outSchema.Fields[nOut-1]
	carried := outSchema.Fields[:nOut-2]
	if err := unpivotNamesDistinct(carried, keyField.Name, valField.Name); err != nil {
		return nil, err
	}

	rows := in.NumRows()
	measures := len(fields)
	rowCap := limits.MustTableMaxRows()
	if rows > rowCap/measures {
		want := rows * measures
		return nil, prismerrors.New(
			"PRISM_RESOLVE_007",
			fmt.Sprintf("Materialisation refused: %d rows would exceed PRISM_TABLE_MAX_ROWS=%d.", want, rowCap),
			map[string]any{"Actual": want, "Limit": rowCap},
		)
	}
	outRows := rows * measures

	idx := make([]int, 0, outRows)
	keyCol := make(table.StringColumn, 0, outRows)
	valCol := make(table.FloatColumn, outRows)
	nulls := table.NewNullBitmap(outRows)
	o := 0
	for i := 0; i < rows; i++ {
		for j, col := range srcs {
			idx = append(idx, i)
			keyCol = append(keyCol, fields[j])
			if col.IsNull(i) {
				nulls.Set(o)
				o++
				continue
			}
			v, ok := coerceFloat(col.ValueAt(i))
			if !ok {
				// A numeric-kind column produced a non-numeric cell
				// (defensive): record it as null rather than invent a
				// value, matching executeCalculate's policy.
				nulls.Set(o)
				o++
				continue
			}
			valCol[o] = v
			o++
		}
	}

	cols := make(map[string]table.Column, nOut)
	for i := range carried {
		name := carried[i].Name
		src, ok := in.Column(name)
		if !ok {
			return nil, fmt.Errorf("UnpivotNode: carried field %q not in input table", name)
		}
		cols[name] = pickRowsByIndex(src, idx)
	}
	cols[keyField.Name] = keyCol

	var value table.Column = valCol
	if nulls.Count() > 0 {
		valField.Nullable = true
		value = table.NullableColumn{Inner: valCol, Nulls: nulls}
	}
	cols[valField.Name] = value

	return table.NewTable(outSchema, cols, outRows, hashChain(in.Hash(), n.Fingerprint()))
}

// unpivotNamesDistinct refuses an output schema whose key / value
// column name collides with a carried column (or with each other). The
// collision would otherwise reach table.NewTable as an opaque
// "have N columns, schema declares N+1" count mismatch.
func unpivotNamesDistinct(carried []table.Field, keyName, valName string) error {
	if keyName == valName {
		return unpivotCollision(keyName, "the unpivot key and value columns share the name")
	}
	for i := range carried {
		switch carried[i].Name {
		case keyName:
			return unpivotCollision(keyName, "the unpivot key column collides with a carried input column named")
		case valName:
			return unpivotCollision(valName, "the unpivot value column collides with a carried input column named")
		}
	}
	return nil
}

// unpivotCollision builds the PRISM_COMPILE_002 envelope for a name
// clash in the unpivot output schema.
func unpivotCollision(name, what string) error {
	reason := fmt.Sprintf("%s %q — set transform \"as\" to a free pair of names", what, name)
	return prismerrors.New(
		"PRISM_COMPILE_002",
		"Transform evaluation failed at runtime: "+reason+".",
		map[string]any{"Reason": reason, "Site": "unpivot"},
	)
}
