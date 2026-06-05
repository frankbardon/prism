package theme

import "github.com/frankbardon/prism/spec"

// ApplyOverride folds a spec-level ThemeOverride into a base theme.
// Translates each spec field to its theme.Theme counterpart, then
// runs through Merge so the cascade matches a JSON-loaded theme.
// Returns a fresh Theme; base is not mutated.
func ApplyOverride(base *Theme, o *spec.ThemeOverride) *Theme {
	if o == nil {
		return base.Clone()
	}
	override := &Theme{}
	// Legacy flat fields seed the equivalent flat theme fields so
	// pre-v2 specs keep working.
	if o.Background != "" {
		override.BackgroundColor = o.Background
	}
	if o.Font != "" {
		override.FontSans = o.Font
	}
	if o.FontSize != 0 {
		override.FontSizeLabel = o.FontSize
	}
	if o.Color != "" {
		override.TextColor = o.Color
	}
	if len(o.Palette) > 0 {
		override.ColorSchemeCategorical = append([]string(nil), o.Palette...)
	}
	if o.Scheme != "" {
		// `scheme` at top level seeds the categorical range slot.
		override.Range = &Range{Category: &RangeSlot{Scheme: o.Scheme}}
	}
	// v2 nested blocks copy via field-by-field translation.
	if o.Mark != nil {
		override.Mark = copyMarkStyle(o.Mark)
	}
	if o.Marks != nil {
		override.Marks = make(map[string]*MarkStyle, len(o.Marks))
		for k, v := range o.Marks {
			override.Marks[k] = copyMarkStyle(v)
		}
	}
	if o.Axis != nil {
		override.Axis = copyAxisStyle(o.Axis)
	}
	if o.Legend != nil {
		override.Legend = copyLegendStyle(o.Legend)
	}
	if o.Title != nil {
		override.Title = copyTitleStyle(o.Title)
	}
	if o.View != nil {
		override.View = copyViewStyle(o.View)
	}
	if o.Range != nil {
		override.Range = copyRange(o.Range)
	}
	if o.States != nil {
		override.States = make(map[string]*StateStyle, len(o.States))
		for k, v := range o.States {
			override.States[k] = copyStateStyle(v)
		}
	}
	if o.Schemes != nil {
		override.Schemes = make(map[string][]string, len(o.Schemes))
		for k, v := range o.Schemes {
			override.Schemes[k] = append([]string(nil), v...)
		}
	}
	if o.Style != nil {
		override.Style = make(map[string]*MarkStyle, len(o.Style))
		for k, v := range o.Style {
			override.Style[k] = copyMarkStyle(v)
		}
	}
	return Merge(base, override)
}

func copyMarkStyle(s *spec.MarkStyle) *MarkStyle {
	if s == nil {
		return nil
	}
	out := &MarkStyle{
		Fill:       s.Fill,
		Stroke:     s.Stroke,
		Shape:      s.Shape,
		FontWeight: s.FontWeight,
		FontStyle:  s.FontStyle,
		Align:      s.Align,
		Baseline:   s.Baseline,
	}
	out.StrokeWidth = copyFloat(s.StrokeWidth)
	out.Opacity = copyFloat(s.Opacity)
	out.FillOpacity = copyFloat(s.FillOpacity)
	out.CornerRadius = copyFloat(s.CornerRadius)
	out.Size = copyFloat(s.Size)
	out.FontSize = copyFloat(s.FontSize)
	if s.StrokeDash != nil {
		out.StrokeDash = append([]float64(nil), s.StrokeDash...)
	}
	return out
}

