package plan

import (
	"encoding/json"
	"fmt"
	"io"
)

// jsonNode is the wire shape for one node in the JSON-rendered DAG.
// Fields are ordered alphabetically by JSON tag for stable goldens.
type jsonNode struct {
	Fingerprint string   `json:"fingerprint"`
	ID          string   `json:"id"`
	Inputs      []string `json:"inputs"`
	Kind        string   `json:"kind"`
	Summary     string   `json:"summary,omitempty"`
}

// jsonDAG is the wire shape for the full DAG. Nodes is in topological
// order (level by level); ties within a level break lexicographically
// — same ordering the executor uses.
type jsonDAG struct {
	Nodes []jsonNode `json:"nodes"`
	Roots []string   `json:"roots"`
	Sinks []string   `json:"sinks"`
}

// RenderJSON emits a stable JSON serialisation of d to w. The output
// is consumed by goldens (plan visualisation regression tests, future
// optimizer-pass before/after snapshots) so the field set and ordering
// are part of the contract — change with care.
func RenderJSON(d *DAG, w io.Writer) error {
	if d == nil {
		return fmt.Errorf("RenderJSON: nil DAG")
	}
	levels, err := d.TopoLevels()
	if err != nil {
		return err
	}
	out := jsonDAG{
		Nodes: make([]jsonNode, 0, d.Size()),
		Roots: nodeIDsAsStrings(d.Roots()),
		Sinks: nodeIDsAsStrings(d.Sinks()),
	}
	for _, level := range levels {
		for _, id := range level {
			n, _ := d.Node(id)
			ins := make([]string, len(n.Inputs()))
			for i, in := range n.Inputs() {
				ins[i] = string(in)
			}
			out.Nodes = append(out.Nodes, jsonNode{
				ID:          string(id),
				Kind:        kindOf(n),
				Inputs:      ins,
				Fingerprint: n.Fingerprint(),
				Summary:     summaryOf(n),
			})
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func nodeIDsAsStrings(ids []NodeID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}
