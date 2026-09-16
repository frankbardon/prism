package marks

import "github.com/frankbardon/prism/encode/scene"

// curveFor maps the mark-level `interpolate` / `tension` properties
// onto the scene IR curve discriminator that render/svg turns into
// path geometry.
//
// The accepted vocabulary is exactly the six scene.CurveType values —
// "linear", "monotone", "step", "step-before", "step-after" and
// "cardinal". The basis / bundle families and d3's "-open" / "-closed"
// variants are out of scope by design and rejected by
// schema/v1/mark.schema.json, so an unrecognised string here can only
// reach us from a caller that bypassed validation; it degrades to
// scene.CurveLinear rather than failing the render.
//
// The returned tension is meaningful for scene.CurveCardinal only and
// is clamped to [0, 1] (the schema bound). Every other curve reports 0
// so the emitted geom stays byte-identical to the pre-curve scene JSON.
func curveFor(in Inputs) (scene.CurveType, float64) {
	if in.Mark == nil {
		return scene.CurveLinear, 0
	}
	curve := scene.CurveLinear
	switch in.Mark.Interpolate {
	case string(scene.CurveMonotone):
		curve = scene.CurveMonotone
	case string(scene.CurveStep):
		curve = scene.CurveStep
	case string(scene.CurveStepBefore):
		curve = scene.CurveStepBefore
	case string(scene.CurveStepAfter):
		curve = scene.CurveStepAfter
	case string(scene.CurveCardinal):
		curve = scene.CurveCardinal
	}
	if curve != scene.CurveCardinal || in.Mark.Tension == nil {
		return curve, 0
	}
	t := *in.Mark.Tension
	switch {
	case t < 0:
		t = 0
	case t > 1:
		t = 1
	}
	return curve, t
}
