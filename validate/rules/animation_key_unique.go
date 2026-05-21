package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// AnimationKeyUnique implements PRISM_SPEC_024: at most one encoding
// channel in any given spec node may carry key:true. Composite keys
// are not supported in v1. The rule walks composition children
// (layer / concat / facet child) and emits one error per node that
// exceeds the cap.
type AnimationKeyUnique struct{}

// Code returns PRISM_SPEC_024.
func (AnimationKeyUnique) Code() string { return "PRISM_SPEC_024" }

// Check fires for every encoding block whose channels carry key:true
// on more than one entry.
func (AnimationKeyUnique) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	collectKeyErrors(s, &out)
	return out
}

func collectKeyErrors(s *spec.Spec, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	if names := keyChannelNames(s.Encoding); len(names) > 1 {
		sort.Strings(names)
		*out = append(*out, errors.New("PRISM_SPEC_024",
			fmt.Sprintf("multiple encoding channels carry `key: true` (channels: %s); at most one is allowed.", strings.Join(names, ", ")),
			map[string]any{"Channels": strings.Join(names, ", ")},
		))
	}
	for _, child := range s.Layer {
		collectKeyErrors(child, out)
	}
	for _, child := range s.Concat {
		collectKeyErrors(child, out)
	}
	for _, child := range s.HConcat {
		collectKeyErrors(child, out)
	}
	for _, child := range s.VConcat {
		collectKeyErrors(child, out)
	}
	if s.ChildSpec != nil {
		collectKeyErrors(s.ChildSpec, out)
	}
}

func keyChannelNames(e *spec.Encoding) []string {
	if e == nil {
		return nil
	}
	var names []string
	add := func(name string, has bool) {
		if has {
			names = append(names, name)
		}
	}
	add("x", e.X != nil && e.X.Key)
	add("y", e.Y != nil && e.Y.Key)
	add("x2", e.X2 != nil && e.X2.Key)
	add("y2", e.Y2 != nil && e.Y2.Key)
	add("theta", e.Theta != nil && e.Theta.Key)
	add("radius", e.Radius != nil && e.Radius.Key)
	add("color", e.Color != nil && e.Color.Key)
	add("fill", e.Fill != nil && e.Fill.Key)
	add("stroke", e.Stroke != nil && e.Stroke.Key)
	add("opacity", e.Opacity != nil && e.Opacity.Key)
	add("size", e.Size != nil && e.Size.Key)
	add("shape", e.Shape != nil && e.Shape.Key)
	add("source", e.Source != nil && e.Source.Key)
	add("target", e.Target != nil && e.Target.Key)
	add("value", e.Value != nil && e.Value.Key)
	add("longitude", e.Longitude != nil && e.Longitude.Key)
	add("latitude", e.Latitude != nil && e.Latitude.Key)
	add("feature", e.Feature != nil && e.Feature.Key)
	return names
}
