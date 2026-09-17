package build_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/plan/nodes"
	_ "github.com/frankbardon/prism/plan/passes" // registers plan.DefaultPasses
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// buildOffsetDAG decodes an inline spec and returns its DAG + tip node.
func buildOffsetDAG(t *testing.T, body string) (*plan.DAG, plan.Node) {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	d, tip, err := build.Build(s, build.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	n, ok := d.Node(tip)
	if !ok {
		t.Fatalf("tip node %q not in DAG", tip)
	}
	return d, n
}

// offsetGroupedBarSpec is the canonical grouped-bar shape: a nominal
// category on x, an aggregated measure on y, and x_offset naming the
// series column the band slot is subdivided by.
const offsetGroupedBarSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"metric": "revenue", "series": "a", "offset_key": "2024", "value": 10},
    {"metric": "revenue", "series": "a", "offset_key": "2025", "value": 20},
    {"metric": "cost", "series": "b", "offset_key": "2024", "value": 5},
    {"metric": "cost", "series": "b", "offset_key": "2025", "value": 7}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "metric", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "value", "type": "quantitative"},
    "detail": {"field": "series", "type": "nominal"},
    "x_offset": {"field": "offset_key", "type": "nominal"}
  }
}`

// offsetSharedFieldSpec binds color and x_offset to the SAME column —
// the common authoring shape, where the dodge and the palette describe
// one series dimension.
const offsetSharedFieldSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"metric": "revenue", "series": "2024", "value": 10},
    {"metric": "revenue", "series": "2025", "value": 20}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "metric", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "value", "type": "quantitative"},
    "color": {"field": "series", "type": "nominal"},
    "x_offset": {"field": "series", "type": "nominal"}
  }
}`

// offsetNoAggregateSpec binds an offset with no aggregate anywhere, so
// nothing may be injected: that is the byte-identical no-op guarantee.
const offsetNoAggregateSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"metric": "revenue", "series": "2024", "value": 10},
    {"metric": "revenue", "series": "2025", "value": 20}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "metric", "type": "nominal"},
    "y": {"field": "value", "type": "quantitative"},
    "x_offset": {"field": "series", "type": "nominal"}
  }
}`

// offsetYSpec is the horizontal twin: y_offset subdivides a y band.
const offsetYSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"metric": "revenue", "series": "2024", "value": 10},
    {"metric": "revenue", "series": "2025", "value": 20}
  ]},
  "mark": "bar",
  "encoding": {
    "y": {"field": "metric", "type": "nominal"},
    "x": {"aggregate": "sum", "field": "value", "type": "quantitative"},
    "y_offset": {"field": "series", "type": "nominal"}
  }
}`

