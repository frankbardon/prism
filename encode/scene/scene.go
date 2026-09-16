package scene

// Scene is one chart with layered marks. Coordinates in Frame/Plot
// are pre-resolved to pixel space.
type Scene struct {
	ID       string       `json:"id"`
	Frame    Rect         `json:"frame"`
	Plot     Rect         `json:"plot"`
	Title    *TextElement `json:"title,omitempty"`
	Subtitle *TextElement `json:"subtitle,omitempty"`
	Axes     []Axis       `json:"axes,omitempty"`
	Legends  []Legend     `json:"legends,omitempty"`
	Layers   []SceneLayer `json:"layers"`
	// Table (E1) carries the resolved row/column IR for a table mark.
	// nil for every other mark type. It hangs off Scene rather than
	// living inside Layers/Mark because a table has no positioned
	// geometry — Layers still carries a single zero-Marks SceneLayer
	// tagged MarkTable so render/svg's checkMarkSupport guard keeps
	// firing (see encode/scene/table.go).
	Table *Table `json:"table,omitempty"`
	// Custom (E2) carries the resolved renderer name / rows / box for
	// a custom mark. nil for every other mark type. Like Table, it
	// hangs off Scene rather than living inside Layers/Mark because a
	// custom mark has no positioned geometry of its own — Layers
	// still carries a single zero-Marks SceneLayer tagged MarkCustom
	// (see encode/scene/custom.go).
	Custom      *Custom      `json:"custom,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Selections  []Selection  `json:"selections,omitempty"`
	Defs        *Defs        `json:"defs,omitempty"`
	Animation   *Animation   `json:"animation,omitempty"`
	// ClipRef (E2-S2) names a Defs.Clips entry bounding the mark
	// container to Plot. Empty means the marks are unclipped, which is
	// the default for every scene whose position scales derive their
	// domain from the data — nothing can fall outside the plot rect
	// then. The encoder arms it when an author pins `scale.domain` (or
	// asks for it outright with `mark_def.clip`), so an out-of-domain
	// row overflows and is cut at the plot edge instead of drawing over
	// the axes, the legends or the title. Renderers apply it to the
	// mark container ONLY: axes, legends and the title are siblings of
	// that container and must stay unclipped.
	ClipRef string `json:"clip_ref,omitempty"`
}

// Rect is a pixel-resolved bounding box.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Right returns the X coordinate of the right edge.
func (r Rect) Right() float64 { return r.X + r.W }

// Bottom returns the Y coordinate of the bottom edge.
func (r Rect) Bottom() float64 { return r.Y + r.H }

// CenterX returns the horizontal center.
func (r Rect) CenterX() float64 { return r.X + r.W/2 }

// CenterY returns the vertical center.
func (r Rect) CenterY() float64 { return r.Y + r.H/2 }

// TextElement carries one text placement (title, subtitle, etc).
type TextElement struct {
	Content string  `json:"content"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Style   Style   `json:"style,omitempty"`
}
