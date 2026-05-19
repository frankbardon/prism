// Package nodes holds the DAG node implementations. P02 ships SourceNode
// — the leaf that reads a .pulse via Resolver and materialises a Table.
// Future phases (P03+) add Filter, Project, GroupAggregate, Join, etc.
package nodes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/cespare/xxhash/v2"
	"github.com/frankbardon/pulse/encoding"
	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/table"
)

// SourceNode is the leaf DAG node. It resolves a ref to a .pulse cohort
// (or shard), materialises every record into typed columns, and emits
// a *table.Table tagged with the content hash of the on-disk bytes.
//
// SourceNode satisfies the full plan.Node interface (P03 widened it
// to include Schema(in)). The Schema method delegates to
// OutputSchema, which is kept for backwards compatibility with the
// P02 callers (validate.PulseLookup) that already use it.
type SourceNode struct {
	id       plan.NodeID
	ref      string
	fs       afero.Fs
	resolver resolve.Resolver
}

// New constructs a SourceNode. The ref must use one of the four forms
// documented in resolve/resolver.go. fs is the filesystem the resolver
// honours at execute time (test code passes afero.NewMemMapFs()).
// resolver must be non-nil; pass resolve.New(nil) for the default
// EmptyRegistry-backed implementation.
func New(ref string, fs afero.Fs, r resolve.Resolver) *SourceNode {
	return &SourceNode{
		id:       plan.NodeID(deriveID(ref)),
		ref:      ref,
		fs:       fs,
		resolver: r,
	}
}

