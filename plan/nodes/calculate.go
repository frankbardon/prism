package nodes

import (
	"context"
	"encoding/json"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// CalculateNode appends one computed column derived from a structured
// CalcExpr (E2-S2 — replaces the old Pulse-expression string). The
// output column type is inferred from the expression shape + input
// schema (numeric expressions land in F64, string expressions in a
// categorical column).
type CalculateNode struct {
	id      plan.NodeID
	input   plan.NodeID
	expr    spec.CalcExpr
	as      string
	backend plan.Backend
}

// NewCalculate constructs a CalculateNode.
func NewCalculate(id, input plan.NodeID, expr spec.CalcExpr, as string) *CalculateNode {
	return &CalculateNode{id: id, input: input, expr: expr, as: as}
}

// ID implements plan.Node.
func (n *CalculateNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *CalculateNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Output schema is the input plus one field
// named n.as, whose type is inferred from the expression.
func (n *CalculateNode) Schema(in []*table.Schema) (*table.Schema, error) {
	s, err := requireSingleInput("CalculateNode", in)
	if err != nil {
		return nil, err
	}
	return appendField(s, n.as, CalcResultType(n.expr, s)), nil
}

// Execute implements plan.Node via the injected backend.
func (n *CalculateNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("CalculateNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *CalculateNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node. Derived from the canonical JSON of
// the expression so equivalent expressions fingerprint identically.
func (n *CalculateNode) Fingerprint() string {
	return fingerprintFor("CalculateNode", string(n.input), n.exprKey(), n.as)
}

// Calc exposes the structured expression for the executor, optimizer
// passes, and tests.
func (n *CalculateNode) Calc() spec.CalcExpr { return n.expr }

// As exposes the output column name for renderers + tests.
func (n *CalculateNode) As() string { return n.as }

// ReferencedFields returns the columns the expression reads.
func (n *CalculateNode) ReferencedFields() []string { return n.expr.ReferencedFields() }

// exprKey renders the expression as canonical JSON for fingerprinting +
// summaries. Marshal errors collapse to an empty string (an expression
// that cannot marshal cannot execute either).
func (n *CalculateNode) exprKey() string {
	b, err := json.Marshal(n.expr)
	if err != nil {
		return ""
	}
	return string(b)
}

// Kind implements plan.Labeled.
func (n *CalculateNode) Kind() string { return "CalculateNode" }

// Summary implements plan.Labeled.
func (n *CalculateNode) Summary() string { return n.as + " = " + n.exprKey() }

// CalcResultType infers the storage FieldType of a CalcExpr's output
// column against an input schema. Numeric expressions land in F64;
// string expressions (concat, or a coalesce/case/field that resolves to
// a categorical column) land in a categorical column. The default when a
// form is ambiguous is F64 (the conservative numeric bucket).
func CalcResultType(e spec.CalcExpr, in *table.Schema) table.FieldType {
	if calcKindIsString(&e, in) {
		return table.FieldTypeCategoricalU8
	}
	return table.FieldTypeF64
}

// calcKindIsString reports whether the expression evaluates to a string
// column (as opposed to a numeric one).
func calcKindIsString(e *spec.CalcExpr, in *table.Schema) bool {
	switch e.Form() {
	case "field":
		if f := in.Field(e.Field); f != nil {
			return table.KindFromFieldType(f.Type) == table.KindString
		}
		return false
	case "literal":
		_, isStr := e.Literal.(string)
		return isStr
	case "op":
		return false
	case "fn":
		// coalesce, min, max inherit their first argument's kind; abs,
		// round, floor, ceil, neg are numeric.
		switch e.Fn {
		case spec.CalcFnCoalesce, spec.CalcFnMin, spec.CalcFnMax:
			if len(e.Args) > 0 {
				return calcKindIsString(&e.Args[0], in)
			}
		}
		return false
	case "concat":
		return true
	case "case":
		if len(e.Case) > 0 {
			return calcKindIsString(&e.Case[0].Then, in)
		}
		if e.Else != nil {
			return calcKindIsString(e.Else, in)
		}
	}
	return false
}
