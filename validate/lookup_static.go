package validate

// StaticLookup is a SchemaLookup backed by an in-memory map. It is the
// P01 testing/CLI default; tests build one with the dataset/field
// fixtures they expect; the CLI uses it to wire inline datasets without
// touching Pulse.
//
// TODO(P02): swap StaticLookup for a real Pulse-backed implementation
// driven by the resolver.
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
