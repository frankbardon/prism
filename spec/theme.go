package spec

// MarkStyle is the spec-side wire shape for a mark default block.
// Mirrors theme.MarkStyle 1:1; theme/override.go copies the fields
// through when the spec ships an inline theme override. Pointer
// fields keep the JSON merge sparse.
type MarkStyle struct {
	Fill         string    `json:"fill,omitempty"`
	Stroke       string    `json:"stroke,omitempty"`
	StrokeWidth  *float64  `json:"stroke_width,omitempty"`
	StrokeDash   []float64 `json:"stroke_dash,omitempty"`
	Opacity      *float64  `json:"opacity,omitempty"`
	FillOpacity  *float64  `json:"fill_opacity,omitempty"`
	CornerRadius *float64  `json:"corner_radius,omitempty"`
	Size         *float64  `json:"size,omitempty"`
	Shape        string    `json:"shape,omitempty"`
	FontSize     *float64  `json:"font_size,omitempty"`
	FontWeight   string    `json:"font_weight,omitempty"`
	FontStyle    string    `json:"font_style,omitempty"`
	Align        string    `json:"align,omitempty"`
	Baseline     string    `json:"baseline,omitempty"`
}

// AxisStyle mirrors theme.AxisStyle.
type AxisStyle struct {
	DomainColor     string    `json:"domain_color,omitempty"`
	DomainWidth     *float64  `json:"domain_width,omitempty"`
	TickColor       string    `json:"tick_color,omitempty"`
	TickWidth       *float64  `json:"tick_width,omitempty"`
	TickSize        *float64  `json:"tick_size,omitempty"`
	TickOpacity     *float64  `json:"tick_opacity,omitempty"`
	GridColor       string    `json:"grid_color,omitempty"`
	GridWidth       *float64  `json:"grid_width,omitempty"`
	GridDash        []float64 `json:"grid_dash,omitempty"`
	GridOpacity     *float64  `json:"grid_opacity,omitempty"`
	LabelColor      string    `json:"label_color,omitempty"`
	LabelFontSize   *float64  `json:"label_font_size,omitempty"`
	LabelFontWeight string    `json:"label_font_weight,omitempty"`
	LabelPadding    *float64  `json:"label_padding,omitempty"`
	TitleColor      string    `json:"title_color,omitempty"`
	TitleFontSize   *float64  `json:"title_font_size,omitempty"`
	TitleFontWeight string    `json:"title_font_weight,omitempty"`
	TitlePadding    *float64  `json:"title_padding,omitempty"`
}

// LegendStyle mirrors theme.LegendStyle.
type LegendStyle struct {
	FillColor         string   `json:"fill_color,omitempty"`
	StrokeColor       string   `json:"stroke_color,omitempty"`
	StrokeWidth       *float64 `json:"stroke_width,omitempty"`
	Padding           *float64 `json:"padding,omitempty"`
	SymbolSize        *float64 `json:"symbol_size,omitempty"`
	SymbolStrokeWidth *float64 `json:"symbol_stroke_width,omitempty"`
	LabelColor        string   `json:"label_color,omitempty"`
	LabelFontSize     *float64 `json:"label_font_size,omitempty"`
	TitleColor        string   `json:"title_color,omitempty"`
	TitleFontSize     *float64 `json:"title_font_size,omitempty"`
	TitleFontWeight   string   `json:"title_font_weight,omitempty"`
	RowPadding        *float64 `json:"row_padding,omitempty"`
	ColumnPadding     *float64 `json:"column_padding,omitempty"`
}

// TitleStyle mirrors theme.TitleStyle.
type TitleStyle struct {
	Color      string   `json:"color,omitempty"`
	FontSize   *float64 `json:"font_size,omitempty"`
	FontWeight string   `json:"font_weight,omitempty"`
	Align      string   `json:"align,omitempty"`
	Anchor     string   `json:"anchor,omitempty"`
	Padding    *float64 `json:"padding,omitempty"`
}

// ViewStyle mirrors theme.ViewStyle.
type ViewStyle struct {
	Background   string   `json:"background,omitempty"`
	Stroke       string   `json:"stroke,omitempty"`
	StrokeWidth  *float64 `json:"stroke_width,omitempty"`
	Padding      *float64 `json:"padding,omitempty"`
	CornerRadius *float64 `json:"corner_radius,omitempty"`
}

// StateStyle mirrors theme.StateStyle.
type StateStyle struct {
	Opacity     *float64 `json:"opacity,omitempty"`
	StrokeWidth *float64 `json:"stroke_width,omitempty"`
	Stroke      string   `json:"stroke,omitempty"`
	Fill        string   `json:"fill,omitempty"`
}

// Range mirrors theme.Range — per-slot color scheme defaults.
type Range struct {
	Category  *RangeSlot `json:"category,omitempty"`
	Ordinal   *RangeSlot `json:"ordinal,omitempty"`
	Ramp      *RangeSlot `json:"ramp,omitempty"`
	Heatmap   *RangeSlot `json:"heatmap,omitempty"`
	Diverging *RangeSlot `json:"diverging,omitempty"`
	Symbol    *RangeSlot `json:"symbol,omitempty"`
	Cyclic    *RangeSlot `json:"cyclic,omitempty"`
}

// RangeSlot mirrors theme.RangeSlot.
type RangeSlot struct {
	Scheme string   `json:"scheme,omitempty"`
	Colors []string `json:"colors,omitempty"`
}
