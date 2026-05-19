package inmem

import (
	"context"
	"fmt"
	"math"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeBin appends an F64 column (n.As()) carrying each row's bin
// lower-edge value. Auto picks 10 buckets across [min, max] (Vega-Lite
// default). Maxbins / Step / Extent overrides are honoured per the
// BinParams shape — not all wired in P04 (Step + Extent land when
// histogram demands them in P10); auto + maxbins cover bar/histogram
// in P04.
func executeBin(_ context.Context, n *nodes.BinNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	if n.As() == "" {
		return nil, fmt.Errorf("BinNode: missing 'as' name")
	}

	src, ok := in.Column(n.Field())
	if !ok {
		return nil, fmt.Errorf("BinNode: source field %q not in input table", n.Field())
	}
	values := floatColumnValues(src)
	if len(values) == 0 {
		// Empty input — append an empty F64 column to preserve schema.
		schema := cloneSchemaShallow(in.Schema())
		schema.Fields = append(schema.Fields, encoding.Field{Name: n.As(), Type: encoding.FieldTypeF64})
		cols := make(map[string]table.Column, len(in.FieldNames())+1)
		for _, name := range in.FieldNames() {
			c, _ := in.Column(name)
			cols[name] = c
		}
		cols[n.As()] = make(table.FloatColumn, 0)
		return table.NewTable(schema, cols, 0, hashChain(in.Hash(), n.Fingerprint()))
	}

	lo := values[0]
	hi := values[0]
	for _, v := range values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}

	maxbins := 10
	width := niceStep(lo, hi, maxbins)
	if width == 0 {
		width = 1
	}

	out := make(table.FloatColumn, len(values))
	for i, v := range values {
		out[i] = math.Floor((v-lo)/width)*width + lo
	}

	schema := cloneSchemaShallow(in.Schema())
	schema.Fields = append(schema.Fields, encoding.Field{Name: n.As(), Type: encoding.FieldTypeF64})

	cols := make(map[string]table.Column, len(in.FieldNames())+1)
	for _, name := range in.FieldNames() {
		c, _ := in.Column(name)
		cols[name] = c
	}
	cols[n.As()] = out

	return table.NewTable(schema, cols, in.NumRows(), hashChain(in.Hash(), n.Fingerprint()))
}

// niceStep picks a "nice" bin width rounded to a power-of-10
// increment. Matches the Vega-Lite default tick algorithm closely
// enough that hand-checked outputs agree on common ranges.
func niceStep(lo, hi float64, maxbins int) float64 {
	if hi == lo || maxbins <= 0 {
		return 0
	}
	rough := (hi - lo) / float64(maxbins)
	if rough <= 0 {
		return 0
	}
	mag := math.Pow(10, math.Floor(math.Log10(rough)))
	frac := rough / mag
	var nice float64
	switch {
	case frac < 1.5:
		nice = 1
	case frac < 3:
		nice = 2
	case frac < 7:
		nice = 5
	default:
		nice = 10
	}
	return nice * mag
}
