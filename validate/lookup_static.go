package validate

// StaticLookup is a SchemaLookup backed by an in-memory map. Used for
// inline datasets (`data.values`, `data.fields`) and as a test fixture
// holder. Its sibling DatasetLookup (lookup_dataset.go) tracks
// source-bound dataset names; CompositeLookup mixes both when a spec uses
// both inline and source bindings.
type StaticLookup struct {
	Schemas map[string]*SchemaShim
}

// NewStaticLookup constructs an empty StaticLookup.
func NewStaticLookup() *StaticLookup {
	return &StaticLookup{Schemas: map[string]*SchemaShim{}}
}

// Register adds or replaces the entry for the given dataset name.
func (l *StaticLookup) Register(name string, schema *SchemaShim) {
	if l.Schemas == nil {
		l.Schemas = map[string]*SchemaShim{}
	}
	if schema != nil {
		schema.Name = name
	}
	l.Schemas[name] = schema
}

// Schema implements SchemaLookup.
func (l *StaticLookup) Schema(name string) (*SchemaShim, bool) {
	if l == nil {
		return nil, false
	}
	s, ok := l.Schemas[name]
	return s, ok
}

// Names implements the Namer interface (see lookup_dataset.go) by
// returning the registered dataset names in arbitrary order.
func (l *StaticLookup) Names() []string {
	if l == nil {
		return nil
	}
	out := make([]string, 0, len(l.Schemas))
	for k := range l.Schemas {
		out = append(out, k)
	}
	return out
}
