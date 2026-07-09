package validate

import (
	"github.com/frankbardon/prism/internal/vfs"
	"github.com/frankbardon/prism/resolve"
)

// DatasetLookup enumerates the dataset names bound to a `data.source`
// (or `cohort:<id>`) reference. It once resolved each ref's `.pulse`
// header to a field schema, but the Pulse loader was removed in epic E4
// and Prism never reads `.pulse` bytes. Field schema for inline data
// (`data.values` / `data.fields`) is supplied by StaticLookup; a
// source-bound dataset has no inline schema, so DatasetLookup.Schema is
// best-effort — it reports a miss and semantic rules skip field-existence
// checks for that dataset rather than firing false positives (validate
// reads the spec plus an *optional* schema).
//
// Register still records the names so the dataset-reference rule can
// treat externally-bound datasets as declared.
type DatasetLookup struct {
	bindings map[string]string
}

// NewDatasetLookup constructs a DatasetLookup. The resolver and fs
// parameters are retained for call-site compatibility but are no longer
// consulted (no `.pulse` is read); bindings start empty.
func NewDatasetLookup(_ resolve.Resolver, _ vfs.Fs) *DatasetLookup {
	return &DatasetLookup{bindings: map[string]string{}}
}

// Register records a dataset name bound to a source ref. A no-op when
// name or ref is empty.
func (l *DatasetLookup) Register(name, ref string) {
	if name == "" || ref == "" {
		return
	}
	l.bindings[name] = ref
}

// Names returns every registered dataset name in arbitrary order.
// Used by the dataset-ref semantic rule to enumerate externally
// declared datasets.
func (l *DatasetLookup) Names() []string {
	if l == nil {
		return nil
	}
	out := make([]string, 0, len(l.bindings))
	for k := range l.bindings {
		out = append(out, k)
	}
	return out
}

// Schema implements SchemaLookup. Source-bound datasets carry no inline
// schema (Prism no longer reads `.pulse`), so lookups are best-effort:
// Schema always reports a miss and rules skip field checks for the
// dataset. Inline `data.values` / `data.fields` are served by
// StaticLookup.
func (l *DatasetLookup) Schema(string) (*SchemaShim, bool) {
	return nil, false
}

// CompositeLookup tries lookups in order and returns the first hit.
// Used by the CLI when a spec mixes inline datasets (StaticLookup) with
// source-bound datasets (DatasetLookup) — both lookups share one
// SchemaLookup surface so semantic rules need no awareness.
type CompositeLookup struct {
	lookups []SchemaLookup
}

// NewCompositeLookup constructs a CompositeLookup over the given
// lookups in priority order. nil lookups are skipped.
func NewCompositeLookup(lookups ...SchemaLookup) *CompositeLookup {
	out := make([]SchemaLookup, 0, len(lookups))
	for _, l := range lookups {
		if l != nil {
			out = append(out, l)
		}
	}
	return &CompositeLookup{lookups: out}
}

// Schema implements SchemaLookup.
func (c *CompositeLookup) Schema(name string) (*SchemaShim, bool) {
	if c == nil {
		return nil, false
	}
	for _, l := range c.lookups {
		if shim, ok := l.Schema(name); ok {
			return shim, true
		}
	}
	return nil, false
}

// Names implements the Namer interface (see Namer below) by unioning
// every constituent lookup's Names. Lookups that do not implement
// Names contribute nothing.
func (c *CompositeLookup) Names() []string {
	if c == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, l := range c.lookups {
		if n, ok := l.(Namer); ok {
			for _, name := range n.Names() {
				if !seen[name] {
					seen[name] = true
					out = append(out, name)
				}
			}
		}
	}
	return out
}

// Namer is the optional capability for SchemaLookup impls that can
// enumerate their registered dataset names. The dataset-reference
// rule (PRISM_SPEC_005) uses this to know which external datasets
// to treat as declared, on top of the in-spec `datasets:` block.
type Namer interface {
	Names() []string
}
