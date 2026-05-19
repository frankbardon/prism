package scene

// Scene is one chart with layered marks. Coordinates in Frame/Plot
// are pre-resolved to pixel space.
type Scene struct {
	ID          string       `json:"id"`
	Frame       Rect         `json:"frame"`
	Plot        Rect         `json:"plot"`
	Title       *TextElement `json:"title,omitempty"`
	Subtitle    *TextElement `json:"subtitle,omitempty"`
	Axes        []Axis       `json:"axes,omitempty"`
	Legends     []Legend     `json:"legends,omitempty"`
	Layers      []SceneLayer `json:"layers"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Selections  []Selection  `json:"selections,omitempty"`
	Defs        *Defs        `json:"defs,omitempty"`
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
