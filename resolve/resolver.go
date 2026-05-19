// Package resolve maps user-facing dataset refs to a (ReadCloser,
// Schema) pair. Four ref forms are supported:
//
//   - cohort.pulse                 local OsFs path
//   - archive.pulse#shard.pulse    anchor inside a Pulse shard archive
//   - gs://bucket/path.pulse       GCS-backed file (PUNT in v1: returns
//                                  PRISM_RESOLVE_GCS_UNAVAILABLE)
//   - cohort:<id>                  registry lookup; the resolved string
//                                  is recursively resolved (one level)
//
// Every read goes through an afero.Fs so tests can use NewMemMapFs.
// The Resolver does not cache; it leaves caching to the DAG executor
// via Table.Hash() + per-node fingerprint.
//
// GCS is intentionally not wired in P02. Pulse 0.8.4 does not ship a
// generic GCS afero.Fs; adding a real GCS SDK in this phase violates
// the dep-parity rule and inflates v1 scope. See D027.
package resolve

import (
	"io"

	"github.com/frankbardon/pulse/encoding"
	"github.com/spf13/afero"
)

// Resolver resolves a ref to a streaming reader plus the cohort's
// schema. The returned ReadCloser, when non-nil, owns its underlying
// file handle; callers must Close it. Schema is always non-nil on
// success.
//
// Concrete implementations:
//   - DefaultResolver (resolve/default.go): the production path.
//   - tests may swap in a fake that returns canned bytes + schema.
type Resolver interface {
	Resolve(ref string, fs afero.Fs) (io.ReadCloser, *encoding.Schema, error)
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
