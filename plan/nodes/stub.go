// File stub.go centralises the helpers every P03 stub node uses:
//
//   - notImplementedErr(kind) — the PRISM_COMPILE_001 error returned by
//     every stubbed Execute body until P04 lands the real impls.
//   - cloneSchema / appendField / projectFields — pure helpers that
//     compute deterministic output schemas from input schemas plus op
//     parameters, so Schema(in) works in P03 without execution data.
//   - fingerprintFor(kind, parts...) — sha256-prefixed cache-key
//     component, deterministic across runs.
//
// Stub nodes (everything that is not SourceNode, InlineNode, or
// SinkNode) wire these helpers in their per-type files (filter.go,
// project.go, ...). The Execute body is always one line:
//
//	return nil, notImplementedErr("FilterNode")
//
// P04 swaps each stub's body for the real implementation; tests that
// currently assert PRISM_COMPILE_001 flip to assert correct output
// tables.
package nodes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
)

// notImplementedErr returns the canonical PRISM_COMPILE_001 AppError
// every stubbed Execute body emits. The Phase context is hard-coded to
// "P04" because every stub lands in that phase; if the rollout slips
// the message references the actual landing phase, not P04.
func notImplementedErr(nodeType string) error {
	return prismerrors.New(
		"PRISM_COMPILE_001",
		fmt.Sprintf("Node type %s is not implemented yet (lands in P04).", nodeType),
		map[string]any{"NodeType": nodeType, "Phase": "P04"},
	)
}

// fingerprintFor builds a deterministic per-node fingerprint by
// hashing (kind, parts...) and prefixing the hex digest with
// `<kind>:`. Identical (kind, parts) → identical fingerprint, which is
// the cache-key invariant CacheKey relies on.
func fingerprintFor(kind string, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(kind))
	h.Write([]byte{0})
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return strings.ToLower(kind) + ":" + hex.EncodeToString(h.Sum(nil)[:8])
}

// requireSingleInput asserts in has exactly one schema and returns it.
// Stubbed single-input nodes call this from their Schema body so the
// shape check lives in one place.
func requireSingleInput(kind string, in []*encoding.Schema) (*encoding.Schema, error) {
	if len(in) != 1 {
		return nil, fmt.Errorf("%s: expected 1 input schema, got %d", kind, len(in))
	}
	if in[0] == nil {
		return nil, fmt.Errorf("%s: input schema is nil", kind)
	}
	return in[0], nil
}

// requireTwoInputsErr is the multi-input analogue of requireSingleInput
// used by JoinNode and any other dyadic node. Returns nil on success.
func requireTwoInputsErr(kind string, in []*encoding.Schema) error {
	if len(in) != 2 {
		return fmt.Errorf("%s: expected 2 input schemas, got %d", kind, len(in))
	}
	if in[0] == nil || in[1] == nil {
		return fmt.Errorf("%s: nil input schema", kind)
	}
	return nil
}

// cloneSchema returns a shallow copy of s (same Field values, fresh
// outer slice). Stub Schema bodies that grow the field list call this
// to avoid mutating the input schema.
func cloneSchema(s *encoding.Schema) *encoding.Schema {
	out := &encoding.Schema{Fields: make([]encoding.Field, len(s.Fields))}
	copy(out.Fields, s.Fields)
	return out
}

// projectFields builds a new schema containing only the named fields
// from s, in the order requested. Missing fields raise PRISM_PLAN_003
// with the available field list so the diagnostic actually helps.
func projectFields(s *encoding.Schema, names []string) (*encoding.Schema, error) {
	if s == nil {
		return nil, fmt.Errorf("projectFields: input schema is nil")
	}
	idx := map[string]*encoding.Field{}
	available := make([]string, 0, len(s.Fields))
	for i := range s.Fields {
		idx[s.Fields[i].Name] = &s.Fields[i]
		available = append(available, s.Fields[i].Name)
	}
	out := &encoding.Schema{Fields: make([]encoding.Field, 0, len(names))}
	for _, n := range names {
		f, ok := idx[n]
		if !ok {
			return nil, prismerrors.New(
				"PRISM_PLAN_003",
				fmt.Sprintf("Field %q not in source schema (available: %s).", n, strings.Join(available, ", ")),
				map[string]any{"Dataset": n, "Available": strings.Join(available, ", ")},
			)
		}
		out.Fields = append(out.Fields, *f)
	}
	return out, nil
}

// appendField appends one field to a cloned schema. Used by Calculate,
// Window, Bin, and GroupAggregate to widen the output shape.
func appendField(s *encoding.Schema, name string, ft encoding.FieldType) *encoding.Schema {
	out := cloneSchema(s)
	out.Fields = append(out.Fields, encoding.Field{Name: name, Type: ft})
	return out
}

// aggregateOutputType maps a Prism aggregate op name to its result
// FieldType. Every cohort-analytics op (count, sum, mean, min, max,
// median, stdev, variance, wmean, ratio, lift, share) lands in
// FieldTypeF64 because the result is always a scalar measure. Domain
// extensions that ship a different result kind (e.g. `argmax` would
// return a categorical) get explicit cases when they land.
func aggregateOutputType(op string) encoding.FieldType {
	switch strings.ToLower(op) {
	case "count":
		// Count is conceptually integer but we keep the result type
		// uniform with the other aggregates for downstream simplicity —
		// scales and encodings already treat F64/integer alike.
		return encoding.FieldTypeF64
	default:
		return encoding.FieldTypeF64
	}
}

// joinedSchema unions two schemas, dropping duplicate occurrences of
// the join key fields (left wins). The output preserves left order
// then right order so test assertions remain readable.
func joinedSchema(left, right *encoding.Schema, on []string) *encoding.Schema {
	keys := map[string]struct{}{}
	for _, k := range on {
		keys[k] = struct{}{}
	}
	out := &encoding.Schema{}
	seen := map[string]struct{}{}
	for i := range left.Fields {
		f := left.Fields[i]
		out.Fields = append(out.Fields, f)
		seen[f.Name] = struct{}{}
	}
	for i := range right.Fields {
		f := right.Fields[i]
		// Skip join keys present on the left; they would duplicate.
		if _, isKey := keys[f.Name]; isKey {
			if _, dup := seen[f.Name]; dup {
				continue
			}
		}
		// Skip any other name collision (left wins).
		if _, dup := seen[f.Name]; dup {
			continue
		}
		out.Fields = append(out.Fields, f)
		seen[f.Name] = struct{}{}
	}
	return out
}
