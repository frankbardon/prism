package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// InlineNode is the leaf node fed by `data.values` (and optional
// `data.fields`) declarations. Unlike SourceNode (which reads .pulse
// bytes via Resolver), InlineNode delegates straight to
// table.FromInline at Execute time.
//
// Not a stub — table.FromInline shipped in P02; the executor needs
// real inline-data behaviour in P03 to make any inline-bound spec
// runnable end-to-end through the pipeline.
type InlineNode struct {
	id     plan.NodeID
	name   string
	values []map[string]any
	fields []spec.FieldSpec

	// cachedSchema is computed once and reused so Schema() does not
	// re-walk the values on every call.
	cachedSchema *encoding.Schema
}

// NewInline constructs an InlineNode from raw spec inputs.
func NewInline(id plan.NodeID, name string, values []map[string]any, fields []spec.FieldSpec) *InlineNode {
	cp := make([]map[string]any, len(values))
	for i, row := range values {
		row2 := make(map[string]any, len(row))
		for k, v := range row {
			row2[k] = v
		}
		cp[i] = row2
	}
	fcp := make([]spec.FieldSpec, len(fields))
	copy(fcp, fields)
	return &InlineNode{id: id, name: name, values: cp, fields: fcp}
}

// ID implements plan.Node.
func (n *InlineNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node. Inline is a leaf — same as Source.
func (n *InlineNode) Inputs() []plan.NodeID { return nil }

// Schema implements plan.Node. Computes via table.FromInline the first
// time it is called, then caches the result. Errors propagate.
func (n *InlineNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	if n.cachedSchema != nil {
		return n.cachedSchema, nil
	}
	_, s, err := table.FromInline(n.name, n.values, n.fields)
	if err != nil {
		return nil, err
	}
	n.cachedSchema = s
	return s, nil
}

// Execute implements plan.Node.
func (n *InlineNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	tbl, _, err := table.FromInline(n.name, n.values, n.fields)
	return tbl, err
}

// Fingerprint implements plan.Node. Hashes a canonical JSON encoding
// of (name, fields, values-with-sorted-keys) so two equivalent inline
// declarations produce the same fingerprint regardless of original
// map ordering.
func (n *InlineNode) Fingerprint() string {
	h := sha256.New()
	h.Write([]byte("inline\x00"))
	h.Write([]byte(n.name))
	h.Write([]byte{0})
	for _, f := range n.fields {
		h.Write([]byte(f.Name + ":" + f.Type + "\x00"))
	}
	for _, row := range n.values {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b, err := json.Marshal(row[k])
			if err != nil {
				b = []byte("null")
			}
			h.Write([]byte(k))
			h.Write([]byte{':'})
			h.Write(b)
			h.Write([]byte{0})
		}
		h.Write([]byte{'\n'})
	}
	return "inline:" + hex.EncodeToString(h.Sum(nil)[:8])
}

// Name exposes the dataset name (empty if unnamed).
func (n *InlineNode) Name() string { return n.name }

// Kind implements plan.Labeled.
func (n *InlineNode) Kind() string { return "InlineNode" }

// Summary implements plan.Labeled.
func (n *InlineNode) Summary() string {
	if n.name != "" {
		return fmt.Sprintf("%s, %d rows", n.name, len(n.values))
	}
	return fmt.Sprintf("%d rows", len(n.values))
}
