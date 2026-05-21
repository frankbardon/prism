package passes

import (
	"encoding/json"
	"strings"

	pulseprocessing "github.com/frankbardon/pulse/processing"
	pulsetypes "github.com/frankbardon/pulse/types"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// PulseChainFusionPass collapses a source-rooted linear chain of
// fusable Prism nodes into one PulseChainNode that calls
// pulse.ProcessChain in a single round-trip. The win is that the
// upstream SourceNode no longer materialises the full cohort into
// table.Table — Pulse pushes filters/aggregates down to its columnar
// reader and returns only the final result rows.
//
// v1 scope (intentionally narrow):
//   - Source must be a single dependent (no branching).
//   - Eligible absorbed kinds: FilterNode, CalculateNode,
//     GroupAggregateNode, SortNode.
//   - GroupAggregate is REQUIRED. A chain with only Filter/Calculate
//     would still pay the source materialisation cost on the Prism
//     side (final result == filtered source ≈ full source rows), so
//     the win is dominated by aggregation. Defer pure filter/calc
//     chains to a follow-up if profiling motivates.
//   - Aggregate alias must be Pulse-backed and scalar-emitting
//     (`mode`, `lift`, `share` are excluded by Pulse's chain gate;
//     `wmean`, `ratio`, `ci0`, `ci1` carry per-call Params and are
//     deferred to a follow-up).
//   - SortNode terminates absorption.
//   - LimitNode is NOT absorbed (Pulse Request has no limit field
//     today); a downstream Limit stays in plan and consumes the
//     chain node's output.
//   - Filter / Calculate AFTER the GroupAggregate are not absorbed
//     (Pulse Filterers/Attributes run pre-aggregation; post-agg
//     filtering needs a second chain stage which v1 does not emit).
//   - Source refs starting with `cohort:` or `gs://` are skipped
//     (the chain executor needs a direct Pulse-compatible path).
//
// Pass ordering (plan/passes/register.go): runs after
// AggregateFusion (so sibling GroupAggregates are already merged
// into one before we try to absorb) and before SampleInjection (so
// chain-fused sources are no longer SourceNodes when the sampler
// looks for them; the row-count probe is unnecessary once Pulse is
// doing the heavy lifting).
type PulseChainFusionPass struct{}

// Name implements plan.Pass.
func (PulseChainFusionPass) Name() string { return "pulse_chain_fusion" }

// Apply implements plan.Pass. Walks each SourceNode root in turn and
// fuses the first eligible linear chain it finds, then returns —
// the fixed-point loop in plan.Optimize re-runs Apply until no more
// fusions happen.
func (p PulseChainFusionPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	for _, rootID := range d.Roots() {
		root, ok := d.Node(rootID)
		if !ok {
			continue
		}
		src, ok := root.(*nodes.SourceNode)
		if !ok {
			continue
		}
		if !pulseChainEligibleRef(src.Ref()) {
			continue
		}
		out, changed := tryFuseChainFromSource(d, src)
		if changed {
			return out, true, nil
		}
	}
	return d, false, nil
}

// pulseChainEligibleRef returns true when the source ref is in a form
// pulse.ProcessChain accepts directly (plain path or archive#shard
// anchor). cohort: and gs:// refs would need extra resolution that
// v1 fusion does not perform.
func pulseChainEligibleRef(ref string) bool {
	if strings.HasPrefix(ref, "gs://") {
		return false
	}
	if strings.HasPrefix(ref, "cohort:") {
		return false
	}
	return ref != ""
}

// tryFuseChainFromSource walks the linear chain rooted at src,
// absorbs eligible nodes into a single Pulse Request, and returns
// the rewired DAG. Returns (d, false) if the chain is not eligible
// (e.g. no GroupAggregate present, branch detected, alias rejected).
func tryFuseChainFromSource(d *plan.DAG, src *nodes.SourceNode) (*plan.DAG, bool) {
	deps := d.Dependents(src.ID())
	if len(deps) != 1 {
		return d, false
	}

	// Walk the linear chain absorbing each eligible node. State
	// tracks whether we've already absorbed a GroupAggregate (after
	// which Filter/Calculate are illegal) and whether we've absorbed
	// a SortNode (which must be terminal).
	var (
		req          = &pulsetypes.Request{}
		absorbed     []plan.NodeID
		hasGroupAgg  bool
		hasSort      bool
		summaryParts = []string{src.Ref()}
		lastAbsorbed = src.ID()
	)

	currentID := deps[0]
	for {
		node, ok := d.Node(currentID)
		if !ok {
			break
		}
		// Branching kills fusion: the absorbed node cannot have
		// extra dependents we'd orphan. The very last absorbed node
		// is allowed to have multiple dependents (those rewire to
		// the chain node), so we check this BEFORE attempting to
		// continue past `currentID`.
		nextDeps := d.Dependents(currentID)
		canContinue := len(nextDeps) == 1

		switch n := node.(type) {
		case *nodes.FilterNode:
			if hasGroupAgg || hasSort {
				return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
			}
			req.Filterers = append(req.Filterers, &pulsetypes.Filterer{
				Type:       pulsetypes.FILTER_EXPRESSION,
				Expression: n.Expr(),
			})
			summaryParts = append(summaryParts, "filter("+n.Expr()+")")
		case *nodes.CalculateNode:
			if hasGroupAgg || hasSort {
				return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
			}
			req.Attributes = append(req.Attributes, &pulsetypes.Attribute{
				Type:       pulsetypes.ATTR_FORMULA,
				Expression: n.Expr(),
				Label:      n.As(),
				Field:      n.As(),
			})
			summaryParts = append(summaryParts, "calc("+n.As()+")")
		case *nodes.GroupAggregateNode:
			if hasGroupAgg || hasSort {
				return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
			}
			if !translateGroupAggregate(req, n) {
				return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
			}
			hasGroupAgg = true
			summaryParts = append(summaryParts, "groupagg("+strings.Join(n.Groupby(), ",")+")")
		case *nodes.SortNode:
			if hasSort {
				return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
			}
			for _, k := range n.Sort() {
				req.Sort = append(req.Sort, pulsetypes.OrderKey{Field: k.Field, Desc: k.Order == "desc"})
			}
			hasSort = true
			summaryParts = append(summaryParts, "sort("+n.SortLabel()+")")
		default:
			return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
		}

		absorbed = append(absorbed, currentID)
		lastAbsorbed = currentID

		if !canContinue || hasSort {
			return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
		}
		currentID = nextDeps[0]
	}
	return finaliseFusion(d, src, req, absorbed, summaryParts, lastAbsorbed)
}

// finaliseFusion materialises the captured request into a
// PulseChainNode, swaps it into the DAG, and rewires the tail's
// downstream dependents.
func finaliseFusion(
	d *plan.DAG,
	src *nodes.SourceNode,
	req *pulsetypes.Request,
	absorbed []plan.NodeID,
	summaryParts []string,
	lastAbsorbed plan.NodeID,
) (*plan.DAG, bool) {
	if len(absorbed) == 0 {
		return d, false
	}
	// v1 win condition: chain must absorb at least one
	// GroupAggregate. Pure filter / calculate chains skip fusion.
	if len(req.Aggregations) == 0 && len(req.Groups) == 0 {
		return d, false
	}

	req.Cohort = &pulsetypes.Cohort{Filename: src.Ref()}
	chainReq := &pulsetypes.ChainRequest{
		Cohort: req.Cohort,
		Stages: []*pulsetypes.ChainStage{
			{Name: "fused", Request: req},
		},
	}

	// Pre-flight: the chain gate must accept this stage. The gate
	// needs a schema, but the source schema requires opening the
	// cohort; we have a stub schema only at pass time. Skip the
	// pre-flight call here — Pulse will surface
	// PULSE_CHAIN_NOT_MERGEABLE at execute time if a stage fails,
	// and the chain node wraps it as PRISM_PLAN_CHAIN_NOT_MERGEABLE.
	// The cheap scalar-only check still applies (caught in
	// translateGroupAggregate).

	outSchema, err := pulseprocessing.ChainOutputSchema(req)
	if err != nil {
		return d, false
	}

	chainID := nodes.DerivePulseChainID(src.Ref(), absorbed)
	chainNode := nodes.NewPulseChain(
		chainID,
		src.Ref(),
		sourceFs(src),
		chainReq,
		outSchema,
		strings.Join(summaryParts, " -> "),
		absorbed,
	)

	out := d
	// Detect whether the tail was a sink before we drop it.
	tailWasSink := isSink(out, lastAbsorbed)
	// Drop the source and every absorbed node.
	out = out.WithoutNode(src.ID())
	for _, id := range absorbed {
		out = out.WithoutNode(id)
	}
	// Re-add the chain node and mark it as a root.
	out = out.WithNode(chainNode).WithRootAdded(chainID)
	if tailWasSink {
		out = out.WithSinkAdded(chainID)
	}
	// Rewire any dependent of the tail that survived (i.e. not absorbed).
	for _, depID := range d.Dependents(lastAbsorbed) {
		if isAbsorbed(absorbed, depID) {
			continue
		}
		depNode, ok := out.Node(depID)
		if !ok {
			continue
		}
		if rebuilt := rewireChainConsumer(depNode, lastAbsorbed, chainID); rebuilt != nil {
			out = out.WithNode(rebuilt)
		}
	}
	return out, true
}

// translateGroupAggregate folds a GroupAggregateNode into req. Returns
// false if any AggOp uses an alias the v1 chain gate cannot accept:
//   - aliases not in compile.AliasToPulse;
//   - aliases marked deferred (lift, share);
//   - `mode` (Pulse's chain gate excludes AGG_MODE because it emits
//     a string per group, not a scalar f64);
//   - aliases whose Params depend on per-call sibling-column
//     conventions (wmean, ratio, ci0, ci1) — request builders need
//     extra context the pass does not have today.
func translateGroupAggregate(req *pulsetypes.Request, n *nodes.GroupAggregateNode) bool {
	for _, field := range n.Groupby() {
		req.Groups = append(req.Groups, &pulsetypes.Group{
			Type:  pulsetypes.GROUP_CATEGORY,
			Field: field,
		})
	}
	for _, op := range n.Aggs() {
		alias := strings.ToLower(op.Op)
		mapping, ok := compile.AliasToPulse[alias]
		if !ok || mapping.IsDeferredFromPulse() {
			return false
		}
		if alias == "mode" {
			return false
		}
		if alias == "wmean" || alias == "ratio" || alias == "ci0" || alias == "ci1" {
			return false
		}
		agg := &pulsetypes.Aggregation{
			Type:  mapping.Type,
			Field: op.Field,
			Label: op.As,
		}
		if len(mapping.Params) > 0 {
			agg.Params = json.RawMessage(append([]byte(nil), mapping.Params...))
		}
		req.Aggregations = append(req.Aggregations, agg)
	}
	return true
}

// rewireChainConsumer rebuilds a dependent of the chain tail so its
// first input points at the new chain node. Returns nil for kinds the
// pass does not recognise — those dependents simply keep their old
// input pointer and the fixed-point loop will surface the dangling
// reference at Build / Execute time (defensive only; the v1 plan
// shapes Vega-Lite emits never produce such dependents).
func rewireChainConsumer(n plan.Node, oldIn, newIn plan.NodeID) plan.Node {
	if rebuilt := rewireSingleInput(n, oldIn, newIn); rebuilt != nil {
		return rebuilt
	}
	switch v := n.(type) {
	case *nodes.LimitNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewLimit(v.ID(), newIn, v.Limit(), v.Offset())
	case *nodes.SortNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewSort(v.ID(), newIn, v.Sort())
	case *nodes.CalculateNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewCalculate(v.ID(), newIn, v.Expr(), v.As())
	case *nodes.SampleNode:
		if v.Inputs()[0] != oldIn {
			return nil
		}
		return nodes.NewSample(v.ID(), newIn, v.N(), v.Seed())
	}
	return nil
}

// isAbsorbed reports whether id appears in the absorbed slice.
func isAbsorbed(absorbed []plan.NodeID, id plan.NodeID) bool {
	for _, a := range absorbed {
		if a == id {
			return true
		}
	}
	return false
}

// isSink reports whether id is listed in d.Sinks().
func isSink(d *plan.DAG, id plan.NodeID) bool {
	for _, s := range d.Sinks() {
		if s == id {
			return true
		}
	}
	return false
}

// sourceFs returns the SourceNode's afero filesystem so the
// PulseChainNode can call pulse.New with the same fs the resolver
// would have used. The single getter on SourceNode is enough; this
// thin wrapper keeps the pass file readable.
func sourceFs(src *nodes.SourceNode) afero.Fs { return src.FS() }
