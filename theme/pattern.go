package theme

// PatternTypes lists the built-in pattern catalogue names accepted
// by PatternDef.Type. A PatternDef either names one of these (tuned
// via Color/Spacing/Size) or supplies raw SVG through Content — not
// both, not neither; see (*Theme).Validate.
var PatternTypes = []string{"diagonal-stripes", "dots", "cross-hatch", "grid"}

// IsBuiltinPatternType reports whether name is a member of
// PatternTypes.
func IsBuiltinPatternType(name string) bool {
	for _, t := range PatternTypes {
		if t == name {
			return true
		}
	}
	return false
}

// PatternDef is a named pattern fill a theme can declare under
// Theme.Patterns. Style blocks will reference an entry via
// url(#name) once Fill/Stroke/Background resolution wires it up
// (E3-S2); actual SVG <pattern> emission lands in E3-S3. This story
// is model + validation only.
//
// Type names a built-in catalogue entry (see PatternTypes), tuned by
// Color/Spacing/Size. Content is a raw SVG pattern-inner-content
// body for bespoke patterns — the same trust tier as
// Theme.Filters/Theme.RawCSS (developer-authored, never sanitized).
// Exactly one of Type or Content must be set.
type PatternDef struct {
	Type    string   `json:"type,omitempty"`
	Color   string   `json:"color,omitempty"`
	Spacing *float64 `json:"spacing,omitempty"`
	Size    *float64 `json:"size,omitempty"`
	Content string   `json:"content,omitempty"`
}

// Clone deep-copies a PatternDef.
func (p PatternDef) Clone() PatternDef {
	out := p
	out.Spacing = copyFloat(p.Spacing)
	out.Size = copyFloat(p.Size)
	return out
}
