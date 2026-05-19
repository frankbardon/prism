package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// UnpivotNode reshapes wide → long. P03 stub.
//
// Output schema is the input minus the unpivoted fields, plus two new
// fields: a categorical key column (named as[0], default "key") and a
// numeric value column (named as[1], default "value").
type UnpivotNode struct {
	id      plan.NodeID
	input   plan.NodeID
	unpivot []string
	as      []string
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
func (n *UnpivotNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
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
	out := &encoding.Schema{Fields: make([]encoding.Field, 0, len(s.Fields))}
	for i := range s.Fields {
		f := s.Fields[i]
		if _, drop := drop[f.Name]; drop {
			continue
		}
		out.Fields = append(out.Fields, f)
	}
	out.Fields = append(out.Fields,
		encoding.Field{Name: keyName, Type: encoding.FieldTypeCategoricalU8},
		encoding.Field{Name: valName, Type: encoding.FieldTypeF64},
	)
	if len(out.Fields) == 0 {
		return nil, fmt.Errorf("UnpivotNode: empty output schema")
	}
	return out, nil
}

// Execute implements plan.Node. P03 stub.
func (n *UnpivotNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("UnpivotNode")
}

// Fingerprint implements plan.Node.
func (n *UnpivotNode) Fingerprint() string {
	return fingerprintFor("UnpivotNode",
		string(n.input), strings.Join(n.unpivot, ","),
		strings.Join(n.as, ","),
	)
}

// Unpivot exposes the source fields for renderers + tests.
func (n *UnpivotNode) Unpivot() []string { return n.unpivot }
