package validate

// StaticLookup is a SchemaLookup backed by an in-memory map. Used for
// inline datasets (`data.values`, `data.fields`) and as a test fixture
// holder. The Pulse-backed sibling is PulseLookup (lookup_pulse.go);
// CompositeLookup mixes both when a spec uses both inline and source
// bindings.
type StaticLookup struct {
	Schemas map[string]*PulseSchemaShim
}

// NewStaticLookup constructs an empty StaticLookup.
func NewStaticLookup() *StaticLookup {
	return &StaticLookup{Schemas: map[string]*PulseSchemaShim{}}
}

// Register adds or replaces the entry for the given dataset name.
func (l *StaticLookup) Register(name string, schema *PulseSchemaShim) {
	if l.Schemas == nil {
		l.Schemas = map[string]*PulseSchemaShim{}
	}
	if schema != nil {
		schema.Name = name
	}
	l.Schemas[name] = schema
}

// Schema implements SchemaLookup.
func (l *StaticLookup) Schema(name string) (*PulseSchemaShim, bool) {
	if l == nil {
		return nil, false
	}
	s, ok := l.Schemas[name]
	return s, ok
}

// Names implements the Namer interface (see lookup_pulse.go) by
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
