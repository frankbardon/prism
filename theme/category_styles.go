package theme

// cloneCategoryStyles deep-copies the nested field→value→style map so
// sparse-override merges (and callers holding onto a Theme) never
// alias into another Theme's map. Leaf values clone via
// MarkStyle.Clone(), mirroring how Marks/Style clone their leaves in
// Theme.Clone.
func cloneCategoryStyles(m map[string]map[string]*MarkStyle) map[string]map[string]*MarkStyle {
	if m == nil {
		return nil
	}
	out := make(map[string]map[string]*MarkStyle, len(m))
	for field, values := range m {
		if values == nil {
			out[field] = nil
			continue
		}
		inner := make(map[string]*MarkStyle, len(values))
		for value, style := range values {
			inner[value] = style.Clone()
		}
		out[field] = inner
	}
	return out
}

// mergeCategoryStyles folds override over base for the nested
// field→value→style map. Unlike the wholesale per-key replacement
// Gradients/Patterns use, each (field, value) leaf merges through
// MergeMarkStyle so override wins per MarkStyle field while base
// fields not touched by override survive — the same fine-grained
// cascade MergeMarkStyle already gives Mark/Marks/Style. Field/value
// pairs present only in base (not touched by override) are preserved
// unchanged; pairs present only in override are added.
func mergeCategoryStyles(base, override map[string]map[string]*MarkStyle) map[string]map[string]*MarkStyle {
	if base == nil && override == nil {
		return nil
	}
	out := cloneCategoryStyles(base)
	if out == nil {
		out = make(map[string]map[string]*MarkStyle, len(override))
	}
	for field, values := range override {
		inner := out[field]
		if inner == nil {
			inner = make(map[string]*MarkStyle, len(values))
			out[field] = inner
		}
		for value, style := range values {
			inner[value] = MergeMarkStyle(inner[value], style)
		}
	}
	return out
}
