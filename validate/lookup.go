package validate

// SchemaLookup resolves dataset name → minimal field metadata. Semantic
// rules that need to know whether a field exists or what type it is go
// through this interface so the validator stays decoupled from any data
// backend.
//
// StaticLookup (lookup_static.go) serves field metadata for inline
// datasets (`data.values` / `data.fields`) and test fixtures. A
// source-bound dataset carries no inline schema, so its lookup reports a
// miss and field-existence rules skip it — validate reads the spec plus
// an optional schema and never opens a data source.
type SchemaLookup interface {
	// Schema returns the schema for the named dataset and reports whether
	// it was found.
	Schema(dataset string) (*SchemaShim, bool)
}

// SchemaShim is the minimal field-metadata shape used by the semantic
// rules. It carries just enough to satisfy rules 001 (field exists),
// 002 (agg/type compat), and 007 (scale/type compat).
//
// The shape is intentionally minimal and stable so rule code does not
// change as the schema source evolves.
type SchemaShim struct {
	// Name is the dataset's logical name.
	Name string
	// Fields lists the field name → measure type ("nominal" |
	// "ordinal" | "quantitative" | "temporal") in declaration order.
	Fields []FieldShim
}

// FieldShim is one field in a SchemaShim.
type FieldShim struct {
	// Name is the field name as referenced by encodings / transforms.
	Name string
	// Type is the measure type bucket (nominal/ordinal/quantitative/temporal).
	Type string
}

// Field returns the FieldShim for name and whether it was found.
func (s *SchemaShim) Field(name string) (FieldShim, bool) {
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
func (s *SchemaShim) FieldNames() []string {
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
// an EmptyLookup — the mode used when no dataset schema is bound.
type EmptyLookup struct{}

// Schema implements SchemaLookup.
func (EmptyLookup) Schema(string) (*SchemaShim, bool) { return nil, false }
