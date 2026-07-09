package resolve

import (
	"context"
	"fmt"
	"io"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/vfs"
	"github.com/frankbardon/prism/table"
)

// DefaultResolver is the production Resolver. Since the Pulse loader was
// removed in epic E4 it no longer opens `.pulse` files; it serves data
// only through the inline path (InlineResolver), delegating to an
// optional caller-supplied DataResolver. `cohort:<id>` indirection is
// resolved through the Registry before the DataResolver is consulted.
//
// Construction:
//
//	r := resolve.New(nil)                  // EmptyRegistry, no DataResolver
//	r := resolve.New(reg)                  // custom Registry
//	r := resolve.NewWithData(reg, data)    // + inline DataResolver
type DefaultResolver struct {
	registry Registry
	data     DataResolver
}

// New constructs a DefaultResolver with no inline DataResolver wired.
// Passing a nil registry yields an EmptyRegistry — every cohort:<id>
// ref then returns PRISM_RESOLVE_004. Without a DataResolver every ref
// resolves to PRISM_RESOLVE_REF_UNRESOLVED.
func New(reg Registry) *DefaultResolver {
	if reg == nil {
		reg = EmptyRegistry{}
	}
	return &DefaultResolver{registry: reg}
}

// NewWithData constructs a DefaultResolver whose ResolveInline delegates
// to the supplied DataResolver. Callers that serve inline rows (the
// browser setDataResolver bridge, a server-side dataset registry) use
// this to satisfy source refs without a Pulse loader.
func NewWithData(reg Registry, data DataResolver) *DefaultResolver {
	r := New(reg)
	r.data = data
	return r
}

const maxRegistryDepth = 4

// Resolve implements Resolver. The Pulse byte-loader is gone, so there
// are no source bytes to stream: every ref reports
// PRISM_RESOLVE_REF_UNRESOLVED. Callers wanting rows use ResolveInline.
func (r *DefaultResolver) Resolve(ref string, _ vfs.Fs) (io.ReadCloser, *table.Schema, error) {
	return nil, nil, unresolvedRef(ref)
}

// ResolveInline implements InlineResolver. It canonicalises cohort:<id>
// indirection through the Registry, then delegates to the configured
// DataResolver. With no DataResolver wired — or when the DataResolver
// reports the ref unresolved — it returns PRISM_RESOLVE_REF_UNRESOLVED.
func (r *DefaultResolver) ResolveInline(ctx context.Context, ref string) (*Dataset, error) {
	canon, err := r.canonical(ref, 0)
	if err != nil {
		return nil, err
	}
	if r.data == nil {
		return nil, unresolvedRef(canon)
	}
	ds, err := r.data.ResolveData(ctx, canon)
	if err != nil {
		if _, ok := err.(ErrDataRefUnresolved); ok {
			return nil, unresolvedRef(canon)
		}
		return nil, err
	}
	if ds == nil {
		return nil, unresolvedRef(canon)
	}
	return ds, nil
}

// canonical dispatches the ref forms, resolving cohort:<id> indirection
// through the Registry (bounded by maxRegistryDepth) into the backing
// name the DataResolver understands. Empty and gs:// refs are rejected;
// everything else is returned verbatim.
func (r *DefaultResolver) canonical(ref string, depth int) (string, error) {
	if depth > maxRegistryDepth {
		return "", prismerrors.New(
			"PRISM_RESOLVE_005",
			fmt.Sprintf("Reference %q recursed more than %d times via cohort:<id>; suspecting a cycle.", ref, maxRegistryDepth),
			map[string]any{"Ref": ref},
		)
	}
	switch {
	case ref == "":
		return "", prismerrors.New(
			"PRISM_RESOLVE_005",
			`Reference "" does not match any known form (name, cohort:id).`,
			map[string]any{"Ref": ""},
		)
	case strings.HasPrefix(ref, "gs://"):
		return "", prismerrors.New(
			"PRISM_RESOLVE_GCS_UNAVAILABLE",
			fmt.Sprintf("gs:// references are not implemented in v1 (ref: %s).", ref),
			map[string]any{"Ref": ref},
		)
	case strings.HasPrefix(ref, "cohort:"):
		id := strings.TrimPrefix(ref, "cohort:")
		if id == "" {
			return "", prismerrors.New(
				"PRISM_RESOLVE_005",
				`Reference "cohort:" is missing an id.`,
				map[string]any{"Ref": ref},
			)
		}
		resolved, ok := r.registry.Lookup(id)
		if !ok {
			return "", prismerrors.New(
				"PRISM_RESOLVE_004",
				fmt.Sprintf("Cohort id %q is not registered in the active resolver registry.", id),
				map[string]any{"Id": id},
			)
		}
		return r.canonical(resolved, depth+1)
	default:
		return ref, nil
	}
}

// unresolvedRef builds the PRISM_RESOLVE_REF_UNRESOLVED envelope for a
// ref that no inline DataResolver could satisfy.
func unresolvedRef(ref string) error {
	return prismerrors.New(
		"PRISM_RESOLVE_REF_UNRESOLVED",
		fmt.Sprintf("Data ref %q was not resolved by the active DataResolver.", ref),
		map[string]any{"Ref": ref},
	)
}
