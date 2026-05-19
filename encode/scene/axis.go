package scene

// Channel is the encoding channel discriminator (matches the
// channel names in spec.Encoding).
type Channel string

const (
	ChannelX       Channel = "x"
	ChannelY       Channel = "y"
	ChannelX2      Channel = "x2"
	ChannelY2      Channel = "y2"
	ChannelColor   Channel = "color"
	ChannelSize    Channel = "size"
	ChannelShape   Channel = "shape"
	ChannelOpacity Channel = "opacity"
)

// AxisPosition controls where an axis renders relative to its plot.
type AxisPosition string

const (
	AxisPositionBottom AxisPosition = "bottom"
	AxisPositionLeft   AxisPosition = "left"
	AxisPositionTop    AxisPosition = "top"
	AxisPositionRight  AxisPosition = "right"
)

// ScaleType is the canonical scale-type discriminator.
type ScaleType string

const (
	ScaleLinear  ScaleType = "linear"
	ScaleLog     ScaleType = "log"
	ScalePow     ScaleType = "pow"
	ScaleSqrt    ScaleType = "sqrt"
	ScaleTime    ScaleType = "time"
	ScaleBand    ScaleType = "band"
	ScalePoint   ScaleType = "point"
	ScaleOrdinal ScaleType = "ordinal"
)

// Axis is the post-resolve description of one chart axis.
type Axis struct {
	ID         string       `json:"id"`
	Channel    Channel      `json:"channel"`
	Position   AxisPosition `json:"position"`
	Scale      ScaleSpec    `json:"scale"`
	Ticks      []Tick       `json:"ticks,omitempty"`
	Title      string       `json:"title,omitempty"`
	Domain     Line         `json:"domain,omitempty"`
	Grid       []Line       `json:"grid,omitempty"`
	LabelStyle Style        `json:"label_style,omitempty"`
	TitleStyle Style        `json:"title_style,omitempty"`
}

// Tick is one resolved tick mark: value + pixel + pre-formatted label.
type Tick struct {
	Value any     `json:"value"`
	Pixel float64 `json:"pixel"`
	Label string  `json:"label"`
	Minor bool    `json:"minor,omitempty"`
}

// ScaleSpec is the post-resolve scale (Type + Domain + Range + flags).
type ScaleSpec struct {
	Type    ScaleType  `json:"type"`
	Domain  []any      `json:"domain,omitempty"`
	Range   [2]float64 `json:"range"`
	Padding float64    `json:"padding,omitempty"`
	Base    float64    `json:"base,omitempty"`
	Exp     float64    `json:"exp,omitempty"`
	Nice    bool       `json:"nice,omitempty"`
	Clamp   bool       `json:"clamp,omitempty"`
}

// Line is a pre-resolved line segment, used for axis domain lines and
// grid lines (both anchored to the plot region).
type Line struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}
