package marks

import (
	"errors"
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/projection"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/geodata"
)

// encodeGeoshape emits one scene.Mark per polygon piece of every
// row's feature. Multipolygon features (e.g. Canada with its
// archipelago) produce multiple marks, each carrying its own
// PolygonGeom and the same Datum back-reference so hit-testing groups
// them by source row.
//
// Required inputs:
//
//	in.Feature.Field  — table column holding geodata IDs
//	in.Projection     — non-nil projection (defaults applied by encoder)
//
// Color channel (optional) drives choropleth fill; Style.Fill is the
// fallback when no color binding exists.
func encodeGeoshape(in Inputs) ([]scene.Mark, error) {
	if in.Feature.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"geoshape mark requires an encoding.feature binding.",
			map[string]any{"Field": "<feature>", "Source": "<spec>", "Available": "feature.field=<id-column>"},
		)
	}
	if in.Projection == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"geoshape mark requires a projection (spec.projection.type).",
			map[string]any{"Field": "<projection>", "Source": "<spec>", "Available": joinStrings(projection.Available())},
		)
	}
	ids, err := readField(in.Table, in.Feature.Field)
	if err != nil {
		return nil, err
	}
	store := in.GeoStore
	if store == nil {
		store = geodata.DefaultStore()
	}
	tier := in.GeoTier
	if tier == "" {
		tier = geodata.TierWorld110m
	}

	// Surface tier-bundle load failures (unset directory / missing tier
	// file) up front so a geo mark fails loudly with a coded error rather
	// than silently skipping its layer when no feature row triggers a
	// lazy load.
	if err := store.Preload(tier); err != nil {
		return nil, mapGeoStoreError(tier, err)
	}

	var colorValues []any
	if in.Color != nil && in.Color.Field != "" {
		cv, err := readField(in.Table, in.Color.Field)
		if err != nil {
			return nil, err
		}
		colorValues = cv
	}

	out := make([]scene.Mark, 0, len(ids))
	for i, raw := range ids {
		id, ok := raw.(string)
		if !ok || id == "" {
			continue
		}
		feat, err := store.Lookup(tier, id)
		if err != nil {
			return nil, prismerrors.New(
				"PRISM_GEO_001",
				fmt.Sprintf("Feature %q not found in tier %q.", id, tier),
				map[string]any{"Field": in.Feature.Field, "Source": string(tier), "Available": id},
			)
		}
		style := in.Style
		if in.Color != nil && i < len(colorValues) {
			if cat, ok := colorValues[i].(string); ok {
				c := lookupCategoryColor(cat, in.Color.Categories, in.Color.Palette)
				if c != nil {
					style.Fill = c
				}
			}
		}
		for pi, poly := range feat.Polygons {
			projected := projectPolygon(in.Projection, poly)
			if len(projected.Outer) < 3 {
				continue
			}
			out = append(out, scene.Mark{
				Type:     scene.MarkGeoshape,
				ID:       fmt.Sprintf("geoshape-%d-%d", i, pi),
				Style:    style,
				Geoshape: projected,
			})
		}
	}
	return out, nil
}

// mapGeoStoreError translates a geodata store/load failure into the
// matching PRISM_* envelope. The unset-directory and missing-tier-file
// cases get dedicated, fixup-bearing codes; anything else falls back to
// the generic geo-bundle-load code.
func mapGeoStoreError(tier geodata.Tier, err error) error {
	if errors.Is(err, geodata.ErrBundleDirUnset) {
		return prismerrors.Wrap(
			"PRISM_GEODATA_DIR_UNSET",
			fmt.Sprintf("Geodata bundle directory is not configured; cannot load tier %q.", tier),
			map[string]any{"Tier": string(tier)},
			err,
		)
	}
	var missing *geodata.TierMissingError
	if errors.As(err, &missing) {
		return prismerrors.Wrap(
			"PRISM_GEODATA_TIER_MISSING",
			fmt.Sprintf("Geodata tier file for %q not found at %s.", missing.Tier, missing.Path),
			map[string]any{"Tier": string(missing.Tier), "Path": missing.Path},
			err,
		)
	}
	return prismerrors.Wrap(
		"PRISM_GEO_002",
		fmt.Sprintf("Geo bundle could not be loaded for tier %q.", tier),
		map[string]any{"Tier": string(tier), "Reason": err.Error()},
		err,
	)
}

// projectPolygon projects every ring in poly through p. Points where
// the projection clips (ok=false) are dropped silently; the resulting
// ring may be empty.
func projectPolygon(p projection.Projection, poly geodata.Polygon) *scene.PolygonGeom {
	out := &scene.PolygonGeom{Outer: projectRing(p, poly.Outer)}
	if len(poly.Holes) > 0 {
		out.Holes = make([][][2]float64, 0, len(poly.Holes))
		for _, h := range poly.Holes {
			ring := projectRing(p, h)
			if len(ring) >= 3 {
				out.Holes = append(out.Holes, ring)
			}
		}
	}
	return out
}

func projectRing(p projection.Projection, in geodata.Ring) [][2]float64 {
	out := make([][2]float64, 0, len(in))
	for _, pt := range in {
		x, y, ok := p.Project(pt[0], pt[1])
		if !ok {
			continue
		}
		if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			continue
		}
		out = append(out, [2]float64{x, y})
	}
	return out
}

func joinStrings(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += "|"
		}
		out += s
	}
	return out
}
