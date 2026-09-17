package scene

// Channel is the encoding channel discriminator (matches the
// channel names in spec.Encoding).
type Channel string

const (
	ChannelX  Channel = "x"
	ChannelY  Channel = "y"
	ChannelX2 Channel = "x2"
	ChannelY2 Channel = "y2"
	// ChannelXOffset / ChannelYOffset name the offset (dodge)
	// sub-band scale a position channel's band slot is divided by.
	// They carry no axis and no legend; they exist so cross-child
	// scale resolution can address the offset scale by name the way
	// `resolve.scale.x_offset` does on the wire.
	ChannelXOffset Channel = "x_offset"
	ChannelYOffset Channel = "y_offset"
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
	LabelAngle float64      `json:"label_angle,omitempty"`
	LabelStyle Style        `json:"label_style,omitempty"`
	TitleStyle Style        `json:"title_style,omitempty"`
	// HideLabels / HideTicks / HideDomain carry the per-component
	// suppression of `axis.labels`, `axis.ticks` and `axis.domain`
	// (E3-S2). They are stated as *negations* so an absent key keeps
	// the component drawn, which is both the Vega-Lite default and
	// what makes the field omitempty-invisible to existing consumers.
	//
	// The ticks themselves stay in Ticks even when HideTicks is set —
	// the labels and the grid lines are derived from the same list, so
	// only the tick *marks* are suppressed. Likewise Domain keeps its
	// geometry when HideDomain is set; the flag is the instruction,
	// not the absence of coordinates.
	//
	// Whole-axis suppression (`"axis": null`, E1-S4) is a different
	// mechanism: the encoder never emits a scene.Axis at all, and the
	// side releases its layout padding.
	HideLabels bool `json:"hide_labels,omitempty"`
	HideTicks  bool `json:"hide_ticks,omitempty"`
	HideDomain bool `json:"hide_domain,omitempty"`
	// TickSize / LabelPadding carry the spec-level `axis.tick_size` and
	// `axis.label_padding` overrides (E3-S2), in pixels. Nil means the
	// spec said nothing, and the renderer falls back to the theme
	// tokens (scene.Theme.AxisTickSize / AxisLabelPadding) and then to
	// its built-in metrics. Spec wins over theme by construction: a
	// non-nil value here is never reconciled against the theme.
	TickSize     *float64 `json:"tick_size,omitempty"`
	LabelPadding *float64 `json:"label_padding,omitempty"`
	// TitlePadding carries the spec-level `axis.title_padding`
	// override (E8-S2), in pixels: the gap between the axis's tick
	// labels and its title. Nil means the spec said nothing, and the
	// renderer falls back to the theme token
	// (scene.Theme.AxisX/AxisY.TitlePadding, then
	// scene.Theme.AxisTitlePadding) and finally to its built-in
	// metric. Same spec-wins-by-construction rule as TickSize above.
	TitlePadding *float64 `json:"title_padding,omitempty"`
	// Zindex is the axis stacking order relative to the marks (E3-S2):
	// 0 (the default) draws the axis and its grid lines behind the
	// marks, any positive value draws them in front. The above-marks
	// group is a sibling of the mark container rather than a child, so
	// an above-marks axis is never subject to the plot-region clip
	// path applied to the marks.
	Zindex int `json:"zindex,omitempty"`
}

// Tick is one resolved tick mark: value + pixel + pre-formatted label.
type Tick struct {
	Value       any     `json:"value"`
	Pixel       float64 `json:"pixel"`
	Label       string  `json:"label"`
	Minor       bool    `json:"minor,omitempty"`
	LabelHidden bool    `json:"label_hidden,omitempty"`
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
