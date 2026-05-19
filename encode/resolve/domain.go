package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// LayerDomain carries one layer's contribution to a channel's domain
// for the shared-resolution union. Type is the layer's resolved scale
// family; Values is the raw column data (any).
type LayerDomain struct {
	LayerID string
	Channel scene.Channel
	Type    scene.ScaleType
	Values  []any
}

// Unify collapses a set of per-layer domains into one shared scale
// type + unified domain. Numeric families collapse to numeric
// min/max; band / ordinal / point collapse to an ordered category
// union (first-seen across layers, dedup); time collapses to the
// numeric (epoch_ms) min/max.
//
// Mixed families raise PRISM_PLAN_005 (the channel cannot be shared;
// caller switches to independent or coerces upstream).
//
// Empty `layers` returns ScaleLinear + an empty domain — the caller
// should never invoke Unify on an empty set in practice (layers
// dropped via PRISM_WARN_LAYER_SKIPPED collapse to a single-layer
// resolve).
func Unify(layers []LayerDomain) (scene.ScaleType, []any, error) {
	if len(layers) == 0 {
		return scene.ScaleLinear, nil, nil
	}
	if len(layers) == 1 {
		return layers[0].Type, layers[0].Values, nil
	}

	family, typesSeen := classifyFamilies(layers)
	if family == familyMixed {
		return "", nil, incompatibleErr(layers[0].Channel, typesSeen)
	}

	switch family {
	case familyNumeric:
		return unifyNumeric(layers)
	case familyCategorical:
		return unifyCategorical(layers)
	case familyTemporal:
		return unifyTemporal(layers)
	}
	// Defensive: an unknown family path should be unreachable since
	// classifyFamilies returns one of the four enum values.
	return "", nil, fmt.Errorf("resolve.Unify: unknown family")
}

type family int

const (
	familyMixed family = iota
	familyNumeric
	familyCategorical
	familyTemporal
)

// classifyFamilies buckets each layer's scale type into one of three
// compatible families. Mixed-family input returns familyMixed plus
// the sorted set of types observed (for the error message).
func classifyFamilies(layers []LayerDomain) (family, []scene.ScaleType) {
	seen := map[scene.ScaleType]bool{}
	for _, l := range layers {
		seen[l.Type] = true
	}
	types := make([]scene.ScaleType, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	allNumeric := true
	allCat := true
	allTime := true
	for _, t := range types {
		if !isNumericScale(t) {
			allNumeric = false
		}
		if !isCategoricalScale(t) {
			allCat = false
		}
		if t != scene.ScaleTime {
			allTime = false
		}
	}
	switch {
	case allNumeric:
		return familyNumeric, types
	case allCat:
		return familyCategorical, types
	case allTime:
		return familyTemporal, types
	}
	return familyMixed, types
}

func isNumericScale(t scene.ScaleType) bool {
	switch t {
	case scene.ScaleLinear, scene.ScaleLog, scene.ScalePow, scene.ScaleSqrt:
		return true
	}
	return false
}

func isCategoricalScale(t scene.ScaleType) bool {
	switch t {
	case scene.ScaleBand, scene.ScalePoint, scene.ScaleOrdinal:
		return true
	}
	return false
}

// unifyNumeric returns ScaleLinear + the combined min/max as the
// domain. Even when one layer requested log/pow/sqrt, the shared
// domain is linear by default — the per-layer specifics carry into
// per-layer encodings, but the unified domain pin is the union.
// (Mixing log + linear under shared resolve is uncommon; users wanting
// log should set it consistently across layers.)
func unifyNumeric(layers []LayerDomain) (scene.ScaleType, []any, error) {
	var mn, mx float64
	first := true
	for _, l := range layers {
		for _, v := range l.Values {
			f, ok := scale.ToFloat(v)
			if !ok {
				continue
			}
			if first {
				mn, mx = f, f
				first = false
				continue
			}
			if f < mn {
				mn = f
			}
			if f > mx {
				mx = f
			}
		}
	}
	if first {
		return scene.ScaleLinear, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolve.Unify(numeric): no parseable numeric values across layers.",
			map[string]any{"Field": "<unify-numeric>", "Source": "<resolve>", "Available": "numeric"},
		)
	}
	return scene.ScaleLinear, []any{mn, mx}, nil
}

// unifyCategorical returns ScaleBand + ordered category union.
// First-seen order across layers is preserved; duplicates skipped.
func unifyCategorical(layers []LayerDomain) (scene.ScaleType, []any, error) {
	seen := map[string]bool{}
	var cats []any
	for _, l := range layers {
		for _, v := range l.Values {
			s, ok := v.(string)
			if !ok || seen[s] {
				continue
			}
			seen[s] = true
			cats = append(cats, s)
		}
	}
	if len(cats) == 0 {
		return scene.ScaleBand, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolve.Unify(categorical): no string categories across layers.",
			map[string]any{"Field": "<unify-categorical>", "Source": "<resolve>", "Available": "string"},
		)
	}
	return scene.ScaleBand, cats, nil
}

// unifyTemporal returns ScaleTime + the combined epoch_ms min/max.
// Domain values are returned as float64 (epoch milliseconds) so the
// caller can hand them straight to ResolveScaleTyped(ScaleTime, ...).
func unifyTemporal(layers []LayerDomain) (scene.ScaleType, []any, error) {
	var mn, mx float64
	first := true
	for _, l := range layers {
		for _, v := range l.Values {
			e, ok := scale.ToEpochMs(v)
			if !ok {
				continue
			}
			if first {
				mn, mx = e, e
				first = false
				continue
			}
			if e < mn {
				mn = e
			}
			if e > mx {
				mx = e
			}
		}
	}
	if first {
		return scene.ScaleTime, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"resolve.Unify(temporal): no parseable time values across layers.",
			map[string]any{"Field": "<unify-temporal>", "Source": "<resolve>", "Available": "iso-8601 | time.Time | epoch_ms"},
		)
	}
	return scene.ScaleTime, []any{mn, mx}, nil
}

// incompatibleErr formats PRISM_PLAN_005 with the channel and the
// set of disagreeing types.
func incompatibleErr(channel scene.Channel, types []scene.ScaleType) error {
	typeNames := make([]string, len(types))
	for i, t := range types {
		typeNames[i] = string(t)
	}
	return prismerrors.New(
		"PRISM_PLAN_005",
		fmt.Sprintf("Channel %s cannot be resolved as shared: layers disagree on type (%s).",
			channel, strings.Join(typeNames, ", ")),
		map[string]any{
			"Channel": string(channel),
			"Types":   strings.Join(typeNames, ", "),
		},
	)
}
