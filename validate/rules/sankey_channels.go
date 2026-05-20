package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SankeyChannels implements PRISM_SPEC_018: sankey marks require
// source, target, and value channel bindings (per D064 flat-table
// input form). Missing channels raise a single error listing every
// missing name.
type SankeyChannels struct{}

// Code returns PRISM_SPEC_018.
func (SankeyChannels) Code() string { return "PRISM_SPEC_018" }

// Check fires when mark is sankey and any of source / target /
// value is missing or has an empty Field.
func (SankeyChannels) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil {
		return nil
	}
	if s.Mark.TypeName() != "sankey" {
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
	if enc == nil || enc.Value == nil || enc.Value.Field == "" {
		missing = append(missing, "value")
	}
	if len(missing) == 0 {
		return nil
	}
	joined := strings.Join(missing, ", ")
	return []*errors.AppError{
		errors.New("PRISM_SPEC_018",
			fmt.Sprintf("Sankey mark requires source, target, and value channels (missing: %s).", joined),
			map[string]any{"Missing": joined},
		),
	}
}
