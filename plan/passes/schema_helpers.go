package passes

import "github.com/frankbardon/prism/table"

// schemaColSet returns the field-name set of a native table schema.
// Defined here so the FilterPushdown / ProjectionPruning passes can read
// column names without duplicating the loop in their files.
func schemaColSet(s *table.Schema) map[string]struct{} {
	out := map[string]struct{}{}
	if s == nil {
		return out
	}
	for i := range s.Fields {
		out[s.Fields[i].Name] = struct{}{}
	}
	return out
}
