package plan

import (
	"fmt"
	"io"
	"sort"
)

// RenderText emits an indented, tree-style listing of d to w rooted at
// each sink, walking inputs depth-first. Two-space indent per depth
// level. Intended for terminal output — concise, copy-paste friendly.
func RenderText(d *DAG, w io.Writer) error {
	if d == nil {
		return fmt.Errorf("RenderText: nil DAG")
	}
	sinks := d.Sinks()
	sort.Slice(sinks, func(i, j int) bool { return sinks[i] < sinks[j] })
	for _, sink := range sinks {
		if err := writeTextNode(d, sink, 0, w); err != nil {
			return err
		}
	}
	return nil
}

func writeTextNode(d *DAG, id NodeID, depth int, w io.Writer) error {
	n, ok := d.Node(id)
	if !ok {
		return fmt.Errorf("RenderText: node %s missing", id)
	}
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	sum := summaryOf(n)
	suffix := ""
	if sum != "" {
		suffix = "  // " + sum
	}
	if _, err := fmt.Fprintf(w, "%s%s [%s]%s\n", indent, kindOf(n), shortID(id), suffix); err != nil {
		return err
	}
	for _, in := range n.Inputs() {
		if err := writeTextNode(d, in, depth+1, w); err != nil {
			return err
		}
	}
	return nil
}
