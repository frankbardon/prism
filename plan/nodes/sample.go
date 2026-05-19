package nodes

import (
	"context"
	"strconv"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// SampleNode draws a random subsample of size n. P03 stub.
type SampleNode struct {
	id     plan.NodeID
	input  plan.NodeID
	sample int
	seed   *int64
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

// Execute implements plan.Node. P03 stub.
func (n *SampleNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("SampleNode")
}

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

// Kind implements plan.Labeled.
func (n *SampleNode) Kind() string { return "SampleNode" }

// Summary implements plan.Labeled.
func (n *SampleNode) Summary() string { return "n: " + strconv.Itoa(n.sample) }
