// Package scene holds the Prism Scene IR — the renderer-agnostic
// intermediate representation produced by the encode stage and
// consumed by every Renderer (SVG ships in P05; PNG / PDF / canvas
// land in later phases). Types mirror design/06-scene-ir.md verbatim.
//
// Coordinates are pre-resolved to pixel space. Renderers do not run
// scale math.
//
// JSON serialisation is the cross-implementation contract (D011):
// the JS port consumes the same shape via the Node test harness in
// P12. Every field uses snake_case JSON tags per D019.
//
// Mark uses a discriminated union with nine nullable *Geom pointers
// (only one populated). The wasted bytes (~70 per mark) buy us
// byte-stable round-trips without per-type MarshalJSON dispatch.
package scene

// SceneDoc is the top-level Scene IR document. Renderer entry point.
type SceneDoc struct {
	Version  string       `json:"version"`
	Theme    *Theme       `json:"theme,omitempty"`
	Grid     SceneGrid    `json:"grid"`
	Datasets []DatasetRef `json:"datasets,omitempty"`
	Warnings []Warning    `json:"warnings,omitempty"`
}

// DatasetRef carries back-references to the named datasets that fed
// this scene. Browser dataset-registry uses these to skip refetch on
// re-render when no upstream dataset changed.
type DatasetRef struct {
	Name string `json:"name"`
	Hash string `json:"hash,omitempty"`
}

// CurrentVersion is the SceneDoc.Version every encoder emits. Bump
// in lockstep with the JS port; additive changes keep "1.0".
const CurrentVersion = "1.0"

// NewDoc returns a SceneDoc skeleton with Version pinned and Theme
// set to Default(). The encoder fills Grid before returning.
func NewDoc() *SceneDoc {
	return &SceneDoc{
		Version: CurrentVersion,
		Theme:   Default(),
	}
}
