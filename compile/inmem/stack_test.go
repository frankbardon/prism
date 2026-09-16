package inmem

import (
	"context"
	"math"
	"testing"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// stackTable builds a (g, c, v) table: g is the stack key, c the
// segment key, v the accumulated measure.
func stackTable(t *testing.T, g, c []string, v []float64) *table.Table {
	t.Helper()
	gc := make(table.StringColumn, len(g))
	copy(gc, g)
	cc := make(table.StringColumn, len(c))
	copy(cc, c)
	vc := make(table.FloatColumn, len(v))
	copy(vc, v)
	schema := &table.Schema{Fields: []table.Field{
		{Name: "g", Type: table.FieldTypeCategoricalU8},
		{Name: "c", Type: table.FieldTypeCategoricalU8},
		{Name: "v", Type: table.FieldTypeF64},
	}}
	tbl, err := table.NewTable(schema, map[string]table.Column{"g": gc, "c": cc, "v": vc}, len(g), "stacksrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// runStack executes a StackNode over in and returns the two bound
// columns as float slices in row order.
func runStack(t *testing.T, n *nodes.StackNode, in *table.Table) (starts, ends []float64) {
	t.Helper()
	out, err := executeStack(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeStack: %v", err)
	}
	sc, ok := out.Column(n.StartAs())
	if !ok {
		t.Fatalf("output has no %q column", n.StartAs())
	}
	ec, ok := out.Column(n.EndAs())
	if !ok {
		t.Fatalf("output has no %q column", n.EndAs())
	}
	starts = make([]float64, sc.Len())
	ends = make([]float64, ec.Len())
	for i := range starts {
		starts[i], _ = numericCell(sc, i)
		ends[i], _ = numericCell(ec, i)
	}
	return starts, ends
}

// assertFloats compares two float slices within a tight tolerance.
func assertFloats(t *testing.T, label string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len = %d, want %d (%v)", label, len(got), len(want), got)
	}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("%s[%d] = %g, want %g (full: %v)", label, i, got[i], want[i], got)
		}
	}
}

// TestPrismStackZeroOffset pins the baseline accumulation: each stack
// runs 0 → sum and segments abut with no gap or overlap.
func TestPrismStackZeroOffset(t *testing.T) {
	in := stackTable(t,
		[]string{"q1", "q2", "q1", "q2"},
		[]string{"north", "north", "south", "south"},
		[]float64{120, 145, 100, 120})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, []string{"c"}, "zero", "", "")
	starts, ends := runStack(t, n, in)
	assertFloats(t, "start", starts, []float64{0, 0, 120, 145})
	assertFloats(t, "end", ends, []float64{120, 145, 220, 265})
}

// TestPrismStackSegmentOrderIsGlobal pins the ordering contract E5-S1
// publishes: segments rank by first appearance of the stack-by tuple
// across the WHOLE table, so "north" sits at the bottom of every stack
// even where the rows for that stack arrive in the other order.
func TestPrismStackSegmentOrderIsGlobal(t *testing.T) {
	in := stackTable(t,
		[]string{"q1", "q1", "q2", "q2"},
		[]string{"north", "south", "south", "north"},
		[]float64{10, 20, 30, 40})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, []string{"c"}, "zero", "", "")
	starts, ends := runStack(t, n, in)
	// q1: north 0→10, south 10→30. q2: north 0→40 (first-appearance
	// rank wins over row order), south 40→70.
	assertFloats(t, "start", starts, []float64{0, 10, 40, 0})
	assertFloats(t, "end", ends, []float64{10, 30, 70, 40})
}

// TestPrismStackNegativesSplit pins that a mixed-sign stack grows in
// both directions from zero instead of cancelling.
func TestPrismStackNegativesSplit(t *testing.T) {
	in := stackTable(t,
		[]string{"g", "g", "g"},
		[]string{"a", "b", "c"},
		[]float64{5, -3, 2})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, []string{"c"}, "zero", "", "")
	starts, ends := runStack(t, n, in)
	assertFloats(t, "start", starts, []float64{0, 0, 5})
	assertFloats(t, "end", ends, []float64{5, -3, 7})
}

// TestPrismStackNormalize pins the 0..1 rescale, per stack.
func TestPrismStackNormalize(t *testing.T) {
	in := stackTable(t,
		[]string{"q1", "q1", "q2", "q2"},
		[]string{"a", "b", "a", "b"},
		[]float64{25, 75, 30, 10})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, []string{"c"}, "normalize", "", "")
	starts, ends := runStack(t, n, in)
	assertFloats(t, "start", starts, []float64{0, 0.25, 0, 0.75})
	assertFloats(t, "end", ends, []float64{0.25, 1, 0.75, 1})
}

// TestPrismStackNormalizeDegenerate pins that an all-zero stack
// normalises to 0 rather than dividing by zero.
func TestPrismStackNormalizeDegenerate(t *testing.T) {
	in := stackTable(t, []string{"g", "g"}, []string{"a", "b"}, []float64{0, 0})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, []string{"c"}, "normalize", "", "")
	starts, ends := runStack(t, n, in)
	assertFloats(t, "start", starts, []float64{0, 0})
	assertFloats(t, "end", ends, []float64{0, 0})
}

// TestPrismStackCustomOutputNames pins the explicit `as` pair.
func TestPrismStackCustomOutputNames(t *testing.T) {
	in := stackTable(t, []string{"g"}, []string{"a"}, []float64{4})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, nil, "", "lo", "hi")
	out, err := executeStack(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeStack: %v", err)
	}
	for _, name := range []string{"lo", "hi"} {
		if _, ok := out.Column(name); !ok {
			t.Errorf("output has no %q column (fields: %v)", name, out.FieldNames())
		}
	}
	if n.Offset() != "zero" {
		t.Errorf("empty offset defaulted to %q, want zero", n.Offset())
	}
}

// TestPrismStackMissingField pins the diagnostic for a stack field the
// upstream table does not carry.
func TestPrismStackMissingField(t *testing.T) {
	in := stackTable(t, []string{"g"}, []string{"a"}, []float64{1})
	n := nodes.NewStack("stack:1", "src", "nope", []string{"g"}, nil, "zero", "", "")
	if _, err := executeStack(context.Background(), n, []*table.Table{in}); err == nil {
		t.Fatal("executeStack: want an error for a missing stack field")
	}
}

// TestPrismStackSchemaAppendsBounds pins the execute-time schema
// derivation and the output-column collision guard.
func TestPrismStackSchemaAppendsBounds(t *testing.T) {
	in := stackTable(t, []string{"g"}, []string{"a"}, []float64{1})
	n := nodes.NewStack("stack:1", "src", "v", []string{"g"}, nil, "zero", "", "")
	got, err := n.Schema([]*table.Schema{in.Schema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(got.Fields) != len(in.Schema().Fields)+2 {
		t.Fatalf("Schema fields = %d, want %d", len(got.Fields), len(in.Schema().Fields)+2)
	}
	last := got.Fields[len(got.Fields)-2:]
	if last[0].Name != "v_start" || last[1].Name != "v_end" {
		t.Errorf("appended fields = %v, want v_start/v_end", last)
	}

	collide := nodes.NewStack("stack:2", "src", "v", []string{"g"}, nil, "zero", "g", "c")
	if _, err := collide.Schema([]*table.Schema{in.Schema()}); err == nil {
		t.Error("Schema: want a collision error when `as` names an existing column")
	}
}
