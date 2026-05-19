package passes

import "github.com/frankbardon/prism/plan"

// init registers the canonical 5-pass list with plan.DefaultPasses.
// Order = D047: semantics-preserving passes first, sampling last.
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
		SampleInjectionPass{},
	})
}
