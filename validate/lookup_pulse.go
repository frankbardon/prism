package validate

import (
	"github.com/frankbardon/pulse/encoding"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/resolve"
)

// PulseLookup is a SchemaLookup backed by a real Resolver. It is the
// production replacement for StaticLookup once a spec binds a dataset
// to a `.pulse` source (path, archive#shard, or cohort:<id>).
//
// Construction:
//
//	pl := validate.NewPulseLookup(resolve.New(nil), afero.NewOsFs())
//	pl.Register("brand_scores", "testdata/cohorts/tiny.pulse")
//
// Lookups resolve through the Resolver and fold the returned
// *encoding.Schema into the minimal *PulseSchemaShim that semantic
// rules consume. Results are cached by dataset name for the lifetime
// of the PulseLookup — Validate is a one-shot caller, so the cache is
// scoped to a single validation pass.
type PulseLookup struct {
	resolver resolve.Resolver
	fs       afero.Fs
	bindings map[string]string
	cache    map[string]*PulseSchemaShim
}

// NewPulseLookup constructs a PulseLookup. resolver and fs must be
// non-nil; bindings start empty (use Register before validating).
func NewPulseLookup(r resolve.Resolver, fs afero.Fs) *PulseLookup {
	return &PulseLookup{
		resolver: r,
		fs:       fs,
		bindings: map[string]string{},
		cache:    map[string]*PulseSchemaShim{},
	}
}

// Register binds a dataset name to a ref the Resolver knows how to
// open. Re-registration replaces the binding and invalidates the cache
// entry. A no-op when name or ref is empty.
func (l *PulseLookup) Register(name, ref string) {
	if name == "" || ref == "" {
		return
	}
	l.bindings[name] = ref
	delete(l.cache, name)
}

// Schema implements SchemaLookup.
//
// Resolves the dataset's ref via the Resolver, folds the Pulse schema
// into a *PulseSchemaShim, and returns it. Returns (nil, false) when
// the name is not registered or the Resolver returns an error — the
// validator interprets a missing schema as "no checks possible" rather
// than firing false-positive rule errors.
func (l *PulseLookup) Schema(name string) (*PulseSchemaShim, bool) {
	if l == nil {
		return nil, false
	}
	if cached, ok := l.cache[name]; ok {
		return cached, cached != nil
	}
	ref, ok := l.bindings[name]
	if !ok {
		return nil, false
	}
	rc, schema, err := l.resolver.Resolve(ref, l.fs)
	if err != nil {
		// Cache the miss so a single Validate pass does not retry the
		// same failed Resolve once per rule.
		l.cache[name] = nil
		return nil, false
	}
	if rc != nil {
		_ = rc.Close()
	}
	shim := pulseSchemaToShim(name, schema)
	l.cache[name] = shim
	return shim, true
}

// pulseSchemaToShim folds an *encoding.Schema into the minimal
// PulseSchemaShim P01 rules consume. Type bucketing:
//
//	IsNumeric()    -> "quantitative"
//	IsCategorical() -> "nominal"
//	FieldTypeDate   -> "temporal"
//	bool (packed/nullable) -> "nominal"
//	anything else   -> "nominal" (conservative fallback)
func pulseSchemaToShim(name string, schema *encoding.Schema) *PulseSchemaShim {
	shim := &PulseSchemaShim{Name: name}
	if schema == nil {
		return shim
	}
	for _, f := range schema.Fields {
		shim.Fields = append(shim.Fields, FieldShim{
			Name: f.Name,
			Type: measureTypeFor(f.Type),
		})
	}
	return shim
}

// measureTypeFor returns the Prism measure-type bucket for a Pulse
// FieldType. Exposed here (lower-case) for internal reuse; the public
// surface stays through PulseLookup.
func measureTypeFor(ft encoding.FieldType) string {
	switch {
	case ft == encoding.FieldTypeDate:
		return "temporal"
	case ft.IsNumeric():
		return "quantitative"
	case ft.IsCategorical():
		return "nominal"
	case ft == encoding.FieldTypePackedBool, ft == encoding.FieldTypeNullableBool:
		return "nominal"
	default:
		return "nominal"
	}
}

// CompositeLookup tries lookups in order and returns the first hit.
// Used by the CLI when a spec mixes inline datasets (StaticLookup) with
// real `.pulse` sources (PulseLookup) — both lookups share one
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
func (c *CompositeLookup) Schema(name string) (*PulseSchemaShim, bool) {
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
