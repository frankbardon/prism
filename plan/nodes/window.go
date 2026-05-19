package nodes

import (
	"context"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// WindowOp is one windowed calculation: op (rank, dense_rank, lag,
// lead, sum, mean, ...), source field, output name, optional
// parameter.
type WindowOp struct {
	Op    string
	Field string
	As    string
	Param *float64
}

// String returns a stable text form for fingerprints.
func (w WindowOp) String() string {
	p := ""
	if w.Param != nil {
		p = fmt.Sprintf("/%g", *w.Param)
	}
	return w.Op + "(" + w.Field + ")->" + w.As + p
}

// SortKey is a (field, order) pair the window operator consumes.
type SortKey struct {
	Field string
	Order string // "asc"|"desc"; empty defaults to "asc"
}

// String returns a stable text form for fingerprints.
func (s SortKey) String() string {
	if s.Order == "" {
		return s.Field + ":asc"
	}
	return s.Field + ":" + s.Order
}

// WindowNode applies windowed aggregates / ranks over an input. P03 stub.
type WindowNode struct {
	id          plan.NodeID
	input       plan.NodeID
	ops         []WindowOp
	partitionby []string
	sort        []SortKey
	frame       []any
}

// NewWindow constructs a WindowNode. All slices are copied.
func NewWindow(id, input plan.NodeID, ops []WindowOp, partitionby []string, sort []SortKey, frame []any) *WindowNode {
	op := make([]WindowOp, len(ops))
	copy(op, ops)
	pb := make([]string, len(partitionby))
	copy(pb, partitionby)
	sk := make([]SortKey, len(sort))
	copy(sk, sort)
	fr := make([]any, len(frame))
	copy(fr, frame)
	return &WindowNode{id: id, input: input, ops: op, partitionby: pb, sort: sk, frame: fr}
}

// ID implements plan.Node.
func (n *WindowNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *WindowNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is input + one F64 field
// per WindowOp (ranks come back as float to keep downstream arithmetic
// uniform; real impl can re-type integer ranks if profiling motivates).
func (n *WindowNode) Schema(in []*encoding.Schema) (*encoding.Schema, error) {
	s, err := requireSingleInput("WindowNode", in)
	if err != nil {
		return nil, err
	}
	out := cloneSchema(s)
	for _, op := range n.ops {
		if op.As == "" {
			return nil, fmt.Errorf("WindowNode: op %s missing 'as' name", op.Op)
		}
		out.Fields = append(out.Fields, encoding.Field{Name: op.As, Type: encoding.FieldTypeF64})
	}
	return out, nil
}

// Execute implements plan.Node. P03 stub.
func (n *WindowNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, notImplementedErr("WindowNode")
}

// Fingerprint implements plan.Node.
func (n *WindowNode) Fingerprint() string {
	parts := []string{string(n.input)}
	for _, op := range n.ops {
		parts = append(parts, op.String())
	}
	parts = append(parts, "pb:"+strings.Join(n.partitionby, ","))
	sortStrs := make([]string, len(n.sort))
	for i, sk := range n.sort {
		sortStrs[i] = sk.String()
	}
	parts = append(parts, "sort:"+strings.Join(sortStrs, ","))
	return fingerprintFor("WindowNode", parts...)
}

// Ops exposes the window operations for renderers + tests.
func (n *WindowNode) Ops() []WindowOp { return n.ops }

// Kind implements plan.Labeled.
func (n *WindowNode) Kind() string { return "WindowNode" }

// Summary implements plan.Labeled.
func (n *WindowNode) Summary() string {
	opStrs := make([]string, len(n.ops))
	for i, o := range n.ops {
		opStrs[i] = o.String()
	}
	return "ops: " + strings.Join(opStrs, ",")
}
