package theme

import "github.com/frankbardon/prism/spec"

// ApplyOverride folds a spec-level ThemeOverride into a base theme.
// Maps spec.ThemeOverride fields to the corresponding Theme fields;
// spec-side fields without a Theme counterpart (Mark, Axis, Legend,
// Scale, Title maps) are accepted but not yet plumbed — they land
// when per-mark style overrides ship in a later phase.
//
// Returns a fresh Theme; base is not mutated.
func ApplyOverride(base *Theme, o *spec.ThemeOverride) *Theme {
	if o == nil {
		return base.Clone()
	}
	override := &Theme{}
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
	return Merge(base, override)
}
