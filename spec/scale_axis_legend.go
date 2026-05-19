package spec

// Scale models the scale block on a channel.
type Scale struct {
	Type          string `json:"type,omitempty"`
	Domain        any    `json:"domain,omitempty"`
	Range         any    `json:"range,omitempty"`
	Scheme        string `json:"scheme,omitempty"`
	Padding       *float64 `json:"padding,omitempty"`
	PaddingInner  *float64 `json:"padding_inner,omitempty"`
	PaddingOuter  *float64 `json:"padding_outer,omitempty"`
	Align         *float64 `json:"align,omitempty"`
	Base          *float64 `json:"base,omitempty"`
	Exponent      *float64 `json:"exponent,omitempty"`
	Nice          any    `json:"nice,omitempty"`
	Clamp         *bool  `json:"clamp,omitempty"`
	Zero          *bool  `json:"zero,omitempty"`
	Reverse       *bool  `json:"reverse,omitempty"`
	Round         *bool  `json:"round,omitempty"`
	Interpolate   string `json:"interpolate,omitempty"`
}

// Axis models the axis block on a position channel.
type Axis struct {
	Orient        string  `json:"orient,omitempty"`
	Title         any     `json:"title,omitempty"`
	Format        string  `json:"format,omitempty"`
	TickCount     *int    `json:"tick_count,omitempty"`
	TickMinStep   *float64 `json:"tick_min_step,omitempty"`
	TickSize      *float64 `json:"tick_size,omitempty"`
	Values        []any   `json:"values,omitempty"`
	Grid          *bool   `json:"grid,omitempty"`
	Labels        *bool   `json:"labels,omitempty"`
	LabelAngle    *float64 `json:"label_angle,omitempty"`
	LabelOverlap  any     `json:"label_overlap,omitempty"`
	LabelPadding  *float64 `json:"label_padding,omitempty"`
	LabelLimit    *float64 `json:"label_limit,omitempty"`
	Domain        *bool   `json:"domain,omitempty"`
	Ticks         *bool   `json:"ticks,omitempty"`
	Zindex        *int    `json:"zindex,omitempty"`
}

// Legend models the legend block on a mark channel.
type Legend struct {
	Type        string  `json:"type,omitempty"`
	Orient      string  `json:"orient,omitempty"`
	Title       any     `json:"title,omitempty"`
	Direction   string  `json:"direction,omitempty"`
	Format      string  `json:"format,omitempty"`
	TickCount   *int    `json:"tick_count,omitempty"`
	Values      []any   `json:"values,omitempty"`
	SymbolType  string  `json:"symbol_type,omitempty"`
	SymbolSize  *float64 `json:"symbol_size,omitempty"`
	Padding     *float64 `json:"padding,omitempty"`
	Offset      *float64 `json:"offset,omitempty"`
	LabelLimit  *float64 `json:"label_limit,omitempty"`
}
