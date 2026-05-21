package rules

import (
	"fmt"
	"slices"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// AnimationEasingKnown implements PRISM_SPEC_022: when an animation
// block declares an easing, the name must be one of
// spec.AnimationEasings. An empty / omitted easing is fine (the
// encoder applies the default at scene-emit time).
type AnimationEasingKnown struct{}

// Code returns PRISM_SPEC_022.
func (AnimationEasingKnown) Code() string { return "PRISM_SPEC_022" }

// Check fires when animation.easing is non-empty and unrecognised.
func (AnimationEasingKnown) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Animation == nil || s.Animation.Easing == "" {
		return nil
	}
	if slices.Contains(spec.AnimationEasings, s.Animation.Easing) {
		return nil
	}
	return []*errors.AppError{
		errors.New("PRISM_SPEC_022",
			fmt.Sprintf("animation.easing %q is not a known easing name.", s.Animation.Easing),
			map[string]any{"Easing": s.Animation.Easing},
		),
	}
}
