package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ScaleDomain implements PRISM_SPEC_041: an explicit `scale.domain`
// must match the shape the scale family can consume.
//
//   - continuous families (linear / log / pow / sqrt) take exactly two
//     numeric bounds in ascending order;
//   - the time family takes exactly two bounds, each a number
//     (epoch ms) or a non-empty date string;
//   - discrete families (band / point / ordinal) take a non-empty list
//     of string categories.
//
// The family comes from `scale.type` when declared, otherwise from the
// channel's measure type. When neither is known the rule no-ops — the
// encoder raises the same code at resolve time.
type ScaleDomain struct{}

// Code returns PRISM_SPEC_041.
func (ScaleDomain) Code() string { return "PRISM_SPEC_041" }

// Check walks every channel carrying a scale block, on the root spec
// and on every composition child, and flags malformed domains.
func (ScaleDomain) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walkScaleDomainSpecs(s, func(sub *spec.Spec) {
		for _, b := range scaleDomainBindings(sub.Encoding) {
			out = append(out, checkScaleDomain(b)...)
		}
	})
	return out
}

// scaleDomainBinding pairs a channel name with its scale block and
// declared measure type.
type scaleDomainBinding struct {
	channel string
	measure string
	scale   *spec.Scale
}

// walkScaleDomainSpecs visits the spec and every composition child
// that can carry its own encoding.
func walkScaleDomainSpecs(s *spec.Spec, fn func(*spec.Spec)) {
	if s == nil {
		return
	}
	fn(s)
	for _, group := range [][]*spec.Spec{s.Layer, s.Concat, s.HConcat, s.VConcat} {
		for _, child := range group {
			walkScaleDomainSpecs(child, fn)
		}
	}
	walkScaleDomainSpecs(s.ChildSpec, fn)
}

// scaleDomainBindings collects every channel on an encoding that
// declares a scale block with a domain.
func scaleDomainBindings(enc *spec.Encoding) []scaleDomainBinding {
	if enc == nil {
		return nil
	}
	var out []scaleDomainBinding
	add := func(ch string, sc *spec.Scale, measure string) {
		if sc == nil || sc.Domain == nil {
			return
		}
		out = append(out, scaleDomainBinding{channel: ch, measure: measure, scale: sc})
	}
	if enc.X != nil {
		add("x", enc.X.Scale, enc.X.Type)
	}
	if enc.Y != nil {
		add("y", enc.Y.Scale, enc.Y.Type)
	}
	if enc.X2 != nil {
		add("x2", enc.X2.Scale, enc.X2.Type)
	}
	if enc.Y2 != nil {
		add("y2", enc.Y2.Scale, enc.Y2.Type)
	}
	if enc.Theta != nil {
		add("theta", enc.Theta.Scale, enc.Theta.Type)
	}
	if enc.Radius != nil {
		add("radius", enc.Radius.Scale, enc.Radius.Type)
	}
	if enc.Color != nil {
		add("color", enc.Color.Scale, enc.Color.Type)
	}
	if enc.Fill != nil {
		add("fill", enc.Fill.Scale, enc.Fill.Type)
	}
	if enc.Stroke != nil {
		add("stroke", enc.Stroke.Scale, enc.Stroke.Type)
	}
	if enc.Opacity != nil {
		add("opacity", enc.Opacity.Scale, enc.Opacity.Type)
	}
	if enc.Size != nil {
		add("size", enc.Size.Scale, enc.Size.Type)
	}
	if enc.Shape != nil {
		add("shape", enc.Shape.Scale, enc.Shape.Type)
	}
	return out
}

// scaleDomainFamily classifies a binding as "continuous", "time",
// "discrete" or "" (unknown).
func scaleDomainFamily(b scaleDomainBinding) string {
	switch b.scale.Type {
	case "linear", "log", "pow", "sqrt":
		return "continuous"
	case "time":
		return "time"
	case "band", "point", "ordinal":
		return "discrete"
	}
	switch b.measure {
	case "quantitative":
		return "continuous"
	case "temporal":
		return "time"
	case "nominal", "ordinal":
		return "discrete"
	}
	return ""
}

func checkScaleDomain(b scaleDomainBinding) []*errors.AppError {
	family := scaleDomainFamily(b)
	if family == "" {
		return nil
	}
	items, ok := b.scale.Domain.([]any)
	if !ok {
		return []*errors.AppError{scaleDomainErr(b, family,
			fmt.Sprintf("scale.domain must be an array, got %T", b.scale.Domain))}
	}
	switch family {
	case "discrete":
		if len(items) == 0 {
			return []*errors.AppError{scaleDomainErr(b, family, "a discrete scale.domain needs at least one category")}
		}
		for _, v := range items {
			if _, isStr := v.(string); !isStr {
				return []*errors.AppError{scaleDomainErr(b, family,
					fmt.Sprintf("a discrete scale.domain lists string categories, got %v", v))}
			}
		}
		return nil
	case "time":
		if len(items) != 2 {
			return []*errors.AppError{scaleDomainErr(b, family,
				fmt.Sprintf("a time scale.domain needs exactly 2 bounds, got %d", len(items)))}
		}
		for _, v := range items {
			switch t := v.(type) {
			case float64, int, int64:
			case string:
				if t == "" {
					return []*errors.AppError{scaleDomainErr(b, family, "a time scale.domain bound cannot be an empty string")}
				}
			default:
				return []*errors.AppError{scaleDomainErr(b, family,
					fmt.Sprintf("a time scale.domain bound must be a date string or epoch-ms number, got %v", v))}
			}
		}
		return nil
	}
	if len(items) != 2 {
		return []*errors.AppError{scaleDomainErr(b, family,
			fmt.Sprintf("a continuous scale.domain needs exactly 2 bounds, got %d", len(items)))}
	}
	lo, ok1 := scaleDomainNumber(items[0])
	hi, ok2 := scaleDomainNumber(items[1])
	if !ok1 || !ok2 {
		return []*errors.AppError{scaleDomainErr(b, family,
			fmt.Sprintf("a continuous scale.domain needs numeric bounds, got [%v, %v]", items[0], items[1]))}
	}
	if lo >= hi {
		return []*errors.AppError{scaleDomainErr(b, family,
			fmt.Sprintf("scale.domain bounds must ascend, got [%v, %v]", items[0], items[1]))}
	}
	return nil
}

func scaleDomainNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func scaleDomainErr(b scaleDomainBinding, family, reason string) *errors.AppError {
	return errors.New("PRISM_SPEC_041",
		fmt.Sprintf("Channel %q has a malformed scale.domain: %s.", b.channel, reason),
		map[string]any{
			"Channel": b.channel,
			"Family":  family,
			"Reason":  reason,
		},
	)
}
