package theme

import "github.com/frankbardon/prism/encode/scene"

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
// Theme.Patterns. Style blocks reference an entry via url(#name)
// (theme.Theme.ResolveFillRef, E3-S2); render/svg emits the actual
// <pattern> def (E3-S3, see sceneDef + render/svg/style.go's
// writeGradientPatternDefs).
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

// Default spacing/size (in user-space pixels, since built-in pattern
// tiles use patternUnits="userSpaceOnUse") applied when a PatternDef
// leaves either unset. Chosen to render a legible, moderate-density
// tile at typical chart scales.
const (
	defaultPatternSpacing = 8.0
	defaultPatternSize    = 4.0
	defaultPatternColor   = "#000000"
)

// sceneDef converts p into the scene-level Pattern shape consumed by
// render/svg's <pattern> emitters (E3-S3): resolves Spacing/Size to
// concrete defaults (Theme.Validate already ensures either is
// positive when explicitly set) and defaults Color for built-in
// catalogue types so a theme author who only sets "type" still gets a
// visible, deterministic pattern. Content (raw-content patterns,
// Type == "") passes through verbatim and ignores Color/Spacing/Size
// defaulting for Color specifically — a raw pattern supplies its own
// colors.
func (p PatternDef) sceneDef() scene.Pattern {
	spacing := defaultPatternSpacing
	if p.Spacing != nil {
		spacing = *p.Spacing
	}
	size := defaultPatternSize
	if p.Size != nil {
		size = *p.Size
	}
	color := p.Color
	if color == "" && p.Type != "" {
		color = defaultPatternColor
	}
	return scene.Pattern{
		Type:    p.Type,
		Color:   color,
		Spacing: spacing,
		Size:    size,
		Content: p.Content,
	}
}
