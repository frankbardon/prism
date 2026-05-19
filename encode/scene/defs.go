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

// Gradient is a linear or radial color gradient.
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
type Pattern struct {
	Type string  `json:"type"`
	Size float64 `json:"size"`
}

// Filter is a post-process effect (blur, drop-shadow).
type Filter struct {
	Type   string  `json:"type"`
	Radius float64 `json:"radius,omitempty"`
}
