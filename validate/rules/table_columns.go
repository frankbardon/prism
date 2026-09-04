package rules

import (
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// TableColumns implements PRISM_SPEC_040: a table mark declares its
// entire visual contract through encoding.columns[] rather than
// position channels (there is no x/y for a table), so at least one
// column definition is required. A missing or empty columns[] would
// render nothing.
type TableColumns struct{}

// Code returns PRISM_SPEC_040.
func (TableColumns) Code() string { return "PRISM_SPEC_040" }

// Check fires when mark is table and encoding.columns is absent or
// empty.
func (TableColumns) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil || s.Mark.TypeName() != "table" {
		return nil
	}
	if s.Encoding != nil && len(s.Encoding.Columns) > 0 {
		return nil
	}
	return []*errors.AppError{
		errors.New("PRISM_SPEC_040",
			`Mark "table" requires at least one column in encoding.columns[].`,
			map[string]any{"Mark": "table"},
		),
	}
}
