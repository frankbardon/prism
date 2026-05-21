package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/encode/projection"
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/geodata"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// GeoProjection ensures the spec's projection block is valid and that
// geo marks have both a projection and the right channel binding.
type GeoProjection struct{}

func (GeoProjection) Code() string { return "PRISM_SPEC_021" }

func (GeoProjection) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	markType := ""
	if s.Mark != nil {
		markType = s.Mark.TypeName()
	}
	geo := markType == "geoshape" || markType == "geopoint"

	// Validate projection.type when set.
	if s.Projection != nil && s.Projection.Type != "" {
		if !isKnownProjection(s.Projection.Type) {
			out = append(out, errors.New("PRISM_SPEC_021",
				fmt.Sprintf("Unknown projection type %q.", s.Projection.Type),
				map[string]any{"Field": "projection.type", "Available": strings.Join(projection.Available(), "|")},
			))
		}
		if s.Projection.Tier != "" && !isKnownTier(s.Projection.Tier) {
			out = append(out, errors.New("PRISM_SPEC_021",
				fmt.Sprintf("Unknown projection.tier %q.", s.Projection.Tier),
				map[string]any{"Field": "projection.tier", "Available": "world-110m|world-50m|admin1-50m"},
			))
		}
	}

	// Geo marks need both a projection and the right channel.
	if geo {
		if s.Projection == nil || s.Projection.Type == "" {
			out = append(out, errors.New("PRISM_SPEC_021",
				fmt.Sprintf("Mark %q requires a projection (set spec.projection.type).", markType),
				map[string]any{"Field": "projection", "Mark": markType},
			))
		}
		if s.Encoding == nil {
			return out
		}
		switch markType {
		case "geoshape":
			if s.Encoding.Feature == nil || s.Encoding.Feature.Field == "" {
				out = append(out, errors.New("PRISM_SPEC_021",
					"geoshape mark requires encoding.feature.field.",
					map[string]any{"Field": "encoding.feature", "Mark": markType},
				))
			}
		case "geopoint":
			if s.Encoding.Longitude == nil || s.Encoding.Longitude.Field == "" ||
				s.Encoding.Latitude == nil || s.Encoding.Latitude.Field == "" {
				out = append(out, errors.New("PRISM_SPEC_021",
					"geopoint mark requires encoding.longitude.field and encoding.latitude.field.",
					map[string]any{"Field": "encoding.longitude|latitude", "Mark": markType},
				))
			}
		}
	}
	return out
}

func isKnownProjection(name string) bool {
	for _, n := range projection.Available() {
		if n == name {
			return true
		}
	}
	return false
}

func isKnownTier(t string) bool {
	for _, kt := range geodata.AllTiers() {
		if string(kt) == t {
			return true
		}
	}
	return false
}
