package nodes

import (
	"context"
	"fmt"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// BinParams captures the optional knobs for a numeric bin operation.
// Auto=true (the bin: true shorthand) requests automatic bin selection;
// Maxbins, Step, and Extent override pieces of that selection.
type BinParams struct {
	Auto    bool
	Maxbins *int
	Step    *float64
	Extent  []float64
}

// String returns a deterministic text form for fingerprints.
func (b BinParams) String() string {
	mb := "-"
	if b.Maxbins != nil {
		mb = fmt.Sprintf("%d", *b.Maxbins)
	}
	st := "-"
	if b.Step != nil {
		st = fmt.Sprintf("%g", *b.Step)
	}
	return fmt.Sprintf("auto=%t,maxbins=%s,step=%s,extent=%v", b.Auto, mb, st, b.Extent)
}

// BinNode buckets a numeric field.
type BinNode struct {
	id      plan.NodeID
	input   plan.NodeID
	field   string
	as      string
	params  BinParams
	backend plan.Backend
}

// NewBin constructs a BinNode.
func NewBin(id, input plan.NodeID, field, as string, params BinParams) *BinNode {
	return &BinNode{id: id, input: input, field: field, as: as, params: params}
}

// ID implements plan.Node.
func (n *BinNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *BinNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is input + one F64 field
// named n.as (the bin edge for each row).
func (n *BinNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	s, err := requireSingleInput("BinNode", in)
	if err != nil {
		return nil, err
	}
	if n.as == "" {
		return nil, fmt.Errorf("BinNode: missing 'as' name")
	}
	return appendField(s, n.as, encoding.FieldTypeF64), nil
}

// Execute implements plan.Node via the injected backend.
func (n *BinNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("BinNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// Params exposes the bin parameters for the executor.
func (n *BinNode) Params() BinParams { return n.params }

// SetBackend wires the compile backend that powers Execute.
func (n *BinNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *BinNode) Fingerprint() string {
	return fingerprintFor("BinNode", string(n.input), n.field, n.as, n.params.String())
}

// Field exposes the source field for renderers + tests.
func (n *BinNode) Field() string { return n.field }

// As exposes the output field name for renderers + tests.
func (n *BinNode) As() string { return n.as }

// Kind implements plan.Labeled.
func (n *BinNode) Kind() string { return "BinNode" }

// Summary implements plan.Labeled.
func (n *BinNode) Summary() string { return n.as + " = bin(" + n.field + ")" }
