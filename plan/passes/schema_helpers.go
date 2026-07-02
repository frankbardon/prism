package passes

import (
	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/table"
)

// schemaColSetFromEncoding returns the field-name set of a schema.
// Defined here so the FilterPushdown / ProjectionPruning passes can read
// column names without exporting schema-specific types in their files.
//
// It accepts either the native *table.Schema (returned by Node.Schema,
// post-E1-S2) or the Pulse *encoding.Schema (still returned by
// SourceNode.OutputSchema until the leaf loader is removed in E4).
func schemaColSetFromEncoding(s any) map[string]struct{} {
	out := map[string]struct{}{}
	switch sch := s.(type) {
	case *table.Schema:
		if sch == nil {
			return out
		}
		for i := range sch.Fields {
			out[sch.Fields[i].Name] = struct{}{}
		}
	case *encoding.Schema:
		if sch == nil {
			return out
		}
		for i := range sch.Fields {
			out[sch.Fields[i].Name] = struct{}{}
		}
	}
	return out
}

// schemaFromAny returns the *encoding.Schema iff s is the right type;
// nil otherwise. Helper for code paths (ProjectionPruning) that need
// the declaration order of fields, not just the name set — those read
// the schema straight from SourceNode.OutputSchema, which is still
// Pulse-typed until E4.
func schemaFromAny(s any) *encoding.Schema {
	if sch, ok := s.(*encoding.Schema); ok {
		return sch
	}
	return nil
}
