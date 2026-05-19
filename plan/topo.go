package plan

import (
	"fmt"
	"sort"

	prismerrors "github.com/frankbardon/prism/errors"
)

// TopoLevels runs Kahn's algorithm over the DAG and returns the result
// as a slice of levels. Each level is a slice of NodeIDs whose
// upstream dependencies have all been scheduled in earlier levels;
// the executor consumes one level at a time.
//
// Within a level, IDs are sorted lexicographically so test goldens
// stay deterministic. Across runs, the level shape is identical given
// identical inputs.
//
// On a cyclic graph (Kahn cannot schedule every node), returns a
// PRISM_PLAN_001 AppError whose context carries one representative
// id from the cycle and the count of unscheduled nodes.
func (d *DAG) TopoLevels() ([][]NodeID, error) {
	indeg := make(map[NodeID]int, len(d.nodes))
	for id, n := range d.nodes {
		indeg[id] = len(n.Inputs())
	}

	ready := []NodeID{}
	for id, k := range indeg {
		if k == 0 {
			ready = append(ready, id)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i] < ready[j] })

	var levels [][]NodeID
	seen := 0
	for len(ready) > 0 {
		levels = append(levels, ready)
		seen += len(ready)
		next := []NodeID{}
		// For each scheduled id, decrement indegree of dependents.
		for _, id := range ready {
			for _, dep := range d.Dependents(id) {
				indeg[dep]--
				if indeg[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
		ready = next
	}

	if seen != len(d.nodes) {
		var first NodeID
		for id, k := range indeg {
			if k > 0 {
				if first == "" || id < first {
					first = id
				}
			}
		}
		return nil, prismerrors.New(
			"PRISM_PLAN_001",
			fmt.Sprintf("Cyclic dataset reference detected (involving %s; %d nodes unscheduled).",
				first, len(d.nodes)-seen),
			map[string]any{"Cycle": string(first), "Nodes": len(d.nodes) - seen},
		)
	}
	return levels, nil
}
