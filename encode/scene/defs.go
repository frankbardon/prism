package scene

// Defs holds scene-level reusable resources. SVG renderer emits a
// single <defs> block; Canvas pre-builds equivalents keyed by the
// same IDs. P05 never populates these (no fixture needs gradients).
type Defs struct {
	Gradients map[string]Gradient `json:"gradients,omitempty"`
	Patterns  map[string]Pattern  `json:"patterns,omitempty"`
	Clips     map[string]Rect     `json:"clips,omitempty"`
	Filters   map[string]Filter   `json:"filters,omitempty"`
}

// Gradient is a linear or radial color gradient. Also reused by
// scene.Theme.Gradients (E3-S3) to carry the resolved
// theme.Theme.Gradients registry through to render/svg's
// <linearGradient>/<radialGradient> emitters (writeGradientPatternDefs).
// For Type == "radial", X1/Y1 hold the center (cx/cy) and X2 holds
// the radius (r); Y2 is unused. Coordinates are fractions of the
// bounding box (SVG's default objectBoundingBox gradientUnits) —
// theme.GradientDef.sceneDef does the angle/center math.
type Gradient struct {
	Type  string         `json:"type"` // "linear" | "radial"
	Stops []GradientStop `json:"stops"`
	X1    float64        `json:"x1,omitempty"`
	Y1    float64        `json:"y1,omitempty"`
	X2    float64        `json:"x2,omitempty"`
	Y2    float64        `json:"y2,omitempty"`
}

// GradientStop is one color stop in a gradient.
type GradientStop struct {
	Offset float64 `json:"offset"`
	Color  Color   `json:"color"`
}

// Pattern is a tiled fill pattern (e.g. crosshatch for accessibility).
// This same shape is reused by scene.Theme.Patterns (E3-S3), which
// carries the resolved theme.Theme.Patterns registry through to
// render/svg's <pattern> emitters (writeGradientPatternDefs). Type is
// one of theme.PatternTypes ("diagonal-stripes", "dots",
// "cross-hatch", "grid") for a built-in catalogue entry, or ""
// (empty) for a raw-content pattern, in which case Content carries
// the verbatim SVG to place inside the <pattern> wrapper — same trust
// tier as scene.Theme.Filters/RawCSS (developer-authored, never
// sanitized). Color/Spacing/Size tune the built-in generators only;
// Content ignores them. Spacing/Size are always resolved to concrete
// (defaulted) values by theme.PatternDef.sceneDef before landing here
// — this type never carries an "unset" sentinel for either.
type Pattern struct {
	Type    string  `json:"type"`
	Size    float64 `json:"size"`
	Color   string  `json:"color,omitempty"`
	Spacing float64 `json:"spacing,omitempty"`
	Content string  `json:"content,omitempty"`
}

// Filter is a post-process effect (blur, drop-shadow).
type Filter struct {
	Type   string  `json:"type"`
	Radius float64 `json:"radius,omitempty"`
}
