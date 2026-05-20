package passes

import (
	"context"

	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// SampleInjectionPass injects a SampleNode below every Source whose
// header-reported row count exceeds PRISM_RENDER_MAX_MARKS. Pulse
// v0.10.0 exposes the count via pulse.CountRecords; SourceNode.RowCount
// wraps it. Sources whose count cannot be read (missing file, unsupported
// resolver) skip injection silently — the executor will surface the
// underlying error.
//
// The fixed-point loop is guarded against re-entry: a Source with a
// SampleNode dependent is considered already-sampled and skipped.
//
// Dependents of the rewired Source are reconstructed via
// rewireSingleInput; node kinds not covered by that helper (Calculate,
// Window, Sort, Limit, Bin, Sample, Pivot, Unpivot) are silently left
// pointing at the Source. The fixed-point loop still terminates because
// the Source already has a SampleNode dependent (the injected one).
type SampleInjectionPass struct{}

// Name implements plan.Pass.
func (SampleInjectionPass) Name() string { return "sample_injection" }

// Apply implements plan.Pass.
func (SampleInjectionPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	max := limits.MustRenderMaxMarks()
	out := d
	changed := false
	for _, id := range d.Roots() {
		n, ok := out.Node(id)
		if !ok {
			continue
		}
		src, ok := n.(*nodes.SourceNode)
		if !ok {
			continue
		}
		if hasSampleDescendant(out, id) {
			continue
		}
		count, err := src.RowCount(context.Background())
		if err != nil {
			continue
		}
		if int(count) <= max {
			continue
		}
		sampleID := plan.NodeID("sample:" + shortHash(string(id)))
		sample := nodes.NewSample(sampleID, id, max, nil)
		out = out.WithNode(sample)
		for _, depID := range out.Dependents(id) {
			if depID == sampleID {
				continue
			}
			depNode, ok := out.Node(depID)
			if !ok {
				continue
			}
			if rebuilt := rewireSingleInput(depNode, id, sampleID); rebuilt != nil {
				out = out.WithNode(rebuilt)
			}
		}
		changed = true
	}
	return out, changed, nil
}

// hasSampleDescendant reports whether any direct dependent of id is
// already a SampleNode. Cheaper than a full descendant walk; sufficient
// because the pass only ever injects a Sample as a direct dependent.
func hasSampleDescendant(d *plan.DAG, id plan.NodeID) bool {
	for _, depID := range d.Dependents(id) {
		dep, ok := d.Node(depID)
		if !ok {
			continue
		}
		if _, ok := dep.(*nodes.SampleNode); ok {
			return true
		}
	}
	return false
}
