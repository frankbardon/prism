package validate

// SchemaLookup resolves dataset name → minimal field metadata. Semantic
// rules that need to know whether a field exists or what type it is go
// through this interface so the validator stays decoupled from Pulse.
//
// P01 ships StaticLookup (lookup_static.go) backed by a hand-built field
// table for tests. P02 will land a Pulse-backed implementation that walks
// real .pulse sources via the resolver.
type SchemaLookup interface {
	// Schema returns the schema for the named dataset and reports whether
	// it was found.
	Schema(dataset string) (*PulseSchemaShim, bool)
}

// PulseSchemaShim is the minimal field-metadata shape used by P01
// semantic rules. It carries just enough to satisfy rules 001 (field
// exists), 002 (agg/type compat), and 007 (scale/type compat).
//
// TODO(P02): replace with real Pulse schema type once the resolver
// surfaces it. Keep the field shape stable so rule code does not change.
type PulseSchemaShim struct {
	// Name is the dataset's logical name.
	Name string
	// Fields lists the field name → measure type ("nominal" |
	// "ordinal" | "quantitative" | "temporal") in declaration order.
	Fields []FieldShim
}

// FieldShim is one field in a PulseSchemaShim.
type FieldShim struct {
	// Name is the field name as referenced by encodings / transforms.
	Name string
	// Type is the measure type bucket (nominal/ordinal/quantitative/temporal).
	Type string
}

// Field returns the FieldShim for name and whether it was found.
func (s *PulseSchemaShim) Field(name string) (FieldShim, bool) {
	if s == nil {
		return FieldShim{}, false
	}
	for _, f := range s.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return FieldShim{}, false
}

// FieldNames returns field names in declaration order.
func (s *PulseSchemaShim) FieldNames() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		out = append(out, f.Name)
	}
	return out
}

// EmptyLookup is a SchemaLookup that finds nothing. Semantic rules that
// gate on schema presence (e.g. PRISM_SPEC_001) silently no-op when given
// an EmptyLookup, matching the P01 "no real Pulse source bound" mode.
type EmptyLookup struct{}

// Schema implements SchemaLookup.
func (EmptyLookup) Schema(string) (*PulseSchemaShim, bool) { return nil, false }
