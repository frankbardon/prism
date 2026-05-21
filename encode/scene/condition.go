package scene

// ConditionalAttr is one selection-driven attribute switch attached to
// a Mark. The browser-side selection layer toggles attributes by
// reading these entries when its selection state changes; the server
// renderers (SVG / PDF) ignore them so static output is unaffected.
//
// Static, expression-driven conditions (`{test: "..."}`) are evaluated
// at encode time and baked into Mark.Style — they never appear here.
// See `.planning/tier1-01-condition-encodings-plan.md`.
type ConditionalAttr struct {
	// Attr is the resolved SVG / scene-IR attribute name. One of
	// "fill", "stroke", "stroke_width", "opacity", "size" today.
	Attr string `json:"attr"`
	// Selection is the declared selection name whose active state
	// flips the attribute on. Empty selection means a degenerate
	// always-on entry (treated as static; the encoder normally bakes
	// such entries into Style and they should not appear here).
	Selection string `json:"selection,omitempty"`
	// WhenValue is the attribute value applied while Selection is
	// active. Type is per-attribute: hex strings for fill/stroke,
	// numbers for stroke_width/opacity/size.
	WhenValue any `json:"when_value,omitempty"`
	// Otherwise is the attribute value applied when Selection is
	// inactive. Mirrors the channel's resolved fallback so the
	// browser does not need to re-encode the spec to revert.
	Otherwise any `json:"otherwise,omitempty"`
}
