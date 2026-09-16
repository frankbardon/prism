package nodes

import (
	"context"
	"fmt"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// StackNode accumulates one quantitative column into per-row [start,
// end] bounds within each stack (E5-S2).
//
// It is an ordinary linear transform node: it consumes ANY upstream
// table — a materialised leaf, the synthetic encoding aggregate, or
// the tip of a transform chain — and passes every input column through
// untouched, appending exactly two F64 columns. The output schema is
// therefore derived at execute time from the upstream schema, the way
// GroupAggregateNode derives its own.
//
// Stacks are delimited by Groupby (the dimension position channel).
// Within one stack, segments order by the first-appearance rank of the
// StackBy tuple across the whole table — the same ordering contract
// encode/marks/group.go's groupRows publishes (colour first-appearance
// outer, full-tuple first-appearance inner), so every stack orders its
// segments identically and in legend order. With no StackBy field
// bound, upstream row order is preserved, which an author shapes with
// a preceding `sort` transform.
//
// Positive and negative values accumulate independently from zero, so
// a stack holding both grows up and down from the baseline rather than
// cancelling.
type StackNode struct {
	id      plan.NodeID
	input   plan.NodeID
	field   string
	groupby []string
	stackBy []string
	offset  string
	startAs string
	endAs   string
	backend plan.Backend
}

// NewStack constructs a StackNode. All slices are copied. An empty
// offset defaults to "zero"; empty output names default to
// "<field>_start" / "<field>_end".
func NewStack(id, input plan.NodeID, field string, groupby, stackBy []string, offset, startAs, endAs string) *StackNode {
	gb := make([]string, len(groupby))
	copy(gb, groupby)
	sb := make([]string, len(stackBy))
	copy(sb, stackBy)
	if offset == "" {
		offset = "zero"
	}
	if startAs == "" {
		startAs = field + "_start"
	}
	if endAs == "" {
		endAs = field + "_end"
	}
	return &StackNode{
		id:      id,
		input:   input,
		field:   field,
		groupby: gb,
		stackBy: sb,
		offset:  offset,
		startAs: startAs,
		endAs:   endAs,
	}
}

// ID implements plan.Node.
func (n *StackNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *StackNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node: the upstream schema plus the two F64
// bound columns. Derived at execute time from the real upstream
// schema, so the node composes after any transform.
func (n *StackNode) Schema(in []*table.Schema) (*table.Schema, error) {
	s, err := requireSingleInput("StackNode", in)
	if err != nil {
		return nil, err
	}
	if n.field == "" {
		return nil, fmt.Errorf("StackNode: missing field to stack")
	}
	found := false
	for i := range s.Fields {
		switch s.Fields[i].Name {
		case n.startAs, n.endAs:
			return nil, stackOutputCollisionErr(s.Fields[i].Name, n.field)
		}
		if s.Fields[i].Name == n.field {
			found = true
		}
	}
	if !found {
		return nil, prismerrors.New(
			"PRISM_PLAN_STACK_FIELD_MISSING",
			fmt.Sprintf("Stack field %q is not present in the upstream table.", n.field),
			map[string]any{"Field": n.field, "Available": fieldNameList(s)},
		)
	}
	out := cloneSchema(s)
	out.Fields = append(out.Fields,
		table.Field{Name: n.startAs, Type: table.FieldTypeF64},
		table.Field{Name: n.endAs, Type: table.FieldTypeF64},
	)
	return out, nil
}

// stackOutputCollisionErr reports an output column name already taken
// by an upstream column.
func stackOutputCollisionErr(name, field string) error {
	return prismerrors.New(
		"PRISM_PLAN_STACK_OUTPUT_COLLISION",
		fmt.Sprintf("Stack output column %q already exists upstream; choose a different `as` pair.", name),
		map[string]any{"Column": name, "Field": field},
	)
}

// fieldNameList joins a schema's field names for diagnostics.
func fieldNameList(s *table.Schema) string {
	names := make([]string, len(s.Fields))
	for i := range s.Fields {
		names[i] = s.Fields[i].Name
	}
	return strings.Join(names, ", ")
}

// Execute implements plan.Node via the injected backend.
func (n *StackNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("StackNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *StackNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node.
func (n *StackNode) Fingerprint() string {
	return fingerprintFor("StackNode",
		string(n.input),
		n.field,
		"by:"+strings.Join(n.groupby, ","),
		"seg:"+strings.Join(n.stackBy, ","),
		"offset:"+n.offset,
		"as:"+n.startAs+","+n.endAs,
	)
}

// Field exposes the accumulated column name.
func (n *StackNode) Field() string { return n.field }

// Groupby exposes the stack-delimiting fields.
func (n *StackNode) Groupby() []string { return n.groupby }

// StackBy exposes the segment-ordering fields.
func (n *StackNode) StackBy() []string { return n.stackBy }

// Offset exposes the accumulation mode ("zero" or "normalize").
func (n *StackNode) Offset() string { return n.offset }

// StartAs / EndAs expose the output column names.
func (n *StackNode) StartAs() string { return n.startAs }
func (n *StackNode) EndAs() string   { return n.endAs }

// Kind implements plan.Labeled.
func (n *StackNode) Kind() string { return "StackNode" }

// Summary implements plan.Labeled — "stack: y | by: x | offset: zero".
func (n *StackNode) Summary() string {
	out := "stack: " + n.field
	if len(n.groupby) > 0 {
		out += " | by: " + strings.Join(n.groupby, ",")
	}
	return out + " | offset: " + n.offset
}
