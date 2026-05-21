package rules

import (
	"fmt"

	"github.com/frankbardon/prism/spec"
)

// channelCondition pairs a channel's name with the underlying common
// fields and its condition clause. The pointer to ChannelCommon is
// used so callers can read the channel's own value/field fallback.
type channelCondition struct {
	Channel string
	Path    string
	Common  *spec.ChannelCommon
	Cond    *spec.Condition
}

// walkConditionsTree returns every condition-bearing channel across
// the spec tree, including layer / concat / facet / repeat children.
// The Path is a dotted slug describing the spec node so error reports
// can point users at the right layer (e.g. "layer[1].encoding.color").
func walkConditionsTree(s *spec.Spec) []channelCondition {
	if s == nil {
		return nil
	}
	var out []channelCondition
	collect := func(prefix string, sub *spec.Spec) {
		out = append(out, channelConditionsAt(prefix, sub)...)
	}
	collect("", s)
	for i, l := range s.Layer {
		if l == nil {
			continue
		}
		collect(prefixf("layer[%d]", i), l)
	}
	for i, c := range s.Concat {
		if c == nil {
			continue
		}
		collect(prefixf("concat[%d]", i), c)
	}
	for i, c := range s.HConcat {
		if c == nil {
			continue
		}
		collect(prefixf("hconcat[%d]", i), c)
	}
	for i, c := range s.VConcat {
		if c == nil {
			continue
		}
		collect(prefixf("vconcat[%d]", i), c)
	}
	if s.ChildSpec != nil {
		collect("spec", s.ChildSpec)
	}
	return out
}

func channelConditionsAt(prefix string, s *spec.Spec) []channelCondition {
	if s == nil || s.Encoding == nil {
		return nil
	}
	var out []channelCondition
	e := s.Encoding
	add := func(name string, common *spec.ChannelCommon) {
		if common == nil || common.Condition == nil {
			return
		}
		path := name
		if prefix != "" {
			path = prefix + ".encoding." + name
		} else {
			path = "encoding." + name
		}
		out = append(out, channelCondition{
			Channel: name,
			Path:    path,
			Common:  common,
			Cond:    common.Condition,
		})
	}
	if e.X != nil {
		add("x", &e.X.ChannelCommon)
	}
	if e.Y != nil {
		add("y", &e.Y.ChannelCommon)
	}
	if e.X2 != nil {
		add("x2", &e.X2.ChannelCommon)
	}
	if e.Y2 != nil {
		add("y2", &e.Y2.ChannelCommon)
	}
	if e.Theta != nil {
		add("theta", &e.Theta.ChannelCommon)
	}
	if e.Radius != nil {
		add("radius", &e.Radius.ChannelCommon)
	}
	if e.Color != nil {
		add("color", &e.Color.ChannelCommon)
	}
	if e.Fill != nil {
		add("fill", &e.Fill.ChannelCommon)
	}
	if e.Stroke != nil {
		add("stroke", &e.Stroke.ChannelCommon)
	}
	if e.Opacity != nil {
		add("opacity", &e.Opacity.ChannelCommon)
	}
	if e.Size != nil {
		add("size", &e.Size.ChannelCommon)
	}
	if e.Shape != nil {
		add("shape", &e.Shape.ChannelCommon)
	}
	if e.Source != nil {
		add("source", &e.Source.ChannelCommon)
	}
	if e.Target != nil {
		add("target", &e.Target.ChannelCommon)
	}
	if e.Value != nil {
		add("value", &e.Value.ChannelCommon)
	}
	if e.Longitude != nil {
		add("longitude", &e.Longitude.ChannelCommon)
	}
	if e.Latitude != nil {
		add("latitude", &e.Latitude.ChannelCommon)
	}
	if e.Feature != nil {
		add("feature", &e.Feature.ChannelCommon)
	}
	return out
}

func prefixf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
