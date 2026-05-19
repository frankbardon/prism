package passes

import (
	"github.com/frankbardon/pulse/encoding"
)

// schemaColSetFromEncoding returns the field-name set of an
// *encoding.Schema. Defined here so the FilterPushdown / ProjectionPruning
// passes can read column names without exporting Pulse-specific types
// in their files.
func schemaColSetFromEncoding(s any) map[string]struct{} {
	out := map[string]struct{}{}
	sch, ok := s.(*encoding.Schema)
	if !ok || sch == nil {
		return out
	}
	for i := range sch.Fields {
		out[sch.Fields[i].Name] = struct{}{}
	}
	return out
}

// schemaFromAny returns the *encoding.Schema iff s is the right type;
// nil otherwise. Helper for code paths (ProjectionPruning) that need
// the declaration order of fields, not just the name set.
func schemaFromAny(s any) *encoding.Schema {
	if sch, ok := s.(*encoding.Schema); ok {
		return sch
	}
	return nil
}
