package nodes

import (
	"context"
	"fmt"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// TimeUnitNode truncates a temporal field to a calendar period and
// appends the truncated date as a new column. Client-side (backend-
// compiled) like BinNode — pure epoch arithmetic, no Pulse leaf.
type TimeUnitNode struct {
	id      plan.NodeID
	input   plan.NodeID
	field   string
	unit    string
	as      string
	backend plan.Backend
}

// NewTimeUnit constructs a TimeUnitNode.
func NewTimeUnit(id, input plan.NodeID, field, unit, as string) *TimeUnitNode {
	return &TimeUnitNode{id: id, input: input, field: field, unit: unit, as: as}
}

// ID implements plan.Node.
func (n *TimeUnitNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *TimeUnitNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is input + one date field
// named n.as (the truncated period start for each row).
func (n *TimeUnitNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	s, err := requireSingleInput("TimeUnitNode", in)
	if err != nil {
		return nil, err
	}
	if n.as == "" {
		return nil, fmt.Errorf("TimeUnitNode: missing 'as' name")
	}
	return appendField(s, n.as, encoding.FieldTypeDate), nil
}

// Execute implements plan.Node via the injected backend.
func (n *TimeUnitNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("TimeUnitNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *TimeUnitNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *TimeUnitNode) Fingerprint() string {
	return fingerprintFor("TimeUnitNode", string(n.input), n.field, n.unit, n.as)
}

// Field exposes the source field for renderers + tests.
func (n *TimeUnitNode) Field() string { return n.field }

// Unit exposes the calendar period for the executor.
func (n *TimeUnitNode) Unit() string { return n.unit }

// As exposes the output field name for renderers + tests.
func (n *TimeUnitNode) As() string { return n.as }

// Kind implements plan.Labeled.
func (n *TimeUnitNode) Kind() string { return "TimeUnitNode" }

// Summary implements plan.Labeled.
func (n *TimeUnitNode) Summary() string {
	return n.as + " = timeunit(" + n.field + ", " + n.unit + ")"
}
