package theme

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile reads a theme JSON from disk. When the file's `base`
// field is set to a registered theme name, the file's other fields
// merge sparsely on top of that base. Standalone (base == "") returns
// the theme as declared.
func LoadFile(path string) (*Theme, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("theme.LoadFile %s: %w", path, err)
	}
	return LoadBytes(body)
}

// LoadBytes parses a theme JSON blob. Same merge semantics as
// LoadFile.
func LoadBytes(body []byte) (*Theme, error) {
	var t Theme
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("theme.LoadBytes: %w", err)
	}
	if t.Base == "" {
		return &t, nil
	}
	base, ok := Get(t.Base)
	if !ok {
		return nil, fmt.Errorf("theme.LoadBytes: base theme %q is not registered", t.Base)
	}
	return Merge(base, &t), nil
}

// Merge returns a new Theme that combines base with the non-zero
// fields of override. String/list zero values inherit from base.
func Merge(base, override *Theme) *Theme {
	if base == nil {
		return override.Clone()
	}
	out := base.Clone()
	if override == nil {
		return out
	}
	if override.Name != "" {
		out.Name = override.Name
	}
	if override.AxisColor != "" {
		out.AxisColor = override.AxisColor
	}
	if override.GridColor != "" {
		out.GridColor = override.GridColor
	}
	if override.TextColor != "" {
		out.TextColor = override.TextColor
	}
	if override.BackgroundColor != "" {
		out.BackgroundColor = override.BackgroundColor
	}
	if override.FontSans != "" {
		out.FontSans = override.FontSans
	}
	if override.FontMono != "" {
		out.FontMono = override.FontMono
	}
	if override.FontSizeLabel != 0 {
		out.FontSizeLabel = override.FontSizeLabel
	}
	if override.FontSizeTitle != 0 {
		out.FontSizeTitle = override.FontSizeTitle
	}
	if override.FontSizeAxisTitle != 0 {
		out.FontSizeAxisTitle = override.FontSizeAxisTitle
	}
	if len(override.ColorSchemeCategorical) > 0 {
		out.ColorSchemeCategorical = append([]string(nil), override.ColorSchemeCategorical...)
	}
	if len(override.ColorSchemeSequential) > 0 {
		out.ColorSchemeSequential = append([]string(nil), override.ColorSchemeSequential...)
	}
	if override.Mark != nil {
		out.Mark = MergeMarkStyle(out.Mark, override.Mark)
	}
	if override.Marks != nil {
		if out.Marks == nil {
			out.Marks = make(map[string]*MarkStyle, len(override.Marks))
		}
		for k, v := range override.Marks {
			out.Marks[k] = MergeMarkStyle(out.Marks[k], v)
		}
	}
	if override.Axis != nil {
		out.Axis = mergeAxis(out.Axis, override.Axis)
	}
	if override.Legend != nil {
		out.Legend = mergeLegend(out.Legend, override.Legend)
	}
	if override.Title != nil {
		out.Title = mergeTitle(out.Title, override.Title)
	}
	if override.View != nil {
		out.View = mergeView(out.View, override.View)
	}
	if override.Range != nil {
		out.Range = MergeRange(out.Range, override.Range)
	}
	if override.States != nil {
		if out.States == nil {
			out.States = make(map[string]*StateStyle, len(override.States))
		}
		for k, v := range override.States {
			out.States[k] = mergeState(out.States[k], v)
		}
	}
	if override.Schemes != nil {
		if out.Schemes == nil {
			out.Schemes = make(map[string][]string, len(override.Schemes))
		}
		for k, v := range override.Schemes {
			out.Schemes[k] = append([]string(nil), v...)
		}
	}
	if override.Style != nil {
		if out.Style == nil {
			out.Style = make(map[string]*MarkStyle, len(override.Style))
		}
		for k, v := range override.Style {
			out.Style[k] = MergeMarkStyle(out.Style[k], v)
		}
	}
	return out
}

