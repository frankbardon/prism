package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// axisOrientsForChannel lists the `axis.orient` values that mean
// something on each position channel. An x axis runs horizontally and
// can only hang above or below the plot; a y axis runs vertically and
// can only sit to its left or right. The span channels are included
// because they are position channels too — they draw no axis of their
// own, but an orient written on one is still an author mistake worth
// naming rather than dropping.
var axisOrientsForChannel = map[string][]string{
	"x":  {"bottom", "top"},
	"x2": {"bottom", "top"},
	"y":  {"left", "right"},
	"y2": {"left", "right"},
}

// AxisOrientChannel implements PRISM_SPEC_044: `axis.orient` must name
// a side the channel's axis can actually occupy.
//
// E1-S2 made orient live — it moves the axis and the padding its side
// reserves. That makes a cross-axis value ("left" on x) no longer an
// inert typo: without this rule the encoder would silently fall back
// to the default side and the author would see nothing move, with no
// diagnostic explaining why.
type AxisOrientChannel struct{}

// Code returns PRISM_SPEC_044.
func (AxisOrientChannel) Code() string { return "PRISM_SPEC_044" }

// Check walks every axis block in the spec tree. Emits at most one
// error per offending channel.
func (AxisOrientChannel) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, b := range walkAxisOrients(s) {
		allowed, ok := axisOrientsForChannel[b.Channel]
		if !ok {
			continue
		}
		if orientAllowed(b.Orient, allowed) {
			continue
		}
		out = append(out, errors.New("PRISM_SPEC_044",
			fmt.Sprintf("Axis orient %q is not a side the %q axis can occupy.", b.Orient, b.Channel),
			map[string]any{
				"Channel": b.Channel,
				"Orient":  b.Orient,
				"Allowed": strings.Join(allowed, ", "),
				"Path":    b.Path,
			},
		))
	}
	return out
}

// orientAllowed reports whether orient is one of the sides listed.
func orientAllowed(orient string, allowed []string) bool {
	for _, a := range allowed {
		if orient == a {
			return true
		}
	}
	return false
}

// axisOrientBinding is one channel's declared `axis.orient`. Path is a
// dotted slug naming the spec node it was found on ("" for the root,
// "layer[1]", "concat[0]", …) so an error can point at the right leaf.
type axisOrientBinding struct {
	Path    string
	Channel string
	Orient  string
}

// walkAxisOrients collects every declared `axis.orient` across the
// spec tree, including layer / concat / facet / repeat children. A
// channel with no axis block, or an axis block that sets no orient,
// contributes nothing.
func walkAxisOrients(s *spec.Spec) []axisOrientBinding {
	if s == nil {
		return nil
	}
	var out []axisOrientBinding
	collect := func(prefix string, sub *spec.Spec) {
		out = append(out, axisOrientsAt(prefix, sub)...)
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

// axisOrientsAt returns the orients declared directly on s.
func axisOrientsAt(prefix string, s *spec.Spec) []axisOrientBinding {
	if s == nil || s.Encoding == nil {
		return nil
	}
	channels := []struct {
		name string
		ch   *spec.PositionChannel
	}{
		{"x", s.Encoding.X},
		{"y", s.Encoding.Y},
		{"x2", s.Encoding.X2},
		{"y2", s.Encoding.Y2},
	}
	var out []axisOrientBinding
	for _, c := range channels {
		if c.ch == nil || c.ch.Axis == nil || c.ch.Axis.Orient == "" {
			continue
		}
		out = append(out, axisOrientBinding{
			Path:    prefix,
			Channel: c.name,
			Orient:  c.ch.Axis.Orient,
		})
	}
	return out
}
