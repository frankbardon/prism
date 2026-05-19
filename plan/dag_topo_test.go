package plan_test

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/plan"
)

// flat joins level NodeIDs into one stable comma-separated string per
// level, then joins levels with " | ". Makes test assertions read like
// the DAG shape.
func flat(levels [][]plan.NodeID) string {
	parts := make([]string, len(levels))
	for i, level := range levels {
		strs := make([]string, len(level))
		for j, id := range level {
			strs[j] = string(id)
		}
		parts[i] = strings.Join(strs, ",")
	}
	return strings.Join(parts, " | ")
}

func TestPrismDAGTopoLevels(t *testing.T) {
	t.Run("linear", func(t *testing.T) {
		// src -> filter -> sort
		b := plan.NewBuilder()
		_ = b.AddNode(mkNode("src"))
		_ = b.AddNode(mkNode("filter", "src"))
		_ = b.AddNode(mkNode("sort", "filter"))
		_ = b.MarkRoot("src")
		_ = b.MarkSink("sort")
		d, _ := b.Build()
		levels, err := d.TopoLevels()
		if err != nil {
			t.Fatalf("TopoLevels: %v", err)
		}
		if got := flat(levels); got != "src | filter | sort" {
			t.Errorf("flat=%q", got)
		}
	})

	t.Run("fanout", func(t *testing.T) {
		// src -> {filter, limit}
		b := plan.NewBuilder()
		_ = b.AddNode(mkNode("src"))
		_ = b.AddNode(mkNode("filter", "src"))
		_ = b.AddNode(mkNode("limit", "src"))
		_ = b.MarkRoot("src")
		_ = b.MarkSink("filter")
		_ = b.MarkSink("limit")
		d, _ := b.Build()
		levels, err := d.TopoLevels()
		if err != nil {
			t.Fatalf("TopoLevels: %v", err)
		}
		if got := flat(levels); got != "src | filter,limit" {
			t.Errorf("flat=%q", got)
		}
	})

	t.Run("fanin", func(t *testing.T) {
		// {src1, src2} -> join
		b := plan.NewBuilder()
		_ = b.AddNode(mkNode("src1"))
		_ = b.AddNode(mkNode("src2"))
		_ = b.AddNode(mkNode("join", "src1", "src2"))
		_ = b.MarkRoot("src1")
		_ = b.MarkRoot("src2")
		_ = b.MarkSink("join")
		d, _ := b.Build()
		levels, err := d.TopoLevels()
		if err != nil {
			t.Fatalf("TopoLevels: %v", err)
		}
		if got := flat(levels); got != "src1,src2 | join" {
			t.Errorf("flat=%q", got)
		}
	})

	t.Run("diamond", func(t *testing.T) {
		// src -> {a, b} -> join
		b := plan.NewBuilder()
		_ = b.AddNode(mkNode("src"))
		_ = b.AddNode(mkNode("a", "src"))
		_ = b.AddNode(mkNode("b", "src"))
		_ = b.AddNode(mkNode("join", "a", "b"))
		_ = b.MarkRoot("src")
		_ = b.MarkSink("join")
		d, _ := b.Build()
		levels, err := d.TopoLevels()
		if err != nil {
			t.Fatalf("TopoLevels: %v", err)
		}
		if got := flat(levels); got != "src | a,b | join" {
			t.Errorf("flat=%q", got)
		}
	})
}
