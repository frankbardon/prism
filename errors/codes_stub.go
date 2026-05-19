package errors

// CodeMetadata describes one Prism error code: its message template,
// fixup templates, and any cross-references.
type CodeMetadata struct {
	// Code is the PRISM_* identifier.
	Code string
	// Message is the user-facing template (Go text/template syntax).
	Message string
	// Fixups is the ordered list of fixup templates (Go text/template).
	Fixups []string
	// FixupNotApplicable marks codes that legitimately have no fixups.
	FixupNotApplicable bool
	// SeeAlso lists related codes or doc references.
	SeeAlso []string
}

// Codes is the canonical Prism error code catalog. T01.19 populates the
// full map; the variable lives here so the apperror.go lookup compiles
// before that task lands.
var Codes = map[string]CodeMetadata{}

// formatFixups returns the registered fixup strings as-is in this
// pre-template scaffold. T01.19 rewrites it to expand templates against
// the supplied context.
func formatFixups(templates []string, _ map[string]any) []string {
	out := make([]string, len(templates))
	copy(out, templates)
	return out
}
