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
	ID       string         `json:"id"`
	Channel  Channel        `json:"channel"`
	Position LegendPosition `json:"position"`
	Title    string         `json:"title,omitempty"`
	Entries  []LegendEntry  `json:"entries"`
	Frame    Rect           `json:"frame"`
	// Padding is the interior padding (legend.padding) the renderer
	// insets the title, swatches and labels by, on top of its own
	// fixed 4-px content inset. Zero — the default — reproduces the
	// pre-E1-S3 geometry.
	Padding    float64 `json:"padding,omitempty"`
	TitleStyle Style   `json:"title_style,omitempty"`
	LabelStyle Style   `json:"label_style,omitempty"`
}

// LegendEntry is one row in a legend.
type LegendEntry struct {
	Label  string     `json:"label"`
	Swatch SwatchSpec `json:"swatch"`
	// Ticks are the labelled stops a gradient entry draws alongside
	// its bar, ordered from the domain minimum to the maximum. Empty
	// for a solid or symbol swatch, and empty for a gradient whose
	// legend.tick_count is zero — a renderer then falls back to the
	// entry's own Label.
	Ticks []LegendTick `json:"ticks,omitempty"`
}

// LegendTick is one labelled stop along a gradient legend's bar.
// Offset is the fraction of the bar's length the label sits at: 0 at
// the domain minimum, 1 at the maximum.
type LegendTick struct {
	Offset float64 `json:"offset"`
	Label  string  `json:"label"`
}

// SwatchSpec describes a single legend swatch.
type SwatchSpec struct {
	Type       SwatchType `json:"type"`
	Color      *Color     `json:"color,omitempty"`
	GradientID string     `json:"gradient_id,omitempty"`
	Shape      PointShape `json:"shape,omitempty"`
}
