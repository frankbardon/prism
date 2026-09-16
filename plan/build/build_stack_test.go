package build_test

import (
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/plan/nodes"
	_ "github.com/frankbardon/prism/plan/passes" // registers plan.DefaultPasses
	"github.com/frankbardon/prism/spec"
)

// buildStackDAG decodes an inline spec and returns its DAG + tip node.
func buildStackDAG(t *testing.T, body string) (*plan.DAG, plan.Node) {
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

const stackBarSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"q": "q1", "seg": "north", "v": 10},
    {"q": "q1", "seg": "south", "v": 20},
    {"q": "q2", "seg": "north", "v": 30},
    {"q": "q2", "seg": "south", "v": 40}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "q", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
    "color": {"field": "seg", "type": "nominal"}
  }
}`

// TestPrismStackInjectedAfterAggregate pins that the implicit stack
// node lands on top of the synthetic encoding aggregate, so it
// accumulates the aggregated values rather than the raw rows.
func TestPrismStackInjectedAfterAggregate(t *testing.T) {
	d, tip := buildStackDAG(t, stackBarSpec)
	sn, ok := tip.(*nodes.StackNode)
	if !ok {
		t.Fatalf("tip is %T, want *nodes.StackNode", tip)
	}
	if sn.Field() != "v" || sn.Offset() != "zero" {
		t.Errorf("got field=%q offset=%q, want v/zero", sn.Field(), sn.Offset())
	}
	if len(sn.Groupby()) != 1 || sn.Groupby()[0] != "q" {
		t.Errorf("Groupby() = %v, want [q]", sn.Groupby())
	}
	if len(sn.StackBy()) != 1 || sn.StackBy()[0] != "seg" {
		t.Errorf("StackBy() = %v, want [seg]", sn.StackBy())
	}
	up, ok := d.Node(sn.Inputs()[0])
	if !ok {
		t.Fatalf("stack input %q not in DAG", sn.Inputs()[0])
	}
	ga, ok := up.(*nodes.GroupAggregateNode)
	if !ok {
		t.Fatalf("stack input is %T, want *nodes.GroupAggregateNode", up)
	}
	// The grouping fields must survive the synthetic aggregate or the
	// stack has nothing to partition on.
	gb := map[string]bool{}
	for _, g := range ga.Groupby() {
		gb[g] = true
	}
	for _, want := range []string{"q", "seg"} {
		if !gb[want] {
			t.Errorf("aggregate groupby %v is missing %q", ga.Groupby(), want)
		}
	}
}

// TestPrismStackDisabledLeavesPlanAlone pins that `"stack": null`
// produces exactly the DAG a pre-E5-S2 build produced.
func TestPrismStackDisabledLeavesPlanAlone(t *testing.T) {
	_, tip := buildStackDAG(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"q": "q1", "seg": "north", "v": 10}]},
	  "mark": "bar",
	  "encoding": {
	    "x": {"field": "q", "type": "nominal"},
	    "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "stack": null},
	    "color": {"field": "seg", "type": "nominal"}
	  }
	}`)
	if _, ok := tip.(*nodes.StackNode); ok {
		t.Error("tip is a StackNode; `\"stack\": null` must disable stacking")
	}
}

// TestPrismStackUngroupedBarUnchanged pins the no-churn promise: the
// plain bar every committed golden renders gains no stack node.
func TestPrismStackUngroupedBarUnchanged(t *testing.T) {
	_, tip := buildStackDAG(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"q": "q1", "v": 10}]},
	  "mark": "bar",
	  "encoding": {
	    "x": {"field": "q", "type": "nominal"},
	    "y": {"field": "v", "type": "quantitative"}
	  }
	}`)
	if _, ok := tip.(*nodes.StackNode); ok {
		t.Error("tip is a StackNode; an ungrouped, unaggregated bar must not stack")
	}
}

// TestPrismStackExplicitTransform pins the explicit transform variant's
// plan dispatch, including the `as` output pair.
func TestPrismStackExplicitTransform(t *testing.T) {
	_, tip := buildStackDAG(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"q": "q1", "v": 10}]},
	  "transform": [{"stack": "v", "groupby": ["q"], "offset": "normalize", "as": ["lo", "hi"]}],
	  "mark": "bar",
	  "encoding": {
	    "x": {"field": "q", "type": "nominal"},
	    "y": {"field": "lo", "type": "quantitative"},
	    "y2": {"field": "hi", "type": "quantitative"}
	  }
	}`)
	sn, ok := tip.(*nodes.StackNode)
	if !ok {
		t.Fatalf("tip is %T, want *nodes.StackNode", tip)
	}
	if sn.Offset() != "normalize" {
		t.Errorf("Offset() = %q, want normalize", sn.Offset())
	}
	if sn.StartAs() != "lo" || sn.EndAs() != "hi" {
		t.Errorf("as = %q/%q, want lo/hi", sn.StartAs(), sn.EndAs())
	}
}

// TestPrismStackSurvivesOptimizer pins that the five registered passes
// leave a stacked plan executable — the stack node must still be the
// sink, and still point at a node the DAG holds.
func TestPrismStackSurvivesOptimizer(t *testing.T) {
	d, _ := buildStackDAG(t, stackBarSpec)
	if len(plan.DefaultPasses) == 0 {
		t.Fatal("plan.DefaultPasses is empty; plan/passes did not register")
	}
	out, err := plan.Optimize(d, plan.DefaultPasses)
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	sinks := out.Sinks()
	if len(sinks) != 1 {
		t.Fatalf("Sinks() = %v, want exactly one", sinks)
	}
	n, ok := out.Node(sinks[0])
	if !ok {
		t.Fatalf("sink %q not in optimized DAG", sinks[0])
	}
	sn, ok := n.(*nodes.StackNode)
	if !ok {
		t.Fatalf("optimized sink is %T, want *nodes.StackNode", n)
	}
	if _, ok := out.Node(sn.Inputs()[0]); !ok {
		t.Errorf("optimized stack input %q is not in the DAG", sn.Inputs()[0])
	}
}
