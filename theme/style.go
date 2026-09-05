package theme

// MarkStyle is the default visual style for a mark. Fields use
// pointer types so absent fields inherit through the cascade
// (theme.Mark → theme.Marks[type] → spec.MarkDef → encoding). A
// non-pointer field would force every level to declare every key.
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
	// LineHeight and LetterSpacing are text typography tokens for the
	// text mark. Unset (nil) inherits the renderer's existing default
	// (single-line, no extra tracking). Model + schema only in this
	// story (E2-S1); rendering lands in E2-S2.
	LineHeight    *float64 `json:"line_height,omitempty"`
	LetterSpacing *float64 `json:"letter_spacing,omitempty"`
	// Filter names an entry in Theme.Filters — a raw SVG <filter>
	// inner-content body applied to marks styled by this block. A
	// name that does not resolve fails loudly at theme load
	// (PRISM_THEME_FILTER_UNKNOWN) rather than silently no-op'ing;
	// see theme/validate.go. Rendering wires the actual <filter>
	// element / filter="" attribute in a later story (E1-S2).
	Filter string `json:"filter,omitempty"`
}

// Clone deep-copies the MarkStyle.
func (m *MarkStyle) Clone() *MarkStyle {
	if m == nil {
		return nil
	}
	out := *m
	if m.StrokeDash != nil {
		out.StrokeDash = append([]float64(nil), m.StrokeDash...)
	}
	if m.StrokeWidth != nil {
		v := *m.StrokeWidth
		out.StrokeWidth = &v
	}
	if m.Opacity != nil {
		v := *m.Opacity
		out.Opacity = &v
	}
	if m.FillOpacity != nil {
		v := *m.FillOpacity
		out.FillOpacity = &v
	}
	if m.CornerRadius != nil {
		v := *m.CornerRadius
		out.CornerRadius = &v
	}
	if m.Size != nil {
		v := *m.Size
		out.Size = &v
	}
	if m.FontSize != nil {
		v := *m.FontSize
		out.FontSize = &v
	}
	if m.LineHeight != nil {
		v := *m.LineHeight
		out.LineHeight = &v
	}
	if m.LetterSpacing != nil {
		v := *m.LetterSpacing
		out.LetterSpacing = &v
	}
	return &out
}

// MergeMarkStyle returns a fresh MarkStyle where override wins per
// field. Either side may be nil.
func MergeMarkStyle(base, override *MarkStyle) *MarkStyle {
	if base == nil && override == nil {
		return nil
	}
	out := base.Clone()
	if out == nil {
		out = &MarkStyle{}
	}
	if override == nil {
		return out
	}
	if override.Fill != "" {
		out.Fill = override.Fill
	}
	if override.Stroke != "" {
		out.Stroke = override.Stroke
	}
	if override.StrokeWidth != nil {
		v := *override.StrokeWidth
		out.StrokeWidth = &v
	}
	if override.StrokeDash != nil {
		out.StrokeDash = append([]float64(nil), override.StrokeDash...)
	}
	if override.Opacity != nil {
		v := *override.Opacity
		out.Opacity = &v
	}
	if override.FillOpacity != nil {
		v := *override.FillOpacity
		out.FillOpacity = &v
	}
	if override.CornerRadius != nil {
		v := *override.CornerRadius
		out.CornerRadius = &v
	}
	if override.Size != nil {
		v := *override.Size
		out.Size = &v
	}
	if override.Shape != "" {
		out.Shape = override.Shape
	}
	if override.FontSize != nil {
		v := *override.FontSize
		out.FontSize = &v
	}
	if override.FontWeight != "" {
		out.FontWeight = override.FontWeight
	}
	if override.FontStyle != "" {
		out.FontStyle = override.FontStyle
	}
	if override.Align != "" {
		out.Align = override.Align
	}
	if override.Baseline != "" {
		out.Baseline = override.Baseline
	}
	if override.LineHeight != nil {
		v := *override.LineHeight
		out.LineHeight = &v
	}
	if override.LetterSpacing != nil {
		v := *override.LetterSpacing
		out.LetterSpacing = &v
	}
	if override.Filter != "" {
		out.Filter = override.Filter
	}
	return out
}

