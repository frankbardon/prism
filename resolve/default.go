package resolve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/encoding"
	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
)

// DefaultResolver is the production Resolver. It dispatches ref forms
// against the four known shapes and delegates to Pulse for the bulk
// of the work (single-file open, archive#shard anchor extraction).
//
// Construction:
//
//	r := resolve.New(nil)            // EmptyRegistry, no cohort:<id> lookups
//	r := resolve.New(reg)            // custom Registry
//
// The fs argument to Resolve is taken per-call so callers can swap
// between OsFs, MemMapFs, or BasePathFs without rebuilding the resolver.
type DefaultResolver struct {
	registry Registry
}

// New constructs a DefaultResolver. Passing a nil registry yields an
// EmptyRegistry — every cohort:<id> ref will then return
// PRISM_RESOLVE_004.
func New(reg Registry) *DefaultResolver {
	if reg == nil {
		reg = EmptyRegistry{}
	}
	return &DefaultResolver{registry: reg}
}

// Resolve implements Resolver.
func (r *DefaultResolver) Resolve(ref string, fs afero.Fs) (io.ReadCloser, *encoding.Schema, error) {
	return r.resolve(ref, fs, 0)
}

const maxRegistryDepth = 4

// resolve performs the ref dispatch with a depth counter. cohort:<id>
// indirection may chain (an id resolves to a path that resolves to
// another id), but we bound recursion to prevent infinite loops.
func (r *DefaultResolver) resolve(ref string, fs afero.Fs, depth int) (io.ReadCloser, *encoding.Schema, error) {
	if depth > maxRegistryDepth {
		return nil, nil, prismerrors.New(
			"PRISM_RESOLVE_005",
			fmt.Sprintf("Reference %q recursed more than %d times via cohort:<id>; suspecting a cycle.", ref, maxRegistryDepth),
			map[string]any{"Ref": ref},
		)
	}
	if ref == "" {
		return nil, nil, prismerrors.New(
			"PRISM_RESOLVE_005",
			`Reference "" does not match any known form (path, archive#shard, gs://, or cohort:id).`,
			map[string]any{"Ref": ""},
		)
	}
	switch {
	case strings.HasPrefix(ref, "gs://"):
		return nil, nil, prismerrors.New(
			"PRISM_RESOLVE_GCS_UNAVAILABLE",
			fmt.Sprintf("gs:// references are not implemented in v1 (ref: %s).", ref),
			map[string]any{"Ref": ref},
		)
	case strings.HasPrefix(ref, "cohort:"):
		id := strings.TrimPrefix(ref, "cohort:")
		if id == "" {
			return nil, nil, prismerrors.New(
				"PRISM_RESOLVE_005",
				`Reference "cohort:" is missing an id.`,
				map[string]any{"Ref": ref},
			)
		}
		resolved, ok := r.registry.Lookup(id)
		if !ok {
			return nil, nil, prismerrors.New(
				"PRISM_RESOLVE_004",
				fmt.Sprintf("Cohort id %q is not registered in the active resolver registry.", id),
				map[string]any{"Id": id},
			)
		}
		return r.resolve(resolved, fs, depth+1)
	default:
		// Local path or archive#shard anchor; both go through resolvePath.
		return r.resolvePath(ref, fs)
	}
}

