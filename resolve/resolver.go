// Package resolve maps user-facing dataset refs to inline rows.
//
// Prism no longer reads `.pulse` bytes: the Pulse loader was removed in
// epic E4. Data now enters exclusively through:
//
//   - inline `data.values` / `data.fields` (materialised by
//     table.FromInline, wired via InlineNode);
//   - named `datasets` blocks (same inline path);
//   - a caller-supplied DataResolver (the `data: {ref}` variant),
//     surfaced here through the InlineResolver seam.
//
// A `cohort:<id>` ref is still resolved through the Registry to its
// backing name before the DataResolver is consulted (one level, bounded
// by maxRegistryDepth). `gs://` refs remain unavailable.
package resolve

import (
	"context"
	"io"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/table"
)

// Resolver is the legacy byte-seam. Before E4 it streamed `.pulse`
// bytes plus the cohort schema; with the Pulse loader removed there are
// no source bytes to stream, so DefaultResolver.Resolve now always
// reports PRISM_RESOLVE_REF_UNRESOLVED. The interface is retained so
// existing call sites (prism.go, CLI, wasm) keep compiling until E4-S1b
// migrates them; new code should use InlineResolver.
type Resolver interface {
	Resolve(ref string, fs afero.Fs) (io.ReadCloser, *table.Schema, error)
}

// InlineResolver is the Pulse-free rows seam. A Resolver that can serve
// a ref as inline rows (DefaultResolver, backed by a DataResolver)
// implements it; SourceNode uses it to materialise a native
// table.Table via table.FromInline without touching Pulse. Refs that
// cannot be served return PRISM_RESOLVE_REF_UNRESOLVED.
type InlineResolver interface {
	ResolveInline(ctx context.Context, ref string) (*Dataset, error)
}

// Registry maps a cohort id (`cohort:<id>`) to its backing ref (path,
// anchor, or gs:// URL). The default in-process implementation is
// MapRegistry; the dashboard/orchestrator wires its own at runtime.
type Registry interface {
	Lookup(id string) (string, bool)
}

// EmptyRegistry rejects every lookup. Used when no registry is wired.
type EmptyRegistry struct{}

// Lookup implements Registry.
func (EmptyRegistry) Lookup(string) (string, bool) { return "", false }

// MapRegistry is the default in-memory Registry. The zero value is an
// empty map; callers may also construct it via MapRegistry{"id": ref}.
type MapRegistry map[string]string

// Lookup implements Registry.
func (m MapRegistry) Lookup(id string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[id]
	return v, ok
}
