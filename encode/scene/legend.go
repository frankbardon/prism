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

	// Direction lays entries out down a column ("vertical", the
	// default) or across a row ("horizontal"). A legend under the plot
	// wants the row: stacking six series vertically below a chart
	// steals height the data needs, while a single wrapped row costs
	// two label heights.
	Direction LegendDirection `json:"direction,omitempty"`
	// RowHeight and SymbolSize are resolved from theme tokens at
	// encode time so the renderer places rows without re-deriving
	// geometry the layout already reserved space for.
	RowHeight  float64 `json:"row_height,omitempty"`
	SymbolSize float64 `json:"symbol_size,omitempty"`
}

// LegendDirection is the entry flow direction.
type LegendDirection string

const (
	LegendVertical   LegendDirection = "vertical"
	LegendHorizontal LegendDirection = "horizontal"
)

// LegendEntry is one row in a legend.
type LegendEntry struct {
	Label  string     `json:"label"`
	Swatch SwatchSpec `json:"swatch"`
	// Full is the untruncated label, set only when Label was
	// shortened; the renderer emits it as a <title> child.
	Full string `json:"full,omitempty"`
	// X and Y offset this entry from the legend frame origin. Set for
	// horizontal legends, where entry width varies with label length
	// and even spacing would leave ragged gaps.
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

// SwatchSpec describes a single legend swatch.
type SwatchSpec struct {
	Type       SwatchType `json:"type"`
	Color      *Color     `json:"color,omitempty"`
	GradientID string     `json:"gradient_id,omitempty"`
	Shape      PointShape `json:"shape,omitempty"`
}