// offsetFieldlessSpec declares an offset object that binds no field.
// spec.ResolveOffset answers nil for it, so the groupby must not grow.
const offsetFieldlessSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"metric": "revenue", "value": 10}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "metric", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "value", "type": "quantitative"},
    "x_offset": {"type": "nominal"}
  }
}`

// TestPrismOffsetFieldJoinsSyntheticAggregate is the story: the offset
// column has to survive the synthetic GroupAggregateNode or the
// encoder has nothing to subdivide the band by.
func TestPrismOffsetFieldJoinsSyntheticAggregate(t *testing.T) {
	d, tip := buildOffsetDAG(t, offsetGroupedBarSpec)
	ga := offsetGroupAggregate(t, d, tip)
	assertGroupbyCount(t, ga.Groupby(), "metric", 1)
	assertGroupbyCount(t, ga.Groupby(), "offset_key", 1)
	// The measure is aggregated, never a groupby entry.
	assertGroupbyCount(t, ga.Groupby(), "value", 0)
	if len(ga.Aggs()) != 1 || ga.Aggs()[0].Field != "value" {
		t.Fatalf("Aggs() = %+v, want one sum over \"value\"", ga.Aggs())
	}
}

// TestPrismOffsetSharedFieldDedupes pins that color and x_offset
// naming one field yield exactly one groupby entry — the existing
// `seen` dedup in injectEncodingAggregate covers it, and no second
// dedup belongs here.
func TestPrismOffsetSharedFieldDedupes(t *testing.T) {
	d, tip := buildOffsetDAG(t, offsetSharedFieldSpec)
	ga := offsetGroupAggregate(t, d, tip)
	assertGroupbyCount(t, ga.Groupby(), "series", 1)
	assertGroupbyCount(t, ga.Groupby(), "metric", 1)
	if len(ga.Groupby()) != 2 {
		t.Fatalf("Groupby() = %v, want exactly [metric series]", ga.Groupby())
	}
}

// TestPrismOffsetYChannelJoinsSyntheticAggregate is the same guarantee
// for the horizontal orientation, reached through the same resolver.
func TestPrismOffsetYChannelJoinsSyntheticAggregate(t *testing.T) {
	d, tip := buildOffsetDAG(t, offsetYSpec)
	ga := offsetGroupAggregate(t, d, tip)
	assertGroupbyCount(t, ga.Groupby(), "series", 1)
	assertGroupbyCount(t, ga.Groupby(), "metric", 1)
}

// TestPrismOffsetWithoutAggregateInjectsNothing is the additive
// guarantee: with no aggregate anywhere, no synthetic node appears.
func TestPrismOffsetWithoutAggregateInjectsNothing(t *testing.T) {
	d, tip := buildOffsetDAG(t, offsetNoAggregateSpec)
	if _, ok := tip.(*nodes.GroupAggregateNode); ok {
		t.Fatal("an offset with no aggregate injected a GroupAggregateNode")
	}
	for _, id := range d.Nodes() {
		n, _ := d.Node(id)
		if _, ok := n.(*nodes.GroupAggregateNode); ok {
			t.Fatalf("an offset with no aggregate injected a GroupAggregateNode at %q", id)
		}
	}
}

// TestPrismOffsetWithoutFieldDoesNotWidenGroupby pins that the plan
// reads the binding through spec.ResolveOffset rather than from a bare
// nil-pointer test: an offset object carrying no field binds nothing.
func TestPrismOffsetWithoutFieldDoesNotWidenGroupby(t *testing.T) {
	d, tip := buildOffsetDAG(t, offsetFieldlessSpec)
	ga := offsetGroupAggregate(t, d, tip)
	if len(ga.Groupby()) != 1 || ga.Groupby()[0] != "metric" {
		t.Fatalf("Groupby() = %v, want exactly [metric]", ga.Groupby())
	}
}

// TestPrismOffsetColumnSurvivesToTheSink runs the grouped-bar spec all
// the way through the optimizer and the in-memory executor, then reads
// the tip table's schema. This is the criterion that actually matters:
// whatever the passes do, the encoder must find the offset column on
// the table it is handed.
//
// The pruning pass keys off a GroupAggregateNode's groupby
// (collectColsFromNode in plan/passes/projection_pruning.go), so the
// offset field being a groupby entry is exactly what protects it — the
// pass needed no change, and this test is the proof rather than the
// assertion that it did.
func TestPrismOffsetColumnSurvivesToTheSink(t *testing.T) {
	s, err := spec.DecodeBytes([]byte(offsetGroupedBarSpec))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	d, tip, err := build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	opt, err := plan.Optimize(d, plan.DefaultPasses)
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	// Any ProjectNode the pruning pass injected must still carry the
	// offset column.
	for _, id := range opt.Nodes() {
		pn, _ := opt.Node(id)
		p, ok := pn.(*nodes.ProjectNode)
		if !ok {
			continue
		}
		found := false
		for _, f := range p.Fields() {
			if f == "offset_key" {
				found = true
			}
		}
		if !found {
			t.Fatalf("ProjectNode %q projects %v, dropping the offset column", id, p.Fields())
		}
	}
	res, err := plan.Execute(context.Background(), opt, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("Execute node errors: %v", res.Errors)
	}
	tbl, ok := res.Tables[tip]
	if !ok || tbl == nil {
		t.Fatal("tip table missing")
	}
	sch := tbl.Schema()
	found := false
	for i := range sch.Fields {
		if sch.Fields[i].Name == "offset_key" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sink schema %v has no offset column", sch.Fields)
	}
}

// offsetGroupAggregate returns the GroupAggregateNode at or directly
// upstream of n. A grouped bar never stacks (spec.ResolveStack yields
// to a bound offset), so the aggregate is normally the tip — but this
// tolerates one injected node above it so the test pins the groupby
// rather than the chain shape, which build_order_test.go already owns.
func offsetGroupAggregate(t *testing.T, d *plan.DAG, n plan.Node) *nodes.GroupAggregateNode {
	t.Helper()
	if ga, ok := n.(*nodes.GroupAggregateNode); ok {
		return ga
	}
	ins := n.Inputs()
	if len(ins) == 1 {
		up, ok := d.Node(ins[0])
		if ok {
			if ga, ok := up.(*nodes.GroupAggregateNode); ok {
				return ga
			}
		}
	}
	t.Fatalf("node %T is not a GroupAggregateNode and has none directly upstream", n)
	return nil
}

// assertGroupbyCount checks a field appears in groupby exactly want
// times — the "exactly once" half of the dedup guarantee.
func assertGroupbyCount(t *testing.T, groupby []string, field string, want int) {
	t.Helper()
	got := 0
	for _, g := range groupby {
		if g == field {
			got++
		}
	}
	if got != want {
		t.Fatalf("Groupby() = %v carries %q %d time(s), want %d", groupby, field, got, want)
	}
}
