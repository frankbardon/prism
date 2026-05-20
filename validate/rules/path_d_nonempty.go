package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// PathDNonEmpty implements PRISM_SPEC_017: the path mark's `d`
// attribute (carried as MarkDef.Path on the spec) must be a
// non-empty SVG path string. Empty / whitespace-only strings are
// rejected at validate time.
type PathDNonEmpty struct{}

// Code returns PRISM_SPEC_017.
func (PathDNonEmpty) Code() string { return "PRISM_SPEC_017" }

// Check fires when the spec's mark is "path" and the d-string is
// empty or whitespace-only.
func (PathDNonEmpty) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil {
		return nil
	}
	if s.Mark.TypeName() != "path" {
		return nil
	}
	d := ""
	if s.Mark.Def != nil {
		d = s.Mark.Def.Path
	}
	if strings.TrimSpace(d) != "" {
		return nil
	}
	got := "<empty>"
	if d != "" {
		got = fmt.Sprintf("%q", d)
	}
	return []*errors.AppError{
		errors.New("PRISM_SPEC_017",
			fmt.Sprintf("Mark \"path\" requires a non-empty d field (got %s).", got),
			map[string]any{"Got": got},
		),
	}
}
