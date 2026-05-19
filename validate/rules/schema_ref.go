package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SchemaRef implements PRISM_SPEC_009: the spec's $schema value must
// reference a known Prism schema location. Accepted forms:
//   - The canonical URN urn:prism:schema:v1:spec.
//   - A relative or absolute path ending in spec.schema.json.
//   - A file:// URL ending in spec.schema.json.
type SchemaRef struct{}

// Code returns PRISM_SPEC_009.
func (SchemaRef) Code() string { return "PRISM_SPEC_009" }

// Check fires when s.Schema does not match any accepted form.
func (SchemaRef) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	if isAcceptedSchemaRef(s.Schema) {
		return nil
	}
	return []*errors.AppError{
		errors.New("PRISM_SPEC_009",
			fmt.Sprintf("$schema value %q does not reference a known Prism schema.", s.Schema),
			map[string]any{"Schema": s.Schema},
		),
	}
}

func isAcceptedSchemaRef(ref string) bool {
	switch {
	case ref == "urn:prism:schema:v1:spec":
		return true
	case strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "file://"):
		return strings.HasSuffix(ref, "spec.schema.json")
	default:
		return false
	}
}
