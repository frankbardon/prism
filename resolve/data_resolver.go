package resolve

import (
	"context"

	"github.com/frankbardon/prism/spec"
)

// DataResolver maps an opaque runtime reference to an inline dataset.
// Used to support the `data: {ref: "<name>"}` spec variant: the spec
// describes *what to draw*; the resolver provides *the data to draw
// it with*. Different environments resolve the same ref differently:
//
//   - A browser environment looks up an injected dataset from the
//     page runtime via prism.setDataResolver.
//   - A server environment reads a pre-positioned file or queries a
//     trusted backend.
//   - A test environment returns canned rows from a fixture map.
//
// Implementations should be safe for concurrent use — the executor
// may call Resolve from multiple goroutines when several layers
// reference the same ref.
type DataResolver interface {
	ResolveData(ctx context.Context, ref string) (*Dataset, error)
}

// Dataset is the in-memory dataset shape returned by a DataResolver.
// It carries row values + optional field schema, matching the inline
// `data: {values: [...]}` variant of the spec.
type Dataset struct {
	Values []map[string]any
	Fields []spec.FieldSpec
}

// DataResolverFunc adapts a function to the DataResolver interface.
type DataResolverFunc func(ctx context.Context, ref string) (*Dataset, error)

// ResolveData implements DataResolver.
func (f DataResolverFunc) ResolveData(ctx context.Context, ref string) (*Dataset, error) {
	return f(ctx, ref)
}

// MapDataResolver is a static map-backed resolver. Useful in tests
// and for embedding small canned datasets without writing a custom
// resolver. The zero value is a usable, empty resolver — Resolve
// returns ErrDataRefUnresolved for every ref.
type MapDataResolver map[string]*Dataset

// ResolveData implements DataResolver.
func (m MapDataResolver) ResolveData(_ context.Context, ref string) (*Dataset, error) {
	if d, ok := m[ref]; ok && d != nil {
		return d, nil
	}
	return nil, ErrDataRefUnresolved{Ref: ref}
}

// ErrDataRefUnresolved signals that the caller's DataResolver could
// not satisfy the given ref. Plan / build surfaces it as
// PRISM_RESOLVE_REF_UNRESOLVED.
type ErrDataRefUnresolved struct {
	Ref string
}

// Error implements error.
func (e ErrDataRefUnresolved) Error() string {
	return "data resolver: unresolved ref " + e.Ref
}

// ChainDataResolvers walks each resolver in order, returning the
// first non-unresolved result. Errors that are *not*
// ErrDataRefUnresolved short-circuit immediately. Nil resolvers in
// the chain are skipped silently.
func ChainDataResolvers(resolvers ...DataResolver) DataResolver {
	return DataResolverFunc(func(ctx context.Context, ref string) (*Dataset, error) {
		var lastUnresolved error
		for _, r := range resolvers {
			if r == nil {
				continue
			}
			ds, err := r.ResolveData(ctx, ref)
			if err == nil {
				return ds, nil
			}
			if _, ok := err.(ErrDataRefUnresolved); ok {
				lastUnresolved = err
				continue
			}
			return nil, err
		}
		if lastUnresolved != nil {
			return nil, lastUnresolved
		}
		return nil, ErrDataRefUnresolved{Ref: ref}
	})
}
