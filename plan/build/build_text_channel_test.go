package build_test

import (
	"testing"

	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
)

// buildTextSpec decodes an inline spec and returns the built DAG tip.
func buildTextSpec(t *testing.T, body string) *nodes.GroupAggregateNode {
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
	ga, ok := n.(*nodes.GroupAggregateNode)
	if !ok {
		return nil
	}
	return ga
}

// TestPrismTextChannelAggregateInjectsNode pins that text.aggregate is
// honoured — it routes through the same synthetic GroupAggregateNode
// every other channel's aggregate uses (E4-S4).
func TestPrismTextChannelAggregateInjectsNode(t *testing.T) {
	ga := buildTextSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"g": "a", "score": 1}, {"g": "a", "score": 3}]},
	  "mark": {"type": "text"},
	  "encoding": {
	    "x": {"field": "g", "type": "nominal"},
	    "text": {"field": "score", "type": "quantitative", "aggregate": "mean"}
	  }
	}`)
	if ga == nil {
		t.Fatal("tip is not a GroupAggregateNode; text.aggregate did not inject one")
	}
	if len(ga.Groupby()) != 1 || ga.Groupby()[0] != "g" {
		t.Errorf("Groupby() = %v, want [g]", ga.Groupby())
	}
	aggs := ga.Aggs()
	if len(aggs) != 1 || aggs[0].Op != "mean" || aggs[0].Field != "score" {
		t.Errorf("Aggs() = %+v, want one mean(score)", aggs)
	}
}

// TestPrismTextChannelJoinsGroupby pins the mirror case: a
// non-aggregated text field participates in the group-by when another
// channel declares an aggregate, so the label survives the rollup.
func TestPrismTextChannelJoinsGroupby(t *testing.T) {
	ga := buildTextSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"g": "a", "label": "A", "score": 1}]},
	  "mark": {"type": "text"},
	  "encoding": {
	    "x": {"field": "g", "type": "nominal"},
	    "y": {"field": "score", "type": "quantitative", "aggregate": "mean"},
	    "text": {"field": "label", "type": "nominal"}
	  }
	}`)
	if ga == nil {
		t.Fatal("tip is not a GroupAggregateNode")
	}
	var sawLabel bool
	for _, g := range ga.Groupby() {
		if g == "label" {
			sawLabel = true
		}
	}
	if !sawLabel {
		t.Errorf("Groupby() = %v, want it to include the text field \"label\"", ga.Groupby())
	}
}

// TestPrismTextChannelNoAggregateNoNode pins that a plain text channel
// alone does NOT inject an aggregate node — the committed text-mark
// fixtures must keep their pre-E4-S4 plan shape.
func TestPrismTextChannelNoAggregateNoNode(t *testing.T) {
	ga := buildTextSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"g": "a", "label": "A"}]},
	  "mark": {"type": "text"},
	  "encoding": {
	    "x": {"field": "g", "type": "nominal"},
	    "text": {"field": "label", "type": "nominal"}
	  }
	}`)
	if ga != nil {
		t.Errorf("tip is a GroupAggregateNode (groupby=%v); want none for an aggregate-free encoding", ga.Groupby())
	}
}

// TestPrismDuplicateEncodingAggregateCollapses pins the companion fix:
// two channels naming the SAME aggregate of the same field alias to
// one output column, so the injected node must emit one AggOp, not
// two. The natural E4-S4 spec — a text mark labelling its own
// aggregated y value — is exactly this shape.
func TestPrismDuplicateEncodingAggregateCollapses(t *testing.T) {
	ga := buildTextSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"g": "a", "score": 1}, {"g": "a", "score": 3}]},
	  "mark": {"type": "text"},
	  "encoding": {
	    "x": {"field": "g", "type": "nominal"},
	    "y": {"field": "score", "type": "quantitative", "aggregate": "mean"},
	    "text": {"field": "score", "type": "quantitative", "aggregate": "mean"}
	  }
	}`)
	if ga == nil {
		t.Fatal("tip is not a GroupAggregateNode")
	}
	if got := len(ga.Aggs()); got != 1 {
		t.Errorf("len(Aggs()) = %d, want 1 (duplicates collapsed); got %+v", got, ga.Aggs())
	}
}
