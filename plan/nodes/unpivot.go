package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// UnpivotNode reshapes wide → long.
//
// Output schema is the input minus the unpivoted fields, plus two new
// fields: a categorical key column (named as[0], default "key") and a
// numeric value column (named as[1], default "value").
//
// Execute routes through the injected backend (compile/inmem owns the
// reshape); it falls back to PRISM_COMPILE_001 when no backend is
// wired, preserving the P03 stub behaviour for callers that construct
// the node by hand.
type UnpivotNode struct {
	id      plan.NodeID
	input   plan.NodeID
	unpivot []string
	as      []string
	backend plan.Backend
}

// NewUnpivot constructs an UnpivotNode.
func NewUnpivot(id, input plan.NodeID, unpivot, as []string) *UnpivotNode {
	up := make([]string, len(unpivot))
	copy(up, unpivot)
	a := make([]string, len(as))
	copy(a, as)
	return &UnpivotNode{id: id, input: input, unpivot: up, as: a}
}

// ID implements plan.Node.
func (n *UnpivotNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *UnpivotNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Drops the unpivoted fields from the
// input schema and appends two new fields (key column + value column).
func (n *UnpivotNode) Schema(in []*table.Schema) (*table.Schema, error) {
	s, err := requireSingleInput("UnpivotNode", in)
	if err != nil {
		return nil, err
	}
	drop := map[string]struct{}{}
	for _, f := range n.unpivot {
		drop[f] = struct{}{}
	}
	keyName, valName := "key", "value"
	if len(n.as) > 0 && n.as[0] != "" {
		keyName = n.as[0]
	}
	if len(n.as) > 1 && n.as[1] != "" {
		valName = n.as[1]
	}
	out := &table.Schema{Fields: make([]table.Field, 0, len(s.Fields))}
	for i := range s.Fields {
		f := s.Fields[i]
		if _, drop := drop[f.Name]; drop {
			continue
		}
		out.Fields = append(out.Fields, f)
	}
	out.Fields = append(out.Fields,
		table.Field{Name: keyName, Type: table.FieldTypeCategoricalU8},
		table.Field{Name: valName, Type: table.FieldTypeF64},
	)
	if len(out.Fields) == 0 {
		return nil, fmt.Errorf("UnpivotNode: empty output schema")
	}
	return out, nil
}

// Execute implements plan.Node. Routes through the injected backend
// when one is wired; returns PRISM_COMPILE_001 otherwise.
func (n *UnpivotNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("UnpivotNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute. The
// builder calls this after construction so node constructors keep
// their P03 signatures stable. See D033.
func (n *UnpivotNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *UnpivotNode) Fingerprint() string {
	return fingerprintFor("UnpivotNode",
		string(n.input), strings.Join(n.unpivot, ","),
		strings.Join(n.as, ","),
	)
}

// Unpivot exposes the source fields for renderers + tests.
func (n *UnpivotNode) Unpivot() []string { return n.unpivot }

// Kind implements plan.Labeled.
func (n *UnpivotNode) Kind() string { return "UnpivotNode" }

// Summary implements plan.Labeled.
func (n *UnpivotNode) Summary() string {
	return "fields: " + strings.Join(n.unpivot, ",")
}
