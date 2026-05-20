package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SelectionIntervalChannel implements PRISM_SPEC_020: an interval
// selection's `encodings` list must contain only position channels
// (x | y | x2 | y2 | theta). Color / size / shape intervals are not
// supported in v1 — those use cases route through point selections.
type SelectionIntervalChannel struct{}

// Code returns PRISM_SPEC_020.
func (SelectionIntervalChannel) Code() string { return "PRISM_SPEC_020" }

var allowedIntervalChannels = map[string]bool{
	"x": true, "y": true, "x2": true, "y2": true, "theta": true,
}

// Check walks every interval selection's encodings list and reports
// any non-position entry.
func (SelectionIntervalChannel) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || len(s.Selection) == 0 {
		return nil
	}
	var out []*errors.AppError
	for selName, sel := range s.Selection {
		if sel.Interval == nil {
			continue
		}
		for _, c := range sel.Interval.Encodings {
			if allowedIntervalChannels[c] {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_020",
				fmt.Sprintf("Interval selection %q uses non-position channel %q (intervals brush over position axes only).",
					selName, c),
				map[string]any{
					"Selection": selName,
					"Channel":   c,
				},
			))
		}
	}
	return out
}
