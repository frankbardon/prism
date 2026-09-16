package rules

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ScaleBandGeometry implements PRISM_SPEC_049: the band-geometry knobs
// on a scale block must sit inside the ranges their geometry is
// defined over.
//
//   - `padding`, `padding_inner` and `padding_outer` are fractions of
//     the step, so they live in [0,1). An inner padding of 1 collapses
//     every band to zero width; anything beyond that is meaningless.
//   - `align` is a position inside the slack left over after the bands
//     are laid out, so it lives in [0,1].
//
// The encoder pins an out-of-range value into its legal range rather
// than drawing an inside-out band, so without this rule the author
// gets silently different geometry. The JSON Schema carries the same
// bounds; this rule is what turns the violation into a PRISM code with
// fixups.
type ScaleBandGeometry struct{}

// Code returns PRISM_SPEC_049.
func (ScaleBandGeometry) Code() string { return "PRISM_SPEC_049" }

// Check walks every channel carrying a scale block, on the root spec
// and on every composition child.
func (ScaleBandGeometry) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walkScaleGeometrySpecs(s, func(sub *spec.Spec) {
		for _, b := range scaleGeometryBindings(sub.Encoding) {
			out = append(out, checkScaleBandGeometry(b.channel, b.scale)...)
		}
	})
	return out
}

// scaleGeometryBinding pairs a channel name with its scale block.
type scaleGeometryBinding struct {
	channel string
	scale   *spec.Scale
}

// walkScaleGeometrySpecs visits the spec and every composition child
// that can carry its own encoding.
func walkScaleGeometrySpecs(s *spec.Spec, fn func(*spec.Spec)) {
	if s == nil {
		return
	}
	fn(s)
	for _, group := range [][]*spec.Spec{s.Layer, s.Concat, s.HConcat, s.VConcat} {
		for _, child := range group {
			walkScaleGeometrySpecs(child, fn)
		}
	}
	walkScaleGeometrySpecs(s.ChildSpec, fn)
}

// scaleGeometryBindings collects every channel on an encoding that
// declares a scale block.
func scaleGeometryBindings(enc *spec.Encoding) []scaleGeometryBinding {
	if enc == nil {
		return nil
	}
	var out []scaleGeometryBinding
	add := func(name string, ch *spec.PositionChannel) {
		if ch == nil || ch.Scale == nil {
			return
		}
		out = append(out, scaleGeometryBinding{channel: name, scale: ch.Scale})
	}
	addMark := func(name string, ch *spec.MarkChannel) {
		if ch == nil || ch.Scale == nil {
			return
		}
		out = append(out, scaleGeometryBinding{channel: name, scale: ch.Scale})
	}
	add("x", enc.X)
	add("y", enc.Y)
	add("x2", enc.X2)
	add("y2", enc.Y2)
	add("theta", enc.Theta)
	add("radius", enc.Radius)
	addMark("color", enc.Color)
	addMark("fill", enc.Fill)
	addMark("stroke", enc.Stroke)
	addMark("opacity", enc.Opacity)
	addMark("size", enc.Size)
	addMark("shape", enc.Shape)
	return out
}

// checkScaleBandGeometry validates one scale block's padding / align
// knobs.
func checkScaleBandGeometry(channel string, sc *spec.Scale) []*errors.AppError {
	var out []*errors.AppError
	for _, k := range []struct {
		name  string
		value *float64
	}{
		{"padding", sc.Padding},
		{"padding_inner", sc.PaddingInner},
		{"padding_outer", sc.PaddingOuter},
	} {
		if k.value == nil {
			continue
		}
		v := *k.value
		if math.IsNaN(v) || v < 0 || v >= 1 {
			out = append(out, scaleBandGeometryErr(channel, k.name, v, "[0,1)"))
		}
	}
	if sc.Align != nil {
		v := *sc.Align
		if math.IsNaN(v) || v < 0 || v > 1 {
			out = append(out, scaleBandGeometryErr(channel, "align", v, "[0,1]"))
		}
	}
	return out
}

func scaleBandGeometryErr(channel, property string, value float64, want string) *errors.AppError {
	return errors.New("PRISM_SPEC_049",
		fmt.Sprintf("Channel %q sets scale.%s to %v, outside %s.", channel, property, value, want),
		map[string]any{
			"Channel":  channel,
			"Property": property,
			"Value":    value,
			"Range":    want,
		},
	)
}
