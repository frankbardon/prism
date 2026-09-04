package scene

import "github.com/frankbardon/prism/table"

// Custom is the Scene IR node for a custom mark (E2). Like Table, a
// Custom carries no positioned geometry of its own — a custom mark is
// freeform/document-flow and never resolves a shared x/y scale or
// layout algorithm — so it hangs off Scene.Custom as a sibling to
// Layers rather than a Mark geometry variant; Mark.Validate()'s
// "exactly one geometry pointer" invariant is untouched. Additive to
// the Scene IR stability contract — no existing node type changes
// shape.
//
// Unlike Table (HTML-only), a Custom node IS consumed by the SVG
// backend directly (render/svg/custom.go): the render step resolves
// Renderer against the active custommark registry and calls
// RenderSVG/RenderHTML with Rows + Box + the resolved theme tokens.
type Custom struct {
	// Renderer names the registered CustomRenderer to invoke (mirrors
	// spec.MarkDef.Renderer). Resolved against the active registry at
	// render time, not here — encode has no opinion on whether the
	// name is actually registered; an unresolved name is a render-
	// time error (PRISM_RENDER_CUSTOM_MARK_NOT_FOUND).
	Renderer string `json:"renderer"`
	// Box is the content area a render backend allots to this mark —
	// dimensions only, no position (see Box's own doc comment). The
	// render backend positions the rendered fragment at the scene's
	// plot origin.
	Box Box `json:"box"`
	// Rows is the resolved, upstream-filtered/sorted/limited row set
	// the custom renderer receives, already materialised via
	// table.Table.Rows() at encode time — the same transform-pipeline
	// resolution any other mark's upstream table gets.
	Rows []table.Row `json:"rows,omitempty"`
}
