package plan

import (
	"fmt"
	"io"
)

// RenderDOT emits a Graphviz-compatible representation of d to w.
//
// Output shape:
//
//	digraph prism_plan {
//	  rankdir=LR;
//	  node [shape=box, style=rounded];
//	  "<id>" [label="<kind>\n<shortid>\n<summary>"];
//	  "<from>" -> "<to>";
//	}
//
// Nodes and edges sort deterministically (lexicographic on NodeID and
// from/to pair respectively) so re-rendering identical DAGs produces
// byte-identical output — required for golden tests.
//
// Approximate footprint: ~50 LOC per design/05-dag-executor.md.
func RenderDOT(d *DAG, w io.Writer) error {
	if d == nil {
		return fmt.Errorf("RenderDOT: nil DAG")
	}
	if _, err := fmt.Fprintln(w, "digraph prism_plan {"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  rankdir=LR;"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "  node [shape=box, style=rounded];"); err != nil {
		return err
	}

	// Nodes (sorted via d.Nodes()). Labels carry DOT escape sequences
	// (\n, \") verbatim, so we wrap them in double quotes manually
	// instead of going through %q (which would re-escape the backslash).
	for _, id := range d.Nodes() {
		n, _ := d.Node(id)
		if _, err := fmt.Fprintf(w, "  \"%s\" [label=\"%s\"];\n", escapeDotID(string(id)), renderLabel(n)); err != nil {
			return err
		}
	}
	// Edges — for every node, emit one edge per input. Sort by node id
	// (outer) and use the input order from Inputs() (which is itself
	// declaration-stable for every node type).
	for _, id := range d.Nodes() {
		n, _ := d.Node(id)
		for _, in := range n.Inputs() {
			if _, err := fmt.Fprintf(w, "  \"%s\" -> \"%s\";\n",
				escapeDotID(string(in)), escapeDotID(string(id))); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(w, "}")
	return err
}
