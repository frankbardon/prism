package nodes

import (
	"context"
	"strconv"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// SampleNode draws a random subsample of size n.
type SampleNode struct {
	id      plan.NodeID
	input   plan.NodeID
	sample  int
	seed    *int64
	backend plan.Backend
}

// NewSample constructs a SampleNode. seed is optional (nil = unseeded).
func NewSample(id, input plan.NodeID, sample int, seed *int64) *SampleNode {
	return &SampleNode{id: id, input: input, sample: sample, seed: seed}
}

// ID implements plan.Node.
func (n *SampleNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *SampleNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Sampling preserves the schema.
func (n *SampleNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	return requireSingleInput("SampleNode", in)
}

// Execute implements plan.Node via the injected backend.
func (n *SampleNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("SampleNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *SampleNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *SampleNode) Fingerprint() string {
	seedStr := "-"
	if n.seed != nil {
		seedStr = strconv.FormatInt(*n.seed, 10)
	}
	return fingerprintFor("SampleNode", string(n.input), strconv.Itoa(n.sample), seedStr)
}

// N exposes the requested sample size for renderers + tests.
func (n *SampleNode) N() int { return n.sample }

// Seed exposes the (optional) deterministic RNG seed. nil means the
// executor seeds from wall-clock time.
func (n *SampleNode) Seed() *int64 { return n.seed }

// Kind implements plan.Labeled.
func (n *SampleNode) Kind() string { return "SampleNode" }

// Summary implements plan.Labeled.
func (n *SampleNode) Summary() string { return "n: " + strconv.Itoa(n.sample) }
