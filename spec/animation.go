package spec

// Animation declares an optional client-side tween between successive
// scenes. Animation hints live in the Scene IR but are honoured only by
// the browser web component and the WASM runtime; static SVG and PDF
// renderers ignore the block entirely so their output stays terminal.
//
// At most one encoding channel may carry `key: true`; that channel's
// resolved value becomes the per-mark identity used to diff old vs
// new scenes (enter / update / exit partitioning).
type Animation struct {
	DurationMs *int   `json:"duration_ms,omitempty"`
	Easing     string `json:"easing,omitempty"`
	StaggerMs  *int   `json:"stagger_ms,omitempty"`
	Enter      string `json:"enter,omitempty"`
	Exit       string `json:"exit,omitempty"`
}

// AnimationEasings is the canonical set of accepted easing names.
// Validation rule animation_easing_known enforces membership.
var AnimationEasings = []string{
	"linear",
	"cubic_in", "cubic_out", "cubic_in_out",
	"quad_in", "quad_out", "quad_in_out",
	"sine_in", "sine_out", "sine_in_out",
	"expo_in", "expo_out", "expo_in_out",
}

// AnimationEnterExit is the set of accepted enter/exit modes.
var AnimationEnterExit = []string{"fade", "none"}

// Defaults applied at encode time. Kept here so the validator and the
// encoder agree on the empty-field semantics.
const (
	AnimationDefaultDurationMs = 400
	AnimationDefaultEasing     = "cubic_in_out"
	AnimationDefaultStaggerMs  = 0
	AnimationDefaultEnter      = "fade"
	AnimationDefaultExit       = "fade"

	AnimationMaxDurationMs = 5000
	AnimationMaxStaggerMs  = 1000
)
