package passes

import "github.com/frankbardon/prism/plan"

// init registers the canonical pass list with plan.DefaultPasses.
// Order = D047: semantics-preserving passes first, sampling last.
// PulseChainFusion slots between AggregateFusion (so sibling
// GroupAggregates merge first) and SampleInjection (so chain-fused
// sources are no longer SourceNodes when the sampler walks roots).
//
// Side-effect package init is the only avenue here — plan/passes/
// imports plan/, so plan/ cannot import passes/ directly without
// creating a cycle. The init runs at process start whenever any
// caller imports plan/passes; the CLI imports it transitively via
// the plot/execute commands.
func init() {
	plan.SetDefaultPasses([]plan.Pass{
		DedupSourcesPass{},
		FilterPushdownPass{},
		ProjectionPruningPass{},
		AggregateFusionPass{},
		PulseChainFusionPass{},
		SampleInjectionPass{},
	})
}