func mergeAxis(base, override *AxisStyle) *AxisStyle {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		cp := *override
		if override.GridDash != nil {
			cp.GridDash = append([]float64(nil), override.GridDash...)
		}
		return &cp
	}
	out := *base
	if base.GridDash != nil {
		out.GridDash = append([]float64(nil), base.GridDash...)
	}
	if override == nil {
		return &out
	}
	if override.DomainColor != "" {
		out.DomainColor = override.DomainColor
	}
	if override.DomainWidth != nil {
		v := *override.DomainWidth
		out.DomainWidth = &v
	}
	if override.TickColor != "" {
		out.TickColor = override.TickColor
	}
	if override.TickWidth != nil {
		v := *override.TickWidth
		out.TickWidth = &v
	}
	if override.TickSize != nil {
		v := *override.TickSize
		out.TickSize = &v
	}
	if override.TickOpacity != nil {
		v := *override.TickOpacity
		out.TickOpacity = &v
	}
	if override.GridColor != "" {
		out.GridColor = override.GridColor
	}
	if override.GridWidth != nil {
		v := *override.GridWidth
		out.GridWidth = &v
	}
	if override.GridDash != nil {
		out.GridDash = append([]float64(nil), override.GridDash...)
	}
	if override.GridOpacity != nil {
		v := *override.GridOpacity
		out.GridOpacity = &v
	}
	if override.LabelColor != "" {
		out.LabelColor = override.LabelColor
	}
	if override.LabelFontSize != nil {
		v := *override.LabelFontSize
		out.LabelFontSize = &v
	}
	if override.LabelFontWeight != "" {
		out.LabelFontWeight = override.LabelFontWeight
	}
	if override.LabelPadding != nil {
		v := *override.LabelPadding
		out.LabelPadding = &v
	}
	if override.TitleColor != "" {
		out.TitleColor = override.TitleColor
	}
	if override.TitleFontSize != nil {
		v := *override.TitleFontSize
		out.TitleFontSize = &v
	}
	if override.TitleFontWeight != "" {
		out.TitleFontWeight = override.TitleFontWeight
	}
	if override.TitlePadding != nil {
		v := *override.TitlePadding
		out.TitlePadding = &v
	}
	return &out
}

func mergeLegend(base, override *LegendStyle) *LegendStyle {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		cp := *override
		return &cp
	}
	out := *base
	if override == nil {
		return &out
	}
	if override.FillColor != "" {
		out.FillColor = override.FillColor
	}
	if override.StrokeColor != "" {
		out.StrokeColor = override.StrokeColor
	}
	if override.StrokeWidth != nil {
		v := *override.StrokeWidth
		out.StrokeWidth = &v
	}
	if override.Padding != nil {
		v := *override.Padding
		out.Padding = &v
	}
	if override.SymbolSize != nil {
		v := *override.SymbolSize
		out.SymbolSize = &v
	}
	if override.SymbolStrokeWidth != nil {
		v := *override.SymbolStrokeWidth
		out.SymbolStrokeWidth = &v
	}
	if override.LabelColor != "" {
		out.LabelColor = override.LabelColor
	}
	if override.LabelFontSize != nil {
		v := *override.LabelFontSize
		out.LabelFontSize = &v
	}
	if override.TitleColor != "" {
		out.TitleColor = override.TitleColor
	}
	if override.TitleFontSize != nil {
		v := *override.TitleFontSize
		out.TitleFontSize = &v
	}
	if override.TitleFontWeight != "" {
		out.TitleFontWeight = override.TitleFontWeight
	}
	if override.RowPadding != nil {
		v := *override.RowPadding
		out.RowPadding = &v
	}
	if override.ColumnPadding != nil {
		v := *override.ColumnPadding
		out.ColumnPadding = &v
	}
	return &out
}

func mergeTitle(base, override *TitleStyle) *TitleStyle {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		cp := *override
		return &cp
	}
	out := *base
	if override == nil {
		return &out
	}
	if override.Color != "" {
		out.Color = override.Color
	}
	if override.FontSize != nil {
		v := *override.FontSize
		out.FontSize = &v
	}
	if override.FontWeight != "" {
		out.FontWeight = override.FontWeight
	}
	if override.Align != "" {
		out.Align = override.Align
	}
	if override.Anchor != "" {
		out.Anchor = override.Anchor
	}
	if override.Padding != nil {
		v := *override.Padding
		out.Padding = &v
	}
	return &out
}

func mergeView(base, override *ViewStyle) *ViewStyle {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		cp := *override
		return &cp
	}
	out := *base
	if override == nil {
		return &out
	}
	if override.Background != "" {
		out.Background = override.Background
	}
	if override.Stroke != "" {
		out.Stroke = override.Stroke
	}
	if override.StrokeWidth != nil {
		v := *override.StrokeWidth
		out.StrokeWidth = &v
	}
	if override.Padding != nil {
		v := *override.Padding
		out.Padding = &v
	}
	if override.CornerRadius != nil {
		v := *override.CornerRadius
		out.CornerRadius = &v
	}
	return &out
}

func mergeState(base, override *StateStyle) *StateStyle {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		cp := *override
		return &cp
	}
	out := *base
	if override == nil {
		return &out
	}
	if override.Opacity != nil {
		v := *override.Opacity
		out.Opacity = &v
	}
	if override.StrokeWidth != nil {
		v := *override.StrokeWidth
		out.StrokeWidth = &v
	}
	if override.Stroke != "" {
		out.Stroke = override.Stroke
	}
	if override.Fill != "" {
		out.Fill = override.Fill
	}
	return &out
}
