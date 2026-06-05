package encode

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/theme"
)

// DefaultPalette returns an 8-entry color palette derived from D3's
// category10. Used as the fallback color mapping for color-channel
// encodings until P06 lands the full theme + scheme registry. The
// returned slice is freshly allocated; callers may mutate.
func DefaultPalette() []*scene.Color {
	hex := []string{
		"#3b82f6", // blue-500
		"#ef4444", // red-500
		"#10b981", // emerald-500
		"#f59e0b", // amber-500
		"#8b5cf6", // violet-500
		"#ec4899", // pink-500
		"#14b8a6", // teal-500
		"#a855f7", // purple-500
	}
	out := make([]*scene.Color, len(hex))
	for i, h := range hex {
		c, err := scene.ColorFromHex(h)
		if err != nil {
			continue
		}
		out[i] = c
	}
	return out
}

// ResolveCategoricalPalette walks the cascade for a categorical
// color channel and returns the effective palette. Order:
//
//  1. Explicit scale.scheme on the channel (highest precedence).
//  2. theme.Range.Category slot.
//  3. theme.ColorSchemeCategorical (legacy flat field).
//  4. DefaultPalette() — final hardcoded fallback.
//
// Unknown scheme names degrade silently to the next tier so a
// malformed spec keeps rendering; the validate rule surfaces a
// PRISM_SPEC_028 diagnostic separately.
func ResolveCategoricalPalette(t *theme.Theme, scaleScheme string) []*scene.Color {
	if scaleScheme != "" {
		if hex, ok := theme.SchemeByName(scaleScheme); ok && len(hex) > 0 {
			return hexListToColors(hex)
		}
		// Per-theme custom scheme registry shadows the global catalogue.
		if t != nil {
			if hex, ok := t.Schemes[scaleScheme]; ok && len(hex) > 0 {
				return hexListToColors(hex)
			}
		}
	}
	if t != nil && t.Range != nil {
		if hex := t.Range.Category.Resolve(t); len(hex) > 0 {
			return hexListToColors(hex)
		}
	}
	if t != nil && len(t.ColorSchemeCategorical) > 0 {
		return hexListToColors(t.ColorSchemeCategorical)
	}
	return DefaultPalette()
}

// ResolveSequentialPalette is the sequential analogue. Used by
// heatmap (ramp slot), histogram color bins, and any future
// quantitative-channel encoder. Falls back to a 9-stop Blues ramp
// when nothing in the cascade matches.
func ResolveSequentialPalette(t *theme.Theme, scaleScheme string) []*scene.Color {
	if scaleScheme != "" {
		if hex, ok := theme.SchemeByName(scaleScheme); ok && len(hex) > 0 {
			return hexListToColors(hex)
		}
		if t != nil {
			if hex, ok := t.Schemes[scaleScheme]; ok && len(hex) > 0 {
				return hexListToColors(hex)
			}
		}
	}
	if t != nil && t.Range != nil {
		if hex := t.Range.Ramp.Resolve(t); len(hex) > 0 {
			return hexListToColors(hex)
		}
		if hex := t.Range.Heatmap.Resolve(t); len(hex) > 0 {
			return hexListToColors(hex)
		}
	}
	if t != nil && len(t.ColorSchemeSequential) > 0 {
		return hexListToColors(t.ColorSchemeSequential)
	}
	// Final fallback: 9-stop Blues from the global catalogue.
	if hex, ok := theme.SchemeByName("blues"); ok {
		return hexListToColors(hex)
	}
	return DefaultPalette()
}

func hexListToColors(hex []string) []*scene.Color {
	out := make([]*scene.Color, 0, len(hex))
	for _, h := range hex {
		c, err := scene.ColorFromHex(h)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return DefaultPalette()
	}
	return out
}

// CategoryToColor returns the palette entry for a category, with
// stable ordering — the i-th unique category in `categories` maps
// to palette[i % len(palette)]. Falls back to palette[0] for
// out-of-domain inputs.
func CategoryToColor(category string, categories []string, palette []*scene.Color) *scene.Color {
	if len(palette) == 0 {
		return nil
	}
	for i, c := range categories {
		if c == category {
			return palette[i%len(palette)]
		}
	}
	return palette[0]
}
