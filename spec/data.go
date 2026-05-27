package spec

import (
	"encoding/json"
	"fmt"
)

// Data is the data binding: source path, named ref, runtime resolver
// ref, inline values, or a synthesized feature_collection (geoshape
// basemap mode). The discriminator is which key is present.
type Data struct {
	Source string `json:"source,omitempty"`
	// Ref is an opaque identifier resolved at compile time by the
	// caller-supplied DataResolver (see resolve.DataResolver / the
	// WASM `prism.setDataResolver` hook). Lets a spec stay portable
	// across rendering environments: the spec describes *what to
	// draw*; the resolver supplies *the data to draw it with*.
	Ref    string `json:"ref,omitempty"`
	Format string `json:"format,omitempty"`
	Name   string `json:"name,omitempty"`

	// Inline-only fields.
	Values []map[string]any `json:"values,omitempty"`
	Fields []FieldSpec      `json:"fields,omitempty"`

	// FeatureCollection synthesizes a table with one row per feature in
	// the named geodata tier. Used for "render every country" basemap
	// charts: pair with mark=geoshape and the encoder walks the
	// embedded manifest. Tier defaults to "world-110m" when empty.
	FeatureCollection *FeatureCollectionRef `json:"feature_collection,omitempty"`
}

// FeatureCollectionRef binds a Data block to a geodata tier. Currently
// carries only the tier name; future fields could add per-feature
// filtering (regions=continent_codes, parent_in=[...]).
type FeatureCollectionRef struct {
	Tier string `json:"tier,omitempty"`
}

// FieldSpec optionally types an inline dataset column.
type FieldSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// UnmarshalJSON enforces strict decode and picks the variant from the keys
// present.
func (d *Data) UnmarshalJSON(data []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	hasSource := keyPresent(probe, "source")
	hasName := keyPresent(probe, "name")
	hasRef := keyPresent(probe, "ref")
	hasValues := keyPresent(probe, "values")
	hasFeatures := keyPresent(probe, "feature_collection")

	type rawData Data
	var r rawData
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	switch {
	case hasSource, hasValues, hasName, hasRef, hasFeatures:
		*d = Data(r)
	default:
		return fmt.Errorf("data: must declare one of source, name, ref, values, or feature_collection")
	}
	return nil
}

func keyPresent(m map[string]json.RawMessage, k string) bool {
	_, ok := m[k]
	return ok
}
