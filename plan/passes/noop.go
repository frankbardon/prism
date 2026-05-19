// Package passes holds the optimizer passes. P03 ships an empty
// DefaultPasses registration (in plan/optimize.go) plus this NoopPass
// so the fixed-point loop has at least one test subject. P07 fills in
// DedupSources, FilterPushdown, ProjectionPruning, AggregateFusion,
// SampleInjection per design/05-dag-executor.md.
package passes

import "github.com/frankbardon/prism/plan"

// NoopPass returns the DAG unchanged. Useful as a baseline + as a way
// to exercise the Optimize loop without mutating state.
type NoopPass struct{}

// Name implements plan.Pass.
func (NoopPass) Name() string { return "noop" }

// Apply implements plan.Pass.
func (NoopPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	return d, false, nil
}
