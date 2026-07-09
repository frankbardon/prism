package nodes

import (
	"context"
	"encoding/json"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// FilterNode applies a structured predicate (spec.Predicate) to its
// input table (E2-S1 — replaces the old Pulse-expression string).
// Execute routes through the injected backend; falls back to
// PRISM_COMPILE_001 when no backend is wired (preserves P03 stub
// behaviour for callers that haven't migrated). See D033.
type FilterNode struct {
	id      plan.NodeID
	input   plan.NodeID
	pred    spec.Predicate
	backend plan.Backend
}

// NewFilter constructs a FilterNode with a stable id derived from
// (input, predicate) so two equivalent filters share a fingerprint.
func NewFilter(id, input plan.NodeID, pred spec.Predicate) *FilterNode {
	return &FilterNode{id: id, input: input, pred: pred}
}

// ID implements plan.Node.
func (n *FilterNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node.
func (n *FilterNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Filter does not change the schema.
func (n *FilterNode) Schema(in []*table.Schema) (*table.Schema, error) {
	return requireSingleInput("FilterNode", in)
}

// Execute implements plan.Node. Routes through the injected backend
// when one is wired; returns PRISM_COMPILE_001 otherwise.
func (n *FilterNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("FilterNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute. The
// builder calls this after construction so node constructors keep
// their P03 signatures stable. See D033.
func (n *FilterNode) SetBackend(b plan.Backend) { n.backend = b }

// Fingerprint implements plan.Node. Derived from the canonical JSON of
// the predicate so equivalent predicates fingerprint identically.
func (n *FilterNode) Fingerprint() string {
	return fingerprintFor("FilterNode", string(n.input), n.predKey())
}

// Predicate exposes the structured predicate for the executor,
// optimizer passes, and tests.
func (n *FilterNode) Predicate() spec.Predicate { return n.pred }

// ReferencedFields returns the columns the predicate reads. Used by the
// filter-pushdown pass to decide which join side a filter belongs to.
func (n *FilterNode) ReferencedFields() []string { return n.pred.ReferencedFields() }

// predKey renders the predicate as canonical JSON for fingerprinting +
// summaries. Marshal errors collapse to an empty string (a filter that
// cannot marshal cannot execute either).
func (n *FilterNode) predKey() string {
	b, err := json.Marshal(n.pred)
	if err != nil {
		return ""
	}
	return string(b)
}

// Kind implements plan.Labeled.
func (n *FilterNode) Kind() string { return "FilterNode" }

// Summary implements plan.Labeled.
func (n *FilterNode) Summary() string { return "predicate: " + n.predKey() }