// resolvePath handles both single-file `.pulse` and `archive.pulse#shard.pulse`.
func (r *DefaultResolver) resolvePath(ref string, fs afero.Fs) (io.ReadCloser, *encoding.Schema, error) {
	if fs == nil {
		fs = afero.NewOsFs()
	}

	archivePath, shardName, isAnchor := splitAnchor(ref)
	readPath := archivePath
	if !isAnchor {
		readPath = ref
	}

	exists, err := afero.Exists(fs, readPath)
	if err != nil {
		return nil, nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to open %s: stat error: %v.", ref, err),
			map[string]any{"Ref": ref, "Reason": err.Error()},
			err,
		)
	}
	if !exists {
		return nil, nil, prismerrors.New(
			"PRISM_RESOLVE_002",
			fmt.Sprintf("Local .pulse file %s not found on the configured filesystem.", readPath),
			map[string]any{"Path": readPath},
		)
	}

	// Use Pulse's facade for the heavy lifting. Construct a per-call
	// instance so the resolver can honour an arbitrary fs at call time
	// without holding a long-lived Pulse handle.
	p, err := pulse.New(pulse.Options{FS: fs})
	if err != nil {
		return nil, nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to open %s: %v.", ref, err),
			map[string]any{"Ref": ref, "Reason": err.Error()},
			err,
		)
	}

	cohort, err := p.Open(context.Background(), ref)
	if err != nil {
		return nil, nil, classifyPulseOpenError(ref, archivePath, shardName, isAnchor, err)
	}
	schema := cohort.Schema()

	// Build a ReadCloser over the underlying bytes. For an anchor we
	// extract the shard payload from the archive; for a single-file
	// .pulse we re-open the file. Downstream (P02 SourceNode + P04
	// compiler) consume the ReadCloser when they need record bytes.
	rc, err := openPayload(fs, readPath, shardName, isAnchor)
	if err != nil {
		return nil, schema, err
	}
	return rc, schema, nil
}

// openPayload returns an io.ReadCloser over the bytes of either the
// single-file cohort (when isAnchor is false) or the named shard inside
// the archive (when isAnchor is true).
func openPayload(fs afero.Fs, readPath, shardName string, isAnchor bool) (io.ReadCloser, error) {
	if !isAnchor {
		f, err := fs.Open(readPath)
		if err != nil {
			return nil, prismerrors.Wrap(
				"PRISM_RESOLVE_006",
				fmt.Sprintf("Pulse failed to open %s: %v.", readPath, err),
				map[string]any{"Ref": readPath, "Reason": err.Error()},
				err,
			)
		}
		return f, nil
	}

	// Anchor form: read the archive bytes, parse, extract the shard.
	data, err := afero.ReadFile(fs, readPath)
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to read archive %s: %v.", readPath, err),
			map[string]any{"Ref": readPath, "Reason": err.Error()},
			err,
		)
	}
	arc, err := encoding.OpenArchive(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to parse archive %s: %v.", readPath, err),
			map[string]any{"Ref": readPath, "Reason": err.Error()},
			err,
		)
	}
	rc, err := arc.Open(shardName)
	if err != nil {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_003",
			fmt.Sprintf("Shard %s not present in archive %s.", shardName, readPath),
			map[string]any{"Shard": shardName, "Archive": readPath},
		)
	}
	return rc, nil
}

// classifyPulseOpenError maps a *pulse.Open error to a PRISM_RESOLVE_*
// code. Unknown errors fall through as PRISM_RESOLVE_006 wrapping the
// original.
func classifyPulseOpenError(ref, archivePath, shardName string, isAnchor bool, err error) error {
	msg := err.Error()
	switch {
	case isAnchor && (strings.Contains(msg, "PULSE_SHARD_MISSING") || strings.Contains(msg, "shard ")):
		return prismerrors.New(
			"PRISM_RESOLVE_003",
			fmt.Sprintf("Shard %s not present in archive %s.", shardName, archivePath),
			map[string]any{"Shard": shardName, "Archive": archivePath},
		)
	case strings.Contains(msg, "no such file") || strings.Contains(msg, "does not exist"):
		path := ref
		if isAnchor {
			path = archivePath
		}
		return prismerrors.New(
			"PRISM_RESOLVE_002",
			fmt.Sprintf("Local .pulse file %s not found on the configured filesystem.", path),
			map[string]any{"Path": path},
		)
	default:
		return prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to open %s: %v.", ref, err),
			map[string]any{"Ref": ref, "Reason": err.Error()},
			err,
		)
	}
}

// splitAnchor splits an archive.pulse#shard.pulse ref into its two
// halves. Returns (archive, shard, true) when the ref carries a `#`;
// otherwise (ref, "", false). Mirrors Pulse's service.SplitAnchorPath
// surface — Pulse's function is not exported on the package root, so we
// reimplement the simple form here.
func splitAnchor(ref string) (string, string, bool) {
	i := strings.IndexByte(ref, '#')
	if i < 0 {
		return ref, "", false
	}
	return ref[:i], ref[i+1:], true
}