func copyAxisStyle(s *spec.AxisStyle) *AxisStyle {
	if s == nil {
		return nil
	}
	out := &AxisStyle{
		DomainColor:     s.DomainColor,
		TickColor:       s.TickColor,
		GridColor:       s.GridColor,
		LabelColor:      s.LabelColor,
		LabelFontWeight: s.LabelFontWeight,
		TitleColor:      s.TitleColor,
		TitleFontWeight: s.TitleFontWeight,
	}
	out.DomainWidth = copyFloat(s.DomainWidth)
	out.TickWidth = copyFloat(s.TickWidth)
	out.TickSize = copyFloat(s.TickSize)
	out.TickOpacity = copyFloat(s.TickOpacity)
	out.GridWidth = copyFloat(s.GridWidth)
	out.GridOpacity = copyFloat(s.GridOpacity)
	out.LabelFontSize = copyFloat(s.LabelFontSize)
	out.LabelPadding = copyFloat(s.LabelPadding)
	out.TitleFontSize = copyFloat(s.TitleFontSize)
	out.TitlePadding = copyFloat(s.TitlePadding)
	if s.GridDash != nil {
		out.GridDash = append([]float64(nil), s.GridDash...)
	}
	return out
}

func copyLegendStyle(s *spec.LegendStyle) *LegendStyle {
	if s == nil {
		return nil
	}
	out := &LegendStyle{
		FillColor:       s.FillColor,
		StrokeColor:     s.StrokeColor,
		LabelColor:      s.LabelColor,
		TitleColor:      s.TitleColor,
		TitleFontWeight: s.TitleFontWeight,
	}
	out.StrokeWidth = copyFloat(s.StrokeWidth)
	out.Padding = copyFloat(s.Padding)
	out.SymbolSize = copyFloat(s.SymbolSize)
	out.SymbolStrokeWidth = copyFloat(s.SymbolStrokeWidth)
	out.LabelFontSize = copyFloat(s.LabelFontSize)
	out.TitleFontSize = copyFloat(s.TitleFontSize)
	out.RowPadding = copyFloat(s.RowPadding)
	out.ColumnPadding = copyFloat(s.ColumnPadding)
	return out
}

func copyTitleStyle(s *spec.TitleStyle) *TitleStyle {
	if s == nil {
		return nil
	}
	out := &TitleStyle{
		Color:      s.Color,
		FontWeight: s.FontWeight,
		Align:      s.Align,
		Anchor:     s.Anchor,
	}
	out.FontSize = copyFloat(s.FontSize)
	out.Padding = copyFloat(s.Padding)
	return out
}

func copyViewStyle(s *spec.ViewStyle) *ViewStyle {
	if s == nil {
		return nil
	}
	out := &ViewStyle{
		Background: s.Background,
		Stroke:     s.Stroke,
	}
	out.StrokeWidth = copyFloat(s.StrokeWidth)
	out.Padding = copyFloat(s.Padding)
	out.CornerRadius = copyFloat(s.CornerRadius)
	return out
}

func copyStateStyle(s *spec.StateStyle) *StateStyle {
	if s == nil {
		return nil
	}
	out := &StateStyle{Stroke: s.Stroke, Fill: s.Fill}
	out.Opacity = copyFloat(s.Opacity)
	out.StrokeWidth = copyFloat(s.StrokeWidth)
	return out
}

func copyRange(r *spec.Range) *Range {
	if r == nil {
		return nil
	}
	return &Range{
		Category:  copyRangeSlot(r.Category),
		Ordinal:   copyRangeSlot(r.Ordinal),
		Ramp:      copyRangeSlot(r.Ramp),
		Heatmap:   copyRangeSlot(r.Heatmap),
		Diverging: copyRangeSlot(r.Diverging),
		Symbol:    copyRangeSlot(r.Symbol),
		Cyclic:    copyRangeSlot(r.Cyclic),
	}
}

func copyRangeSlot(s *spec.RangeSlot) *RangeSlot {
	if s == nil {
		return nil
	}
	out := &RangeSlot{Scheme: s.Scheme}
	if s.Colors != nil {
		out.Colors = append([]string(nil), s.Colors...)
	}
	return out
}

func copyFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}
