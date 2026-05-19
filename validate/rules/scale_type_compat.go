package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ScaleTypeCompat implements PRISM_SPEC_007: an explicit scale type on
// an encoding channel must be compatible with the field's measure type.
// log/pow/sqrt/linear belong on quantitative; time on temporal; band /
// point / ordinal on nominal/ordinal.
type ScaleTypeCompat struct{}

// Code returns PRISM_SPEC_007.
func (ScaleTypeCompat) Code() string { return "PRISM_SPEC_007" }

// Check inspects every channel with an inline scale.type and validates it
// against the channel's declared "type" (nominal/ordinal/quantitative/
// temporal). If the channel has no type but the dataset is known, the
// rule falls back to the source field's type. No-ops when neither source
// is available.
func (ScaleTypeCompat) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Encoding == nil {
		return nil
	}
	_, schema, known := datasetForSpec(s, schemas)

	var out []*errors.AppError
	for ch, cs := range channelScales(s.Encoding) {
		if cs.ScaleType == "" {
			continue
		}
		measure := cs.Type
		if measure == "" && known {
			if f, ok := schema.Field(cs.Field); ok {
				measure = f.Type
			}
		}
		if measure == "" {
			continue
		}
		if scaleCompatible(cs.ScaleType, measure) {
			continue
		}
		out = append(out, errors.New("PRISM_SPEC_007",
			fmt.Sprintf("Scale type %q on channel %q is not compatible with field type %q.", cs.ScaleType, ch, measure),
			map[string]any{
				"ScaleType":  cs.ScaleType,
				"Channel":    ch,
				"Field":      cs.Field,
				"FieldType":  measure,
				"Compatible": joinSorted(scalesForType(measure)),
				"ScaleFor":   typeForScale(cs.ScaleType),
			},
		))
	}
	return out
}

// channelScale carries the scale info from one channel.
type channelScale struct {
	ScaleType string
	Field     string
	Type      string
}

func channelScales(enc *spec.Encoding) map[string]channelScale {
	out := map[string]channelScale{}
	if enc == nil {
		return out
	}
	add := func(ch string, scale *spec.Scale, field, typ string) {
		if scale == nil {
			return
		}
		out[ch] = channelScale{ScaleType: scale.Type, Field: field, Type: typ}
	}
	if enc.X != nil {
		add("x", enc.X.Scale, enc.X.Field, enc.X.Type)
	}
	if enc.Y != nil {
		add("y", enc.Y.Scale, enc.Y.Field, enc.Y.Type)
	}
	if enc.X2 != nil {
		add("x2", enc.X2.Scale, enc.X2.Field, enc.X2.Type)
	}
	if enc.Y2 != nil {
		add("y2", enc.Y2.Scale, enc.Y2.Field, enc.Y2.Type)
	}
	if enc.Theta != nil {
		add("theta", enc.Theta.Scale, enc.Theta.Field, enc.Theta.Type)
	}
	if enc.Radius != nil {
		add("radius", enc.Radius.Scale, enc.Radius.Field, enc.Radius.Type)
	}
	if enc.Color != nil {
		add("color", enc.Color.Scale, enc.Color.Field, enc.Color.Type)
	}
	if enc.Fill != nil {
		add("fill", enc.Fill.Scale, enc.Fill.Field, enc.Fill.Type)
	}
	if enc.Stroke != nil {
		add("stroke", enc.Stroke.Scale, enc.Stroke.Field, enc.Stroke.Type)
	}
	if enc.Opacity != nil {
		add("opacity", enc.Opacity.Scale, enc.Opacity.Field, enc.Opacity.Type)
	}
	if enc.Size != nil {
		add("size", enc.Size.Scale, enc.Size.Field, enc.Size.Type)
	}
	if enc.Shape != nil {
		add("shape", enc.Shape.Scale, enc.Shape.Field, enc.Shape.Type)
	}
	return out
}

// scaleCompatible reports whether scaleType works with measureType.
func scaleCompatible(scaleType, measureType string) bool {
	for _, allowed := range scalesForType(measureType) {
		if allowed == scaleType {
			return true
		}
	}
	return false
}

func scalesForType(measureType string) []string {
	switch strings.ToLower(measureType) {
	case "quantitative":
		return []string{"linear", "log", "pow", "sqrt"}
	case "temporal":
		return []string{"time", "linear"}
	case "ordinal":
		return []string{"ordinal", "band", "point"}
	case "nominal":
		return []string{"ordinal", "band", "point"}
	default:
		return []string{"linear", "log", "pow", "sqrt", "time", "band", "point", "ordinal"}
	}
}

// typeForScale returns the measure type the scale is meant for (used in
// fixup text to suggest type changes).
func typeForScale(scaleType string) string {
	switch scaleType {
	case "linear", "log", "pow", "sqrt":
		return "quantitative"
	case "time":
		return "temporal"
	case "band", "point", "ordinal":
		return "nominal or ordinal"
	default:
		return "quantitative"
	}
}

func init() {
	// Keep ordering deterministic for fixup text / tests.
	sort.Strings(scalesForType("quantitative"))
}
