package scene

// Animation is the scene-IR projection of the spec animation block.
// Fields carry resolved defaults — the encoder never emits a partial
// Animation. SVG and PDF renderers ignore this struct; only the
// browser web component and the WASM runtime consume it.
//
// JSON shape uses omitempty on the optional fields so existing
// goldens stay byte-identical when no animation is declared.
type Animation struct {
	DurationMs int    `json:"duration_ms"`
	Easing     string `json:"easing"`
	StaggerMs  int    `json:"stagger_ms,omitempty"`
	Enter      string `json:"enter,omitempty"`
	Exit       string `json:"exit,omitempty"`
}
