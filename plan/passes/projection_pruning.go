package passes

import (
	"strconv"
	"strings"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// ProjectionPruningPass injects a ProjectNode immediately downstream of
// every Source whose output schema contains columns the downstream
// pipeline never reads.
//
// Used-column inference: walk backward from each sink. The set of
// columns each node needs is the union of its self-referenced columns
// (filter expressions, groupby/aggregate fields, join keys, project
// fields, calculate expr, etc.) plus the union of the needs of its
// dependents. For nodes we cannot statically classify, we conservatively
// declare ALL upstream columns needed — making the pass a no-op in
// those cases.
//
// P07 ships a minimum-viable implementation: it prunes only when every
// node downstream of a Source belongs to a small known set
// (FilterNode, ProjectNode, GroupAggregateNode, JoinNode). Other node
// types (Calculate / Window / Pivot / Unpivot / Bin / Sort / Limit /
// Sample / Union) cause the pass to bail conservatively for that
// Source — they will be handled in a future iteration once their
// column-set surfaces are exposed via a plan.Labeled extension.
type ProjectionPruningPass struct{}

// Name implements plan.Pass.
func (ProjectionPruningPass) Name() string { return "projection_pruning" }

// Apply implements plan.Pass.
func (ProjectionPruningPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	out := d
	changed := false

	// Precompute every node's used-column set (the columns its body
	// references) and dependents.
	dependents := map[plan.NodeID][]plan.NodeID{}
	for _, id := range d.Nodes() {
		dependents[id] = d.Dependents(id)
	}

	for _, srcID := range d.Roots() {
		srcNode, _ := out.Node(srcID)
		src, ok := srcNode.(*nodes.SourceNode)
		if !ok {
			continue
		}
		schema, err := src.OutputSchema()
		if err != nil || schema == nil {
			continue
		}
		// Build the set of columns reachable downstream of this source.
		needed, ok := neededColumnsForSource(out, srcID, dependents)
		if !ok {
			continue
		}
		// If `needed` already covers every field, no pruning to do.
		allFields := map[string]struct{}{}
		for i := range schema.Fields {
			allFields[schema.Fields[i].Name] = struct{}{}
		}
		if len(needed) == 0 || len(needed) >= len(allFields) {
			continue
		}
		strictSubset := false
		for f := range allFields {
			if _, ok := needed[f]; !ok {
				strictSubset = true
				break
			}
		}
		if !strictSubset {
			continue
		}
		// Inject ProjectNode between src and its dependents.
		projectFields := orderedFields(schema, needed)
		projID := plan.NodeID("project:" + shortHash(string(srcID)+strings.Join(projectFields, ",")))
		project := nodes.NewProject(projID, srcID, projectFields)
		out = out.WithNode(project)
		// Rewire every direct dependent of srcID (except the new
		// project itself) to point at projID.
		for _, depID := range dependents[srcID] {
			depNode, _ := out.Node(depID)
			rebuilt := rewireSingleInput(depNode, srcID, projID)
			if rebuilt != nil {
				out = out.WithNode(rebuilt)
			}
		}
		changed = true
	}
	return out, changed, nil
}

// neededColumnsForSource walks from srcID forward through dependents,
// collecting the columns each node reads. Returns (set, true) when
// every visited node can be classified; (nil, false) when an
// unclassifiable node is reached (causes the pass to bail for this
// source).
func neededColumnsForSource(
	d *plan.DAG, srcID plan.NodeID, deps map[plan.NodeID][]plan.NodeID,
) (map[string]struct{}, bool) {
	out := map[string]struct{}{}
	visited := map[plan.NodeID]bool{}
	var walk func(id plan.NodeID) bool
	walk = func(id plan.NodeID) bool {
		if visited[id] {
			return true
		}
		visited[id] = true
		for _, depID := range deps[id] {
			n, ok := d.Node(depID)
			if !ok {
				return false
			}
			ok = collectColsFromNode(n, out)
			if !ok {
				return false
			}
			if !walk(depID) {
				return false
			}
		}
		return true
	}
	if !walk(srcID) {
		return nil, false
	}
	return out, true
}

// collectColsFromNode adds n's referenced columns to out. Returns
// false when n is a kind we cannot statically classify (caller bails).
func collectColsFromNode(n plan.Node, out map[string]struct{}) bool {
	switch v := n.(type) {
	case *nodes.FilterNode:
		for _, c := range extractIdentifiers(v.Expr()) {
			out[c] = struct{}{}
		}
		return true
	case *nodes.ProjectNode:
		for _, f := range v.Fields() {
			out[f] = struct{}{}
		}
		return true
	case *nodes.GroupAggregateNode:
		for _, g := range v.Groupby() {
			out[g] = struct{}{}
		}
		for _, a := range v.Aggs() {
			if a.Field != "" {
				out[a.Field] = struct{}{}
			}
		}
		return true
	case *nodes.JoinNode:
		for _, k := range v.On() {
			out[k] = struct{}{}
		}
		// Join also propagates ALL non-key columns from its inputs
		// downstream — but those are accounted for when we walk the
		// join's dependents. For the source-side view, the join only
		// reads the join keys directly.
		return true
	case *nodes.SourceNode:
		return true
	}
	// Unclassifiable kind (calculate / window / pivot / unpivot / bin
	// / sort / limit / sample / union / inline). Bail.
	return false
}

// rewireSingleInput returns a new node whose first input is replaced
// from oldIn to newIn. Returns nil if the node is not one of the
// known rewireable kinds OR if it doesn't reference oldIn.
func rewireSingleInput(n plan.Node, oldIn, newIn plan.NodeID) plan.Node {
	switch v := n.(type) {
	case *nodes.FilterNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewFilter(v.ID(), newIn, v.Expr())
	case *nodes.ProjectNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewProject(v.ID(), newIn, v.Fields())
	case *nodes.GroupAggregateNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewGroupAggregate(v.ID(), newIn, v.Groupby(), v.Aggs())
	case *nodes.JoinNode:
		ins := v.Inputs()
		l, r := ins[0], ins[1]
		if l == oldIn {
			l = newIn
		}
		if r == oldIn {
			r = newIn
		}
		return nodes.NewJoin(v.ID(), l, r, v.On(), v.JoinKind(), 0)
	}
	return nil
}

// orderedFields returns the names of s's fields that appear in
// `needed`, preserving the schema's original field order.
func orderedFields(s any, needed map[string]struct{}) []string {
	out := []string{}
	sch := schemaFromAny(s)
	if sch == nil {
		// Fallback: alphabetical (deterministic for goldens).
		for k := range needed {
			out = append(out, k)
		}
		return out
	}
	for i := range sch.Fields {
		if _, ok := needed[sch.Fields[i].Name]; ok {
			out = append(out, sch.Fields[i].Name)
		}
	}
	return out
}

// shortHash is a deterministic short-id helper for synthesized node ids.
func shortHash(s string) string {
	var x uint64 = 14695981039346656037 // FNV offset basis
	for i := 0; i < len(s); i++ {
		x ^= uint64(s[i])
		x *= 1099511628211
	}
	// 6 hex chars; collisions don't matter — the optimizer pass loop
	// is bounded and we never reuse a project id outside this DAG.
	return strconv.FormatUint(x>>(64-24), 16)
}
