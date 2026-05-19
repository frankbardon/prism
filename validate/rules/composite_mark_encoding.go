package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// CompositeMarkEncoding implements PRISM_SPEC_013: composite marks
// (histogram, heatmap, boxplot, violin) require a specific shape of
// encoding channels. Pie / donut are covered by PRISM_SPEC_008
// (PieDonutEncoding); this rule covers the cartesian-axis composites.
//
// Rules per mark (D059–D062):
//
//   - histogram: requires a single quantitative x channel.
//   - heatmap:   requires both x and y to be bound.
//   - boxplot:   requires exactly one categorical axis (x or y) and
//     the other axis quantitative.
//   - violin:    same as boxplot.
type CompositeMarkEncoding struct{}

// Code returns PRISM_SPEC_013.
func (CompositeMarkEncoding) Code() string { return "PRISM_SPEC_013" }

// Check inspects the spec's mark + encoding against the per-mark
// shape requirements. Emits one PRISM_SPEC_013 per violation.
func (CompositeMarkEncoding) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil {
		return nil
	}
	mark := s.Mark.TypeName()
	switch mark {
	case "histogram", "heatmap", "boxplot", "violin":
	default:
		return nil
	}

	enc := s.Encoding
	var out []*errors.AppError
	emit := func(reason string) {
		out = append(out, errors.New("PRISM_SPEC_013",
			fmt.Sprintf("Composite mark %q cannot expand: %s.", mark, reason),
			map[string]any{
				"Mark":   mark,
				"Reason": reason,
			},
		))
	}

	if enc == nil {
		emit(fmt.Sprintf("mark %q has no encoding block", mark))
		return out
	}

	switch mark {
	case "histogram":
		if enc.X == nil || enc.X.Field == "" {
			emit("histogram needs a quantitative x channel")
		} else if enc.X.Type != "" && enc.X.Type != "quantitative" {
			emit(fmt.Sprintf("histogram x channel must be quantitative (got %q)", enc.X.Type))
		}

	case "heatmap":
		if enc.X == nil || enc.X.Field == "" {
			emit("heatmap needs both x and y channels (x missing)")
		}
		if enc.Y == nil || enc.Y.Field == "" {
			emit("heatmap needs both x and y channels (y missing)")
		}

	case "boxplot", "violin":
		xBound := enc.X != nil && enc.X.Field != ""
		yBound := enc.Y != nil && enc.Y.Field != ""
		if !xBound || !yBound {
			emit(fmt.Sprintf("%s needs one category axis and one quantitative axis", mark))
			break
		}
		xCat := isCategoricalType(enc.X.Type)
		yCat := isCategoricalType(enc.Y.Type)
		if xCat == yCat {
			// Both categorical or both quantitative — invalid.
			emit(fmt.Sprintf("%s needs exactly one category axis (x.type=%q, y.type=%q)", mark, enc.X.Type, enc.Y.Type))
		}
	}
	return out
}

// isCategoricalType returns true for nominal / ordinal channel types.
// Empty type strings default to false (treated as quantitative for
// this rule's purposes — the field-existence rule handles missing
// type elsewhere).
func isCategoricalType(t string) bool {
	switch t {
	case "nominal", "ordinal":
		return true
	}
	return false
}
