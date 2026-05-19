package passes

import (
	"github.com/frankbardon/prism/plan"
)

// SampleInjectionPass would inject a SampleNode below every Source
// whose materialised row count exceeds PRISM_RENDER_MAX_MARKS, emitting
// PRISM_WARN_DOWNSAMPLE. v1 ships the no-op baseline because the row
// count of a Source is not knowable without executing it: Pulse's
// public surface in v0.8.4 does not expose a record-count header field,
// and reading the entire payload defeats the purpose of an
// optimization pass.
//
// The pass stays registered in plan.DefaultPasses so the fixed-point
// loop sees it; it always reports `changed=false`. A future iteration
// (gated on Pulse exposing a record-count) flips the body active.
//
// When this pass becomes active, it must:
//
//  1. Walk plan.DAG.Roots(), filter to SourceNodes.
//  2. For each Source whose declared row count exceeds
//     limits.MustRenderMaxMarks(), inject a nodes.SampleNode below it.
//  3. Append a warning to the eventual ExecResult.Warnings channel
//     (PRISM_WARN_DOWNSAMPLE) so users see the downsampling.
type SampleInjectionPass struct{}

// Name implements plan.Pass.
func (SampleInjectionPass) Name() string { return "sample_injection" }

// Apply implements plan.Pass. P07 baseline: no-op.
func (SampleInjectionPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	return d, false, nil
}
