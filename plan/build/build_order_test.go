package build_test

import (
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/plan/nodes"
	_ "github.com/frankbardon/prism/plan/passes" // registers plan.DefaultPasses
	"github.com/frankbardon/prism/spec"
)

// buildOrderDAG decodes an inline spec and returns its DAG + tip node.
func buildOrderDAG(t *testing.T, body string) (*plan.DAG, plan.Node) {
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

const orderPointSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"x": 1, "y": 10, "z": 3},
    {"x": 2, "y": 20, "z": 1}
  ]},
  "mark": "point",
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"},
    "order": {"field": "z", "type": "quantitative", "sort": "descending"}
  }
}`

const orderUnboundSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"x": 1, "y": 10},
    {"x": 2, "y": 20}
  ]},
  "mark": "point",
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"}
  }
}`

const orderStackSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"q": "q1", "seg": "north", "v": 10, "rank": 2},
    {"q": "q1", "seg": "south", "v": 20, "rank": 1}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "q", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
    "color": {"field": "seg", "type": "nominal"},
    "order": {"field": "rank", "type": "quantitative"}
  }
}`

const orderOnAggregatedFieldSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"q": "q1", "seg": "north", "v": 10},
    {"q": "q1", "seg": "south", "v": 20}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "q", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
    "color": {"field": "seg", "type": "nominal"},
    "order": {"field": "v", "type": "quantitative", "sort": "descending"}
  }
}`

// TestPrismOrderInjectsSortNode pins that a bound order channel adds a
// SortNode carrying the resolved keys in spec order.
func TestPrismOrderInjectsSortNode(t *testing.T) {
	_, tip := buildOrderDAG(t, orderPointSpec)
	sn, ok := tip.(*nodes.SortNode)
	if !ok {
		t.Fatalf("tip is %T, want *nodes.SortNode", tip)
	}
	keys := sn.Sort()
	if len(keys) != 1 {
		t.Fatalf("Sort() = %v, want one key", keys)
	}
	if keys[0].Field != "z" || !keys[0].Descending() {
		t.Fatalf("key = %+v, want z descending", keys[0])
	}
}

// TestPrismOrderUnboundInjectsNothing is the byte-identical guarantee:
// a spec that never declares order gets no node and no reordering.
func TestPrismOrderUnboundInjectsNothing(t *testing.T) {
	d, tip := buildOrderDAG(t, orderUnboundSpec)
	if _, ok := tip.(*nodes.SortNode); ok {
		t.Fatal("unbound order injected a SortNode")
	}
	for _, id := range d.Nodes() {
		n, _ := d.Node(id)
		if _, ok := n.(*nodes.SortNode); ok {
			t.Fatalf("unbound order injected a SortNode at %q", id)
		}
	}
}

// TestPrismOrderSortsBeforeStack pins the position in the chain: the
// sort runs after the synthetic aggregate and before the StackNode,
// which is what makes order control stack order.
func TestPrismOrderSortsBeforeStack(t *testing.T) {
	d, tip := buildOrderDAG(t, orderStackSpec)
	sn, ok := tip.(*nodes.StackNode)
	if !ok {
		t.Fatalf("tip is %T, want *nodes.StackNode", tip)
	}
	up, _ := d.Node(sn.Inputs()[0])
	sort, ok := up.(*nodes.SortNode)
	if !ok {
		t.Fatalf("stack input is %T, want *nodes.SortNode", up)
	}
	upup, _ := d.Node(sort.Inputs()[0])
	ga, ok := upup.(*nodes.GroupAggregateNode)
	if !ok {
		t.Fatalf("sort input is %T, want *nodes.GroupAggregateNode", upup)
	}
	// A non-aggregated order field has to survive the aggregate or the
	// sort below it reads a column that is not there.
	found := false
	for _, g := range ga.Groupby() {
		if g == "rank" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Groupby() = %v, want it to carry the order field \"rank\"", ga.Groupby())
	}
}

// TestPrismOrderOnAggregatedFieldKeepsGranularity pins that naming a
// field another channel already aggregates does NOT widen the groupby
// — the order key reads that aggregate's output column instead.
func TestPrismOrderOnAggregatedFieldKeepsGranularity(t *testing.T) {
	d, tip := buildOrderDAG(t, orderOnAggregatedFieldSpec)
	sn := tip.(*nodes.StackNode)
	sort := mustSortNode(t, d, sn.Inputs()[0])
	ga := mustGroupAggregate(t, d, sort.Inputs()[0])
	for _, g := range ga.Groupby() {
		if g == "v" {
			t.Fatalf("Groupby() = %v, must not carry the aggregated field \"v\"", ga.Groupby())
		}
	}
}

func mustSortNode(t *testing.T, d *plan.DAG, id plan.NodeID) *nodes.SortNode {
	t.Helper()
	n, ok := d.Node(id)
	if !ok {
		t.Fatalf("node %q not in DAG", id)
	}
	sn, ok := n.(*nodes.SortNode)
	if !ok {
		t.Fatalf("node %q is %T, want *nodes.SortNode", id, n)
	}
	return sn
}

func mustGroupAggregate(t *testing.T, d *plan.DAG, id plan.NodeID) *nodes.GroupAggregateNode {
	t.Helper()
	n, ok := d.Node(id)
	if !ok {
		t.Fatalf("node %q not in DAG", id)
	}
	ga, ok := n.(*nodes.GroupAggregateNode)
	if !ok {
		t.Fatalf("node %q is %T, want *nodes.GroupAggregateNode", id, n)
	}
	return ga
}
