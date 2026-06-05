package theme

// Range maps semantic role → default color scheme. Slots mirror
// Vega-Lite's config.range.{category, ordinal, ramp, heatmap,
// diverging, symbol, cyclic}. Each slot is a RangeSlot pointer so
// theme JSON can declare any subset.
//
// A slot resolves through two paths: (a) explicit Colors override
// the registered scheme; (b) Scheme references a named scheme via
// theme.SchemeByName + the built-in catalogue. Colors wins.
type Range struct {
	Category  *RangeSlot `json:"category,omitempty"`
	Ordinal   *RangeSlot `json:"ordinal,omitempty"`
	Ramp      *RangeSlot `json:"ramp,omitempty"`
	Heatmap   *RangeSlot `json:"heatmap,omitempty"`
	Diverging *RangeSlot `json:"diverging,omitempty"`
	Symbol    *RangeSlot `json:"symbol,omitempty"`
	Cyclic    *RangeSlot `json:"cyclic,omitempty"`
}

// RangeSlot is either a named scheme reference or an inline color
// list. Both nil = inherit; both set = Colors wins.
type RangeSlot struct {
	Scheme string   `json:"scheme,omitempty"`
	Colors []string `json:"colors,omitempty"`
}

// Clone deep-copies a Range.
func (r *Range) Clone() *Range {
	if r == nil {
		return nil
	}
	out := *r
	out.Category = r.Category.Clone()
	out.Ordinal = r.Ordinal.Clone()
	out.Ramp = r.Ramp.Clone()
	out.Heatmap = r.Heatmap.Clone()
	out.Diverging = r.Diverging.Clone()
	out.Symbol = r.Symbol.Clone()
	out.Cyclic = r.Cyclic.Clone()
	return &out
}

// Clone deep-copies a slot.
func (s *RangeSlot) Clone() *RangeSlot {
	if s == nil {
		return nil
	}
	out := *s
	if s.Colors != nil {
		out.Colors = append([]string(nil), s.Colors...)
	}
	return &out
}

// Resolve returns the concrete color list for the slot. Colors
// short-circuits; Scheme falls through to the registry. Returns nil
// when the slot is unset or references an unknown scheme.
func (s *RangeSlot) Resolve(t *Theme) []string {
	if s == nil {
		return nil
	}
	if len(s.Colors) > 0 {
		return append([]string(nil), s.Colors...)
	}
	if s.Scheme == "" {
		return nil
	}
	if t != nil {
		if colors, ok := t.Schemes[s.Scheme]; ok && len(colors) > 0 {
			return append([]string(nil), colors...)
		}
	}
	if colors, ok := SchemeByName(s.Scheme); ok {
		return append([]string(nil), colors...)
	}
	return nil
}

// MergeRange folds override over base. Either may be nil.
func MergeRange(base, override *Range) *Range {
	if base == nil && override == nil {
		return nil
	}
	out := base.Clone()
	if out == nil {
		out = &Range{}
	}
	if override == nil {
		return out
	}
	if override.Category != nil {
		out.Category = override.Category.Clone()
	}
	if override.Ordinal != nil {
		out.Ordinal = override.Ordinal.Clone()
	}
	if override.Ramp != nil {
		out.Ramp = override.Ramp.Clone()
	}
	if override.Heatmap != nil {
		out.Heatmap = override.Heatmap.Clone()
	}
	if override.Diverging != nil {
		out.Diverging = override.Diverging.Clone()
	}
	if override.Symbol != nil {
		out.Symbol = override.Symbol.Clone()
	}
	if override.Cyclic != nil {
		out.Cyclic = override.Cyclic.Clone()
	}
	return out
}
