package nodes

import (
	"context"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// ProjectNode keeps only the named fields from its input. P03 stub.
type ProjectNode struct {
	id     plan.NodeID
	input  plan.NodeID
	fields []string
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

// Execute implements plan.Node. P03 stub.
func (n *ProjectNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("ProjectNode")
}

// Fingerprint implements plan.Node.
func (n *ProjectNode) Fingerprint() string {
	return fingerprintFor("ProjectNode", string(n.input), strings.Join(n.fields, ","))
}

// Fields exposes the field list for renderers + tests.
func (n *ProjectNode) Fields() []string { return n.fields }
