package build

import (
	"fmt"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/geodata"
	"github.com/frankbardon/prism/spec"
)

// materializeFeatureCollection turns a spec.FeatureCollectionRef into
// the inline-row shape the InlineNode consumes: one row per feature in
// the requested tier with columns {id, name, parent}.
//
// id     — geodata feature ID (ISO 3166-1 alpha-3 or ISO 3166-2)
// name   — Natural Earth NAME (admin-0) or name field (admin-1)
// parent — admin-0 parent for admin-1 entries; empty otherwise
//
// The synthesized table is small (≤ ~500 rows for the v1 catalog),
// O(N) build cost, no network — pure manifest read.
func materializeFeatureCollection(ref *spec.FeatureCollectionRef) ([]map[string]any, []spec.FieldSpec, error) {
	tier := geodata.TierWorld110m
	if ref != nil && ref.Tier != "" {
		tier = geodata.Tier(ref.Tier)
	}
	m, err := geodata.LoadManifest()
	if err != nil {
		return nil, nil, prismerrors.New(
			"PRISM_GEO_002",
			fmt.Sprintf("Geo manifest unavailable: %v.", err),
			map[string]any{"Tier": string(tier), "Reason": err.Error()},
		)
	}
	ids := m.FeatureIDsForTier(tier)
	if len(ids) == 0 {
		return nil, nil, prismerrors.New(
			"PRISM_GEO_002",
			fmt.Sprintf("Geo manifest tier %q is empty.", tier),
			map[string]any{"Tier": string(tier), "Reason": "no features in tier"},
		)
	}
	rows := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		meta, _ := m.Lookup(id)
		row := map[string]any{"id": id, "name": ""}
		if meta != nil {
			row["name"] = meta.Name
			if meta.Parent != "" {
				row["parent"] = meta.Parent
			} else {
				row["parent"] = ""
			}
		} else {
			row["parent"] = ""
		}
		rows = append(rows, row)
	}
	fields := []spec.FieldSpec{
		{Name: "id", Type: "nominal"},
		{Name: "name", Type: "nominal"},
		{Name: "parent", Type: "nominal"},
	}
	return rows, fields, nil
}
