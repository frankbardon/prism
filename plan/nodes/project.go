package nodes

import (
	"context"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// ProjectNode keeps only the named fields from its input.
type ProjectNode struct {
	id      plan.NodeID
	input   plan.NodeID
	fields  []string
	backend plan.Backend
}

// NewProject constructs a ProjectNode. The fields slice is copied.
func NewProject(id, input plan.NodeID, fields []string) *ProjectNode {
	cp := make([]string, len(fields))
	copy(cp, fields)
	return &ProjectNode{id: id, input: input, fields: cp}
}

// ID implements plan.Node.
func (n *ProjectNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *ProjectNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is the projection of the
// input schema down to fields named in n.fields, in the requested
// order. Missing field names raise PRISM_PLAN_003.
func (n *ProjectNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	s, err := requireSingleInput("ProjectNode", in)
	if err != nil {
		return nil, err
	}
	return projectFields(s, n.fields)
}

// Execute implements plan.Node via the injected backend.
func (n *ProjectNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("ProjectNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *ProjectNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *ProjectNode) Fingerprint() string {
	return fingerprintFor("ProjectNode", string(n.input), strings.Join(n.fields, ","))
}

// Fields exposes the field list for renderers + tests.
func (n *ProjectNode) Fields() []string { return n.fields }

// Kind implements plan.Labeled.
func (n *ProjectNode) Kind() string { return "ProjectNode" }

// Summary implements plan.Labeled.
func (n *ProjectNode) Summary() string { return "fields: " + strings.Join(n.fields, ",") }