// ID implements plan.Node.
func (n *SourceNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node. SourceNode is a leaf.
func (n *SourceNode) Inputs() []plan.NodeID { return nil }

// Fingerprint implements plan.Node. Source fingerprint = sha256(ref).
// Combined with Table.Hash() at execute time by the cache key builder.
func (n *SourceNode) Fingerprint() string {
	h := sha256.Sum256([]byte(n.ref))
	return "src:" + hex.EncodeToString(h[:])
}

// Ref returns the user-supplied ref string. Useful in error messages
// and plan-visualisation tooling.
func (n *SourceNode) Ref() string { return n.ref }

// OutputSchema resolves the ref so the schema is discoverable without
// materialising records. The ReadCloser returned by Resolve is closed
// immediately. Kept as a public method for backwards compatibility with
// validate.PulseLookup; new callers should prefer Schema(nil).
func (n *SourceNode) OutputSchema() (*encoding.Schema, error) {
	rc, schema, err := n.resolver.Resolve(n.ref, n.fs)
	if err != nil {
		return nil, err
	}
	if rc != nil {
		_ = rc.Close()
	}
	return schema, nil
}

// Schema implements plan.Node. Source nodes ignore the `in` slice (they
// have no upstream); the output schema is whatever the resolver reports
// for the underlying .pulse cohort.
func (n *SourceNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	return n.OutputSchema()
}

// Execute implements plan.Node. Reads the resolved payload bytes,
// computes their xxhash64, then decodes every record into typed
// column slices and constructs the Table. Row count is gated by
// limits.TableMaxRows() — once the counter would exceed the cap,
// Execute returns PRISM_RESOLVE_007 immediately (no further decode).
func (n *SourceNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if n.resolver == nil {
		return nil, fmt.Errorf("source: resolver is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rc, schema, err := n.resolver.Resolve(n.ref, n.fs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	// Materialise the entire payload to compute the content hash and to
	// have a seekable byte stream for header+schema skipping.
	payload, err := io.ReadAll(rc)
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to read payload for %s: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	contentHash := fmt.Sprintf("xxh64:%016x", xxhash.Sum64(payload))

	// Position the reader at the first record byte. The payload always
	// starts with HeaderSize bytes (magic + version) followed by the
	// encoded schema; we re-parse the schema here only to advance the
	// reader cursor — the authoritative *encoding.Schema is the one
	// returned by Resolve.
	br := bytes.NewReader(payload)
	if err := encoding.ReadHeader(br); err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse header invalid for %s: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	if _, err := encoding.ReadSchema(br); err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse schema unreadable for %s: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}

	tbl, err := materialise(schema, br, contentHash, n.ref)
	if err != nil {
		return nil, err
	}
	return tbl, nil
}

// materialise drains the RecordReader into per-column slices, enforcing
// the row cap inline. Returns the constructed Table on success.
func materialise(schema *encoding.Schema, r io.Reader, hash, ref string) (*table.Table, error) {
	cap, _ := limits.TableMaxRows()
	rr := encoding.NewRecordReader(r, schema)

	cols := newColumnBuilders(schema)

	values := make(map[string]float64, len(schema.Fields))
	nulls := make(map[string]bool, len(schema.Fields))

	rowCount := 0
	for {
		if err := rr.ReadRecord(values, nulls); err != nil {
			if err == io.EOF {
				break
			}
			return nil, prismerrors.Wrap(
				"PRISM_RESOLVE_006",
				fmt.Sprintf("Pulse decode failed for %s at row %d: %v.", ref, rowCount, err),
				map[string]any{"Ref": ref, "Reason": err.Error()},
				err,
			)
		}
		if rowCount >= cap {
			return nil, prismerrors.New(
				"PRISM_RESOLVE_007",
				fmt.Sprintf("Materialisation refused: more than %d rows would exceed PRISM_TABLE_MAX_ROWS=%d.", rowCount, cap),
				map[string]any{"Actual": rowCount + 1, "Limit": cap},
			)
		}
		appendRow(cols, schema, values)
		rowCount++
	}

	return table.NewTable(schema, finaliseColumns(cols, schema), rowCount, hash)
}

// columnBuilder is a per-field accumulator chosen by Kind. Storage is
// the same as table.Column impls; we hold a pointer to the backing
// slice so appendRow can grow it in place without re-allocating the
// map entry on every iteration.
type columnBuilder struct {
	kind   table.Kind
	ints   *[]int64
	floats *[]float64
	strs   *[]string
	bools  *[]bool
	dates  *[]int64
}

func newColumnBuilders(schema *encoding.Schema) map[string]*columnBuilder {
	out := make(map[string]*columnBuilder, len(schema.Fields))
	for i := range schema.Fields {
		f := &schema.Fields[i]
		kind := table.KindFromPulseFieldType(f.Type)
		cb := &columnBuilder{kind: kind}
		switch kind {
		case table.KindInt:
			s := make([]int64, 0)
			cb.ints = &s
		case table.KindFloat:
			s := make([]float64, 0)
			cb.floats = &s
		case table.KindString:
			s := make([]string, 0)
			cb.strs = &s
		case table.KindBool:
			s := make([]bool, 0)
			cb.bools = &s
		case table.KindDate:
			s := make([]int64, 0)
			cb.dates = &s
		}
		out[f.Name] = cb
	}
	return out
}

func appendRow(cols map[string]*columnBuilder, schema *encoding.Schema, values map[string]float64) {
	for i := range schema.Fields {
		f := &schema.Fields[i]
		cb := cols[f.Name]
		v := values[f.Name]
		switch cb.kind {
		case table.KindInt:
			*cb.ints = append(*cb.ints, int64(v))
		case table.KindFloat:
			*cb.floats = append(*cb.floats, v)
		case table.KindString:
			// Categorical: v is the dictionary id; resolve through the
			// field's dictionary. Missing dictionary falls back to the
			// numeric form so we never panic on malformed schemas.
			if f.Dictionary != nil {
				*cb.strs = append(*cb.strs, f.Dictionary.Resolve(uint32(v)))
			} else {
				*cb.strs = append(*cb.strs, fmt.Sprintf("%d", uint32(v)))
			}
		case table.KindBool:
			*cb.bools = append(*cb.bools, v != 0)
		case table.KindDate:
			*cb.dates = append(*cb.dates, int64(v))
		}
	}
}

func finaliseColumns(cols map[string]*columnBuilder, schema *encoding.Schema) map[string]table.Column {
	out := make(map[string]table.Column, len(cols))
	for i := range schema.Fields {
		name := schema.Fields[i].Name
		cb := cols[name]
		switch cb.kind {
		case table.KindInt:
			out[name] = table.IntColumn(*cb.ints)
		case table.KindFloat:
			out[name] = table.FloatColumn(*cb.floats)
		case table.KindString:
			out[name] = table.StringColumn(*cb.strs)
		case table.KindBool:
			out[name] = table.BoolColumn(*cb.bools)
		case table.KindDate:
			out[name] = table.DateColumn(*cb.dates)
		}
	}
	return out
}

// deriveID picks a stable, readable NodeID from a ref. We hash the ref
// to keep the id length bounded but prefix with "source:" for clarity
// in plan dumps.
func deriveID(ref string) string {
	h := sha256.Sum256([]byte(ref))
	return "source:" + hex.EncodeToString(h[:6])
}
