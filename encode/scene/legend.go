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

// LegendDirection is the flow direction of a legend's entries:
// vertical stacks them in a column (the default, and the geometry
// every legend drew before E3-S4), horizontal lays them out in a row.
// An empty value reads as vertical.
type LegendDirection string

const (
	LegendVertical   LegendDirection = "vertical"
	LegendHorizontal LegendDirection = "horizontal"
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
	Padding float64 `json:"padding,omitempty"`
	// Direction is legend.direction: which way the entries flow.
	// Empty — the default — reads as LegendVertical and reproduces the
	// pre-E3-S4 column geometry, which is why it stays omitempty.
	Direction  LegendDirection `json:"direction,omitempty"`
	TitleStyle Style           `json:"title_style,omitempty"`
	LabelStyle Style           `json:"label_style,omitempty"`
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
//
// Shape names the point-mark symbol a SwatchSymbol swatch is drawn
// with (legend.symbol_type). It is the same PointShape vocabulary a
// point mark uses, and render/svg draws it through the same emitter,
// so a point's diamond and a legend swatch's diamond cannot diverge.
// It is empty on a SwatchSolid / SwatchGradient swatch.
//
// Size is legend.symbol_size: the swatch's pixel extent — the side of
// a solid swatch's square, or the diameter of a symbol's bounding
// box. Zero leaves the renderer's own defaults (12 px solid, 10 px
// symbol), which is what keeps every committed legend golden
// byte-identical.
type SwatchSpec struct {
	Type       SwatchType `json:"type"`
	Color      *Color     `json:"color,omitempty"`
	GradientID string     `json:"gradient_id,omitempty"`
	Shape      PointShape `json:"shape,omitempty"`
	Size       float64    `json:"size,omitempty"`
}
