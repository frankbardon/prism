package theme

import "strings"

// FillRefKind classifies the result of resolving a Fill/Stroke/
// Background string value against the SVG url(#name) paint-server
// convention.
type FillRefKind int

const (
	// FillRefNone means value was not in url(#name) form at all — it
	// resolves as a plain literal color, exactly as before this story
	// landed. Zero behavior change for existing themes.
	FillRefNone FillRefKind = iota
	// FillRefGradient means name resolved against Theme.Gradients.
	FillRefGradient
	// FillRefPattern means name resolved against Theme.Patterns.
	// Checked only when Gradients has no matching entry — Gradients
	// wins on a name collision between the two registries.
	FillRefPattern
	// FillRefUnknown means value was in url(#name) form but name is
	// registered in neither Theme.Gradients nor Theme.Patterns.
	// Theme.Validate escalates this into a fail-loud
	// PRISM_THEME_FILL_REF_UNKNOWN error (see checkFillRef).
	FillRefUnknown
)

// FillRef is the result of resolving a MarkStyle.Fill/Stroke or
// ViewStyle.Background value. Name is set for every Kind except
// FillRefNone.
//
// This is a resolution seam only (E3-S2) — nothing here emits SVG
// <linearGradient>/<radialGradient>/<pattern> markup. A later story
// (E3-S3) switches on Kind to emit fill="url(#prism-gradient-<name>)"
// / fill="url(#prism-pattern-<name>)" and the matching <defs> entry
// without needing to touch this resolution logic.
type FillRef struct {
	Kind FillRefKind
	Name string
}

// ParseFillRefName reports whether value uses the SVG paint-server
// url(#name) convention (e.g. `url(#brand_fade)`) and, if so, returns
// the referenced name. Any other value — including "", a hex color
// like "#4c78a8", or a CSS color keyword like "steelblue" — reports
// ok == false so callers keep resolving it as a literal color exactly
// as today.
func ParseFillRefName(value string) (name string, ok bool) {
	const prefix, suffix = "url(#", ")"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		return "", false
	}
	name = value[len(prefix) : len(value)-len(suffix)]
	if name == "" {
		return "", false
	}
	return name, true
}

// ResolveFillRef checks whether value uses the url(#name) convention
// and, if so, classifies name against t.Gradients (checked first)
// then t.Patterns. A value that is not in url(#name) form returns
// FillRef{Kind: FillRefNone} — the caller keeps treating it as a
// literal color exactly as before this story (zero behavior change
// for existing themes). t may be nil (any url(#name) value then
// resolves to FillRefUnknown, since there is nothing to check it
// against).
//
// This is a pure classifier — it never errors. Theme.Validate (see
// checkFillRef) is what turns FillRefUnknown into a fail-loud
// PRISM_THEME_FILL_REF_UNKNOWN AppError with block/field context.
func (t *Theme) ResolveFillRef(value string) FillRef {
	name, ok := ParseFillRefName(value)
	if !ok {
		return FillRef{Kind: FillRefNone}
	}
	var gradients map[string]GradientDef
	var patterns map[string]PatternDef
	if t != nil {
		gradients = t.Gradients
		patterns = t.Patterns
	}
	if _, found := gradients[name]; found {
		return FillRef{Kind: FillRefGradient, Name: name}
	}
	if _, found := patterns[name]; found {
		return FillRef{Kind: FillRefPattern, Name: name}
	}
	return FillRef{Kind: FillRefUnknown, Name: name}
}