// AxisStyle holds axis tokens used by encode/axis_build.go and the
// CSS variable emitter. Zero values fall back to the existing
// hard-coded defaults so existing themes keep working.
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
	// LabelLineHeight and LabelLetterSpacing are typography tokens for
	// axis tick labels. Model + schema only in this story (E2-S1);
	// rendering lands in E2-S2.
	LabelLineHeight    *float64 `json:"label_line_height,omitempty"`
	LabelLetterSpacing *float64 `json:"label_letter_spacing,omitempty"`
	TitleColor         string   `json:"title_color,omitempty"`
	TitleFontSize      *float64 `json:"title_font_size,omitempty"`
	TitleFontWeight    string   `json:"title_font_weight,omitempty"`
	TitlePadding       *float64 `json:"title_padding,omitempty"`
	// TitleLineHeight and TitleLetterSpacing are typography tokens for
	// the axis title text. See LabelLineHeight.
	TitleLineHeight    *float64 `json:"title_line_height,omitempty"`
	TitleLetterSpacing *float64 `json:"title_letter_spacing,omitempty"`
	// Filter names an entry in Theme.Filters. See MarkStyle.Filter.
	Filter string `json:"filter,omitempty"`
}

// LegendStyle holds legend tokens.
type LegendStyle struct {
	FillColor         string   `json:"fill_color,omitempty"`
	StrokeColor       string   `json:"stroke_color,omitempty"`
	StrokeWidth       *float64 `json:"stroke_width,omitempty"`
	Padding           *float64 `json:"padding,omitempty"`
	SymbolSize        *float64 `json:"symbol_size,omitempty"`
	SymbolStrokeWidth *float64 `json:"symbol_stroke_width,omitempty"`
	LabelColor        string   `json:"label_color,omitempty"`
	LabelFontSize     *float64 `json:"label_font_size,omitempty"`
	// LabelLineHeight and LabelLetterSpacing are typography tokens for
	// legend entry labels. See AxisStyle.LabelLineHeight.
	LabelLineHeight    *float64 `json:"label_line_height,omitempty"`
	LabelLetterSpacing *float64 `json:"label_letter_spacing,omitempty"`
	TitleColor         string   `json:"title_color,omitempty"`
	TitleFontSize      *float64 `json:"title_font_size,omitempty"`
	TitleFontWeight    string   `json:"title_font_weight,omitempty"`
	// TitleLineHeight and TitleLetterSpacing are typography tokens for
	// the legend title text. See AxisStyle.TitleLineHeight.
	TitleLineHeight    *float64 `json:"title_line_height,omitempty"`
	TitleLetterSpacing *float64 `json:"title_letter_spacing,omitempty"`
	RowPadding         *float64 `json:"row_padding,omitempty"`
	ColumnPadding      *float64 `json:"column_padding,omitempty"`
	// Filter names an entry in Theme.Filters. See MarkStyle.Filter.
	Filter string `json:"filter,omitempty"`
}

// TitleStyle holds title block tokens.
type TitleStyle struct {
	Color      string   `json:"color,omitempty"`
	FontSize   *float64 `json:"font_size,omitempty"`
	FontWeight string   `json:"font_weight,omitempty"`
	Align      string   `json:"align,omitempty"`
	Anchor     string   `json:"anchor,omitempty"`
	Padding    *float64 `json:"padding,omitempty"`
	// LineHeight and LetterSpacing are typography tokens for the chart
	// title text. See MarkStyle.LineHeight.
	LineHeight    *float64 `json:"line_height,omitempty"`
	LetterSpacing *float64 `json:"letter_spacing,omitempty"`
	// Filter names an entry in Theme.Filters. See MarkStyle.Filter.
	Filter string `json:"filter,omitempty"`
}

// ViewStyle holds chart-rect tokens: outer background, plot rect
// stroke, padding around the chart body.
type ViewStyle struct {
	Background   string   `json:"background,omitempty"`
	Stroke       string   `json:"stroke,omitempty"`
	StrokeWidth  *float64 `json:"stroke_width,omitempty"`
	Padding      *float64 `json:"padding,omitempty"`
	CornerRadius *float64 `json:"corner_radius,omitempty"`
	// Filter names an entry in Theme.Filters. See MarkStyle.Filter.
	Filter string `json:"filter,omitempty"`
}

// StateStyle holds per-state visual overlays (selected, deselected,
// hover). Lands as CSS-variable knobs the renderer emits; downstream
// JS bindings pick them up through .prism-selected / .prism-deselected.
type StateStyle struct {
	Opacity     *float64 `json:"opacity,omitempty"`
	StrokeWidth *float64 `json:"stroke_width,omitempty"`
	Stroke      string   `json:"stroke,omitempty"`
	Fill        string   `json:"fill,omitempty"`
}
