package scene

// TooltipPosition controls where the tooltip renders relative to the mark.
type TooltipPosition string

const (
	TooltipAuto   TooltipPosition = "auto"
	TooltipTop    TooltipPosition = "top"
	TooltipRight  TooltipPosition = "right"
	TooltipBottom TooltipPosition = "bottom"
	TooltipLeft   TooltipPosition = "left"
)

// Tooltip is the pre-formatted, layout-ready tooltip for one mark.
type Tooltip struct {
	Lines    []TooltipLine   `json:"lines"`
	Position TooltipPosition `json:"position,omitempty"`
}

// TooltipLine is one row of tooltip text.
type TooltipLine struct {
	Label string `json:"label"`
	Style Style  `json:"style,omitempty"`
}
