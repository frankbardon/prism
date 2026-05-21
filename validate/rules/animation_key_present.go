package rules

import (
	"slices"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// AnimationKeyPresent implements PRISM_SPEC_023: when an animation
// block is declared, the spec must expose a join key. Today the only
// supported source is an encoding channel with key:true; future
// versions may accept a top-level data.key field. Composition forms
// (layer / concat / facet / repeat) inherit animation from the parent
// spec, so the rule checks the current node's encoding plus any
// immediate child specs that override animation.
type AnimationKeyPresent struct{}

// Code returns PRISM_SPEC_023.
func (AnimationKeyPresent) Code() string { return "PRISM_SPEC_023" }

// Check fires when s.Animation is set but no descendant encoding
// declares key:true.
func (AnimationKeyPresent) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Animation == nil {
		return nil
	}
	if hasKeyChannel(s) {
		return nil
	}
	return []*errors.AppError{
		errors.New("PRISM_SPEC_023",
			"animation block declared but no encoding channel carries `key: true`.",
			map[string]any{},
		),
	}
}

func hasKeyChannel(s *spec.Spec) bool {
	if s == nil {
		return false
	}
	if encodingHasKey(s.Encoding) {
		return true
	}
	if slices.ContainsFunc(s.Layer, hasKeyChannel) {
		return true
	}
	if slices.ContainsFunc(s.Concat, hasKeyChannel) {
		return true
	}
	if slices.ContainsFunc(s.HConcat, hasKeyChannel) {
		return true
	}
	if slices.ContainsFunc(s.VConcat, hasKeyChannel) {
		return true
	}
	if s.ChildSpec != nil && hasKeyChannel(s.ChildSpec) {
		return true
	}
	return false
}

func encodingHasKey(e *spec.Encoding) bool {
	if e == nil {
		return false
	}
	if e.X != nil && e.X.Key {
		return true
	}
	if e.Y != nil && e.Y.Key {
		return true
	}
	if e.X2 != nil && e.X2.Key {
		return true
	}
	if e.Y2 != nil && e.Y2.Key {
		return true
	}
	if e.Theta != nil && e.Theta.Key {
		return true
	}
	if e.Radius != nil && e.Radius.Key {
		return true
	}
	if e.Color != nil && e.Color.Key {
		return true
	}
	if e.Fill != nil && e.Fill.Key {
		return true
	}
	if e.Stroke != nil && e.Stroke.Key {
		return true
	}
	if e.Opacity != nil && e.Opacity.Key {
		return true
	}
	if e.Size != nil && e.Size.Key {
		return true
	}
	if e.Shape != nil && e.Shape.Key {
		return true
	}
	if e.Source != nil && e.Source.Key {
		return true
	}
	if e.Target != nil && e.Target.Key {
		return true
	}
	if e.Value != nil && e.Value.Key {
		return true
	}
	if e.Longitude != nil && e.Longitude.Key {
		return true
	}
	if e.Latitude != nil && e.Latitude.Key {
		return true
	}
	if e.Feature != nil && e.Feature.Key {
		return true
	}
	return false
}
