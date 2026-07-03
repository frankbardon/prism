// Package nodes holds the DAG node implementations. SourceNode is the
// leaf bound to `data.source`; since the Pulse loader was removed in
// epic E4 it materialises rows through the resolver's inline path
// (InlineResolver) rather than decoding `.pulse` bytes.
package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/table"
)

// SourceNode is the leaf DAG node bound to `data.source`. It resolves a
// ref to its inline rows via the resolver's InlineResolver seam and
// materialises them into a native *table.Table with table.FromInline.
// A resolver that cannot serve the ref (no DataResolver wired) yields
// PRISM_RESOLVE_REF_UNRESOLVED.
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

// FS returns the afero filesystem this source was constructed with.
// Optimizer passes that rewrite a source-rooted subtree (e.g.
// PulseChainFusionPass) reuse this fs so the replacement node still
// honours an in-memory test environment.
func (n *SourceNode) FS() afero.Fs { return n.fs }

// Kind implements plan.Labeled.
func (n *SourceNode) Kind() string { return "SourceNode" }

// Summary implements plan.Labeled — the ref string the user wrote.
func (n *SourceNode) Summary() string { return n.ref }

// OutputSchema returns the native output schema of the source. Kept as
// a public method for the optimizer passes (ProjectionPruning) that read
// a source's fields; it delegates to Schema(nil). Since E4 it returns a
// native *table.Schema — the Pulse shim is gone.
func (n *SourceNode) OutputSchema() (*table.Schema, error) {
	return n.Schema(nil)
}

// Schema implements plan.Node. Source nodes ignore the `in` slice (they
// have no upstream); the output schema is inferred from the inline rows
// the resolver serves for this ref, in the native Prism shape.
func (n *SourceNode) Schema(_ []*table.Schema) (*table.Schema, error) {
	ds, err := n.rows(context.Background())
	if err != nil {
		return nil, err
	}
	_, schema, err := table.FromInline(n.ref, ds.Values, ds.Fields)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

// RowCount returns the number of inline rows the resolver serves for
// this ref. Used by the SampleInjection optimizer pass to decide
// whether to inject a SampleNode below a Source whose row count exceeds
// PRISM_RENDER_MAX_MARKS. The pass skips the source on any error, so an
// unresolvable ref simply disables sampling for it.
func (n *SourceNode) RowCount(ctx context.Context) (uint64, error) {
	ds, err := n.rows(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(len(ds.Values)), nil
}

// rows fetches the inline dataset for this source's ref via the
// resolver's InlineResolver seam. A resolver that does not implement
// InlineResolver, or one that cannot satisfy the ref, yields
// PRISM_RESOLVE_REF_UNRESOLVED.
func (n *SourceNode) rows(ctx context.Context) (*resolve.Dataset, error) {
	if n.resolver == nil {
		return nil, fmt.Errorf("source: resolver is nil")
	}
	ir, ok := n.resolver.(resolve.InlineResolver)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_REF_UNRESOLVED",
			fmt.Sprintf("Data ref %q was not resolved by the active DataResolver.", n.ref),
			map[string]any{"Ref": n.ref},
		)
	}
	ds, err := ir.ResolveInline(ctx, n.ref)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_REF_UNRESOLVED",
			fmt.Sprintf("Data ref %q resolved to a nil dataset.", n.ref),
			map[string]any{"Ref": n.ref},
		)
	}
	return ds, nil
}

// Execute implements plan.Node. Materialises the resolver's inline rows
// into a native Table via table.FromInline. Row count is gated by
// limits.TableMaxRows() — a larger dataset returns PRISM_RESOLVE_007.
func (n *SourceNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ds, err := n.rows(ctx)
	if err != nil {
		return nil, err
	}
	if cap, ok := limits.TableMaxRows(); ok && len(ds.Values) > cap {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_007",
			fmt.Sprintf("Materialisation refused: %d rows would exceed PRISM_TABLE_MAX_ROWS=%d.", len(ds.Values), cap),
			map[string]any{"Actual": len(ds.Values), "Limit": cap},
		)
	}
	tbl, _, err := table.FromInline(n.ref, ds.Values, ds.Fields)
	if err != nil {
		return nil, err
	}
	return tbl, nil
}

// columnBuilder is a per-field accumulator chosen by Kind. Storage is
// the same as table.Column impls; we hold a pointer to the backing
// slice so the join/union builders can grow it in place without
// re-allocating the map entry on every iteration. `nulls` tracks
// per-row null markers for nullable fields; nil otherwise so
// non-nullable columns stay plain slice-backed Column impls. The type
// and currentLen are shared by join_execute.go and union_execute.go.
type columnBuilder struct {
	kind     table.Kind
	nullable bool
	nulls    *table.NullBitmap
	ints     *[]int64
	floats   *[]float64
	strs     *[]string
	bools    *[]bool
	dates    *[]int64
}

func currentLen(cb *columnBuilder) int {
	switch cb.kind {
	case table.KindInt:
		return len(*cb.ints)
	case table.KindFloat:
		return len(*cb.floats)
	case table.KindString:
		return len(*cb.strs)
	case table.KindBool:
		return len(*cb.bools)
	case table.KindDate:
		return len(*cb.dates)
	}
	return 0
}

// deriveID picks a stable, readable NodeID from a ref. We hash the ref
// to keep the id length bounded but prefix with "source:" for clarity
// in plan dumps.
func deriveID(ref string) string {
	h := sha256.Sum256([]byte(ref))
	return "source:" + hex.EncodeToString(h[:6])
}
