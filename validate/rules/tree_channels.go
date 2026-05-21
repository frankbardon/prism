package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// TreeChannels implements PRISM_SPEC_028: tree, dendrogram, and
// network marks require source + target channel bindings (per the
// flat-table input form). Missing channels raise a single error
// listing every missing name.
type TreeChannels struct{}

// Code returns PRISM_SPEC_028.
func (TreeChannels) Code() string { return "PRISM_SPEC_028" }

// Check fires when mark is tree / dendrogram / network and either of
// source / target is missing or has an empty Field.
func (TreeChannels) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil {
		return nil
	}
	switch s.Mark.TypeName() {
	case "tree", "dendrogram", "network":
	default:
		return nil
	}
	enc := s.Encoding
	var missing []string
	if enc == nil || enc.Source == nil || enc.Source.Field == "" {
		missing = append(missing, "source")
	}
	if enc == nil || enc.Target == nil || enc.Target.Field == "" {
		missing = append(missing, "target")
	}
	if len(missing) == 0 {
		return nil
	}
	joined := strings.Join(missing, ", ")
	return []*errors.AppError{
		errors.New("PRISM_SPEC_028",
			fmt.Sprintf("Mark %s requires source + target channels (missing: %s).", s.Mark.TypeName(), joined),
			map[string]any{"Mark": s.Mark.TypeName(), "Missing": joined},
		),
	}
}
