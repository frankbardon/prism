package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// PieDonutEncoding implements PRISM_SPEC_008: pie and donut marks
// (and the underlying arc mark when used in their composite form) must
// encode value via theta and category via color, never x/y. PRISM_SPEC_003
// catches "channel not valid for mark" generically; this rule provides a
// more specific message tailored to the pie/donut workflow.
type PieDonutEncoding struct{}

// Code returns PRISM_SPEC_008.
func (PieDonutEncoding) Code() string { return "PRISM_SPEC_008" }

// Check fires when:
//   - mark is pie or donut and the spec uses x or y, OR
//   - mark is pie or donut and no theta encoding is present.
func (PieDonutEncoding) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil {
		return nil
	}
	switch s.Mark.TypeName() {
	case "pie", "donut":
	default:
		return nil
	}
	var out []*errors.AppError
	if s.Encoding != nil {
		if s.Encoding.X != nil || s.Encoding.Y != nil {
			out = append(out, errors.New("PRISM_SPEC_008",
				fmt.Sprintf("Mark %q uses x/y encodings; expected theta + color.", s.Mark.TypeName()),
				map[string]any{"Mark": s.Mark.TypeName()},
			))
		}
		if s.Encoding.Theta == nil {
			out = append(out, errors.New("PRISM_SPEC_008",
				fmt.Sprintf("Mark %q is missing theta encoding.", s.Mark.TypeName()),
				map[string]any{"Mark": s.Mark.TypeName()},
			))
		}
	} else {
		out = append(out, errors.New("PRISM_SPEC_008",
			fmt.Sprintf("Mark %q requires theta + color encoding but no encoding block is present.", s.Mark.TypeName()),
			map[string]any{"Mark": s.Mark.TypeName()},
		))
	}
	return out
}
