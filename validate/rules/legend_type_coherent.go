package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// LegendTypeCoherent implements PRISM_SPEC_051: `legend.type` must
// name a legend form the channel can actually produce.
//
// E3-S4 made `legend.type` live — it overrides the form the channel
// would otherwise infer from its own `type`. That makes a mismatched
// override no longer inert: a gradient bar needs a continuous domain
// to run between, and a column of category swatches needs categories
// to name. Asking for the wrong one yields an empty legend, which the
// author would see as "my legend disappeared" with no diagnostic.
//
// The inference this guards is in encode.ResolveLegendKind: a
// quantitative channel reads as a ramp, everything else as
// categories. Omitting `legend.type` always picks the coherent form,
// so this rule only ever fires on an explicit override.
type LegendTypeCoherent struct{}

// Code returns PRISM_SPEC_051.
func (LegendTypeCoherent) Code() string { return "PRISM_SPEC_051" }

// Check walks every legend block in the spec tree, emitting one error
// per incoherent override.
func (LegendTypeCoherent) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, b := range walkLegendBlocks(s) {
		if b.Legend.Type == "" {
			continue
		}
		continuous := b.ChannelType == "quantitative"
		wantGradient := b.Legend.Type == "gradient"
		if continuous == wantGradient {
			continue
		}
		want := "a continuous quantitative channel"
		if !wantGradient {
			want = "a discrete channel (nominal, ordinal or temporal)"
		}
		out = append(out, errors.New("PRISM_SPEC_051",
			fmt.Sprintf("Channel %q declares legend.type %q, which needs %s; the channel is %q.",
				b.Channel, b.Legend.Type, want, b.ChannelType),
			map[string]any{
				"Channel":     b.Channel,
				"Type":        b.Legend.Type,
				"ChannelType": b.ChannelType,
				"Path":        b.Path,
			},
		))
	}
	return out
}

// legendBinding is one channel's declared `legend` block. Path is the
// dotted slug naming the spec node it was found on ("" for the root,
// "layer[1]", "concat[0]", …) so an error can point at the right leaf.
//
// It is shared with validate/rules/legend_symbol_type.go
// (PRISM_SPEC_052), the other rule over the same blocks.
type legendBinding struct {
	Path        string
	Channel     string
	ChannelType string
	Legend      *spec.Legend
}

// walkLegendBlocks collects every declared `legend` block across the
// spec tree, including layer / concat / facet / repeat children. A
// channel with no legend block, or one suppressed with `"legend":
// null`, contributes nothing — there is no legend left to configure.
func walkLegendBlocks(s *spec.Spec) []legendBinding {
	if s == nil {
		return nil
	}
	var out []legendBinding
	collect := func(prefix string, sub *spec.Spec) {
		out = append(out, legendBlocksAt(prefix, sub)...)
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

// legendBlocksAt returns the legend blocks declared directly on s,
// over the mark-property channels a legend can describe.
func legendBlocksAt(prefix string, s *spec.Spec) []legendBinding {
	if s == nil || s.Encoding == nil {
		return nil
	}
	channels := []struct {
		name string
		ch   *spec.MarkChannel
	}{
		{"color", s.Encoding.Color},
		{"fill", s.Encoding.Fill},
		{"stroke", s.Encoding.Stroke},
		{"opacity", s.Encoding.Opacity},
		{"size", s.Encoding.Size},
		{"shape", s.Encoding.Shape},
	}
	var out []legendBinding
	for _, c := range channels {
		if c.ch == nil || c.ch.Legend == nil {
			continue
		}
		out = append(out, legendBinding{
			Path:        prefix,
			Channel:     c.name,
			ChannelType: c.ch.Type,
			Legend:      c.ch.Legend,
		})
	}
	return out
}
