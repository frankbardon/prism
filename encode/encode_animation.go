package encode

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// animationFromSpec projects spec.Animation onto scene.Animation,
// applying defaults at the field level. Returns nil when the spec
// has no animation block — the scene-IR field stays omitempty so
// existing goldens are byte-identical.
func animationFromSpec(s *spec.Spec) *scene.Animation {
	if s == nil || s.Animation == nil {
		return nil
	}
	a := s.Animation
	out := &scene.Animation{
		DurationMs: spec.AnimationDefaultDurationMs,
		Easing:     spec.AnimationDefaultEasing,
		StaggerMs:  spec.AnimationDefaultStaggerMs,
		Enter:      spec.AnimationDefaultEnter,
		Exit:       spec.AnimationDefaultExit,
	}
	if a.DurationMs != nil {
		out.DurationMs = *a.DurationMs
	}
	if a.Easing != "" {
		out.Easing = a.Easing
	}
	if a.StaggerMs != nil {
		out.StaggerMs = *a.StaggerMs
	}
	if a.Enter != "" {
		out.Enter = a.Enter
	}
	if a.Exit != "" {
		out.Exit = a.Exit
	}
	return out
}

// keyFieldFromEncoding returns the field name of the first encoding
// channel that carries key:true, or "" if none. Channel uniqueness
// is enforced by validation rule PRISM_SPEC_024 — this helper takes
// the first match in a stable channel order so post-validation
// behaviour is deterministic.
func keyFieldFromEncoding(e *spec.Encoding) string {
	if e == nil {
		return ""
	}
	if e.X != nil && e.X.Key {
		return e.X.Field
	}
	if e.Y != nil && e.Y.Key {
		return e.Y.Field
	}
	if e.X2 != nil && e.X2.Key {
		return e.X2.Field
	}
	if e.Y2 != nil && e.Y2.Key {
		return e.Y2.Field
	}
	if e.Theta != nil && e.Theta.Key {
		return e.Theta.Field
	}
	if e.Radius != nil && e.Radius.Key {
		return e.Radius.Field
	}
	if e.Color != nil && e.Color.Key {
		return e.Color.Field
	}
	if e.Fill != nil && e.Fill.Key {
		return e.Fill.Field
	}
	if e.Stroke != nil && e.Stroke.Key {
		return e.Stroke.Field
	}
	if e.Opacity != nil && e.Opacity.Key {
		return e.Opacity.Field
	}
	if e.Size != nil && e.Size.Key {
		return e.Size.Field
	}
	if e.Shape != nil && e.Shape.Key {
		return e.Shape.Field
	}
	if e.Source != nil && e.Source.Key {
		return e.Source.Field
	}
	if e.Target != nil && e.Target.Key {
		return e.Target.Field
	}
	if e.Value != nil && e.Value.Key {
		return e.Value.Field
	}
	if e.Longitude != nil && e.Longitude.Key {
		return e.Longitude.Field
	}
	if e.Latitude != nil && e.Latitude.Key {
		return e.Latitude.Field
	}
	if e.Feature != nil && e.Feature.Key {
		return e.Feature.Field
	}
	return ""
}
