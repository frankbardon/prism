package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ResolveScaleCompat implements PRISM_PLAN_005 at validate time. When
// a composite spec uses `resolve.scale.<channel>: "shared"` AND the
// layers declare incompatible types on that channel (e.g. one layer
// nominal + another quantitative on `y`), the rule fires before the
// encoder runs. The encoder also catches the same issue (defense-in-
// depth via encode/resolve.Unify) so dynamic-type cases — where
// the spec leaves `type` implicit — are still trapped.
//
// The rule walks only the spec's top-level `layer` array; concat /
// hconcat / vconcat children are full charts that resolve scales
// independently per cell (see D050, D053), so cross-panel sharing is
// not in scope here.
type ResolveScaleCompat struct{}

// Code returns PRISM_PLAN_005.
func (ResolveScaleCompat) Code() string { return "PRISM_PLAN_005" }

// Check inspects every shared channel in resolve.scale and asserts
// type agreement across the layer array. Channels where any layer
// omits an explicit `encoding.<ch>.type` are skipped — the encoder's
// table-driven inference covers that case.
func (ResolveScaleCompat) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || len(s.Layer) == 0 || s.Resolve == nil || s.Resolve.Scale == nil {
		return nil
	}
	scale := s.Resolve.Scale
	// Enumerate channels whose mode is "shared". Iterate a fixed
	// ordering so error output is deterministic.
	channels := []struct {
		name string
		mode string
	}{
		{"x", scale.X}, {"y", scale.Y},
		{"x2", scale.X2}, {"y2", scale.Y2},
		{"color", scale.Color}, {"size", scale.Size},
		{"shape", scale.Shape}, {"opacity", scale.Opacity},
	}

	var out []*errors.AppError
	for _, c := range channels {
		if c.mode != "shared" {
			continue
		}
		types := layerChannelTypes(s.Layer, c.name)
		if len(types) < 2 {
			continue
		}
		if !typesAreCompatible(types) {
			sorted := sortedSet(types)
			out = append(out, errors.New("PRISM_PLAN_005",
				fmt.Sprintf("Channel %s cannot be resolved as shared: layers disagree on type (%s).",
					c.name, strings.Join(sorted, ", ")),
				map[string]any{
					"Channel": c.name,
					"Types":   strings.Join(sorted, ", "),
				},
			))
		}
	}
	return out
}

// layerChannelTypes collects the set of declared channel types
// (`encoding.<ch>.type`) across every layer. Empty strings (channel
// missing or type implicit) are dropped so the encoder's runtime
// inference handles the rest.
func layerChannelTypes(layers []*spec.Spec, channel string) map[string]bool {
	out := map[string]bool{}
	for _, layer := range layers {
		if layer == nil || layer.Encoding == nil {
			continue
		}
		t := channelType(layer.Encoding, channel)
		if t == "" {
			continue
		}
		out[t] = true
	}
	return out
}

func channelType(enc *spec.Encoding, channel string) string {
	switch channel {
	case "x":
		if enc.X != nil {
			return enc.X.Type
		}
	case "y":
		if enc.Y != nil {
			return enc.Y.Type
		}
	case "x2":
		if enc.X2 != nil {
			return enc.X2.Type
		}
	case "y2":
		if enc.Y2 != nil {
			return enc.Y2.Type
		}
	case "color":
		if enc.Color != nil {
			return enc.Color.Type
		}
	case "size":
		if enc.Size != nil {
			return enc.Size.Type
		}
	case "shape":
		if enc.Shape != nil {
			return enc.Shape.Type
		}
	case "opacity":
		if enc.Opacity != nil {
			return enc.Opacity.Type
		}
	}
	return ""
}

// typesAreCompatible bins the observed types into compatible
// families: quantitative, categorical (nominal / ordinal), temporal.
// Compatible types collapse to a single family; mixed families fail.
func typesAreCompatible(types map[string]bool) bool {
	hasQuant := false
	hasCat := false
	hasTime := false
	for t := range types {
		switch t {
		case "quantitative":
			hasQuant = true
		case "nominal", "ordinal":
			hasCat = true
		case "temporal":
			hasTime = true
		}
	}
	families := 0
	if hasQuant {
		families++
	}
	if hasCat {
		families++
	}
	if hasTime {
		families++
	}
	return families <= 1
}

func sortedSet(types map[string]bool) []string {
	out := make([]string, 0, len(types))
	for t := range types {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
