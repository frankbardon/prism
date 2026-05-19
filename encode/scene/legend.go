package scene

// LegendPosition controls where a legend renders relative to its scene.
type LegendPosition string

const (
	LegendRight       LegendPosition = "right"
	LegendLeft        LegendPosition = "left"
	LegendTop         LegendPosition = "top"
	LegendBottom      LegendPosition = "bottom"
	LegendTopRight    LegendPosition = "top-right"
	LegendTopLeft     LegendPosition = "top-left"
	LegendBottomRight LegendPosition = "bottom-right"
	LegendBottomLeft  LegendPosition = "bottom-left"
)

// SwatchType discriminates the visual form of a legend entry's swatch.
type SwatchType string

const (
	SwatchSolid    SwatchType = "solid"
	SwatchGradient SwatchType = "gradient"
	SwatchSymbol   SwatchType = "symbol"
)

// Legend is the resolved legend (post-layout). P05 ships the types
// but the encoder never populates them — no fixture has more than
// one color band.
type Legend struct {
	ID         string         `json:"id"`
	Channel    Channel        `json:"channel"`
	Position   LegendPosition `json:"position"`
	Title      string         `json:"title,omitempty"`
	Entries    []LegendEntry  `json:"entries"`
	Frame      Rect           `json:"frame"`
	TitleStyle Style          `json:"title_style,omitempty"`
	LabelStyle Style          `json:"label_style,omitempty"`
}

// LegendEntry is one row in a legend.
type LegendEntry struct {
	Label  string     `json:"label"`
	Swatch SwatchSpec `json:"swatch"`
}

// SwatchSpec describes a single legend swatch.
type SwatchSpec struct {
	Type       SwatchType `json:"type"`
	Color      *Color     `json:"color,omitempty"`
	GradientID string     `json:"gradient_id,omitempty"`
	Shape      PointShape `json:"shape,omitempty"`
}
