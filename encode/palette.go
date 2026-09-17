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
//
// Callers holding a spec scale block should use
// ResolveCategoricalPaletteWithOpts, which adds the `scale.range`
// tier above scheme.
func ResolveCategoricalPalette(t *theme.Theme, scaleScheme string) []*scene.Color {
	return ResolveCategoricalPaletteWithOpts(t, ScaleOpts{Scheme: scaleScheme})
}

// ResolveCategoricalPaletteWithOpts is ResolveCategoricalPalette with
// the channel's whole scale block folded in. The full cascade:
//
//  0. Explicit scale.range — an inline list of colors (highest
//     precedence; supplied by the author, so nothing overrides it).
//  1. Explicit scale.scheme on the channel.
//  2. theme.Range.Category slot.
//  3. theme.ColorSchemeCategorical (legacy flat field).
//  4. DefaultPalette() — final hardcoded fallback.
//
// scale.interpolate plays no part here: a categorical palette is
// indexed positionally, never traversed.
func ResolveCategoricalPaletteWithOpts(t *theme.Theme, opts ScaleOpts) []*scene.Color {
	if pal := explicitRangePalette(opts.Range); pal != nil {
		return pal
	}
	if scaleScheme := opts.Scheme; scaleScheme != "" {
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
//
// Callers holding a spec scale block should use
// ResolveSequentialPaletteWithOpts, which adds the `scale.range` tier
// and honours `scale.interpolate`.
func ResolveSequentialPalette(t *theme.Theme, scaleScheme string) []*scene.Color {
	return ResolveSequentialPaletteWithOpts(t, ScaleOpts{Scheme: scaleScheme})
}

// ResolveSequentialPaletteWithOpts resolves the continuous ramp with
// the channel's whole scale block folded in. The cascade mirrors
// ResolveCategoricalPaletteWithOpts (range → scheme → theme Ramp /
// Heatmap slot → legacy flat field → Blues).
//
// The resolved ramp is then resampled in `scale.interpolate`'s
// colorspace (see ResampleRamp). The stops handed back are plain
// sRGB, so every downstream consumer — the heatmap encoder's fill
// lerp, the Scene IR's gradient stops, the vendored JS renderer —
// blends adjacent stops linearly and still lands on the requested
// space's curve. `rgb` (the default) returns the ramp untouched.
func ResolveSequentialPaletteWithOpts(t *theme.Theme, opts ScaleOpts) []*scene.Color {
	return ResampleRamp(
		resolveSequentialStops(t, opts),
		opts.Interpolate,
		InterpolatedRampStops,
	)
}

func resolveSequentialStops(t *theme.Theme, opts ScaleOpts) []*scene.Color {
	if pal := explicitRangePalette(opts.Range); pal != nil {
		return pal
	}
	if scaleScheme := opts.Scheme; scaleScheme != "" {
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

// explicitRangePalette turns an inline `scale.range` color list into
// palette entries, or returns nil when the channel declares none.
//
// Unlike the scheme tiers below it, this one does not degrade: the
// author named these colors, so an entry Prism cannot parse is
// dropped and the rest still win the cascade. Only a range whose
// every entry is unparseable falls through to the next tier.
func explicitRangePalette(rng []string) []*scene.Color {
	if len(rng) == 0 {
		return nil
	}
	out := make([]*scene.Color, 0, len(rng))
	for _, h := range rng {
		c, err := scene.ColorFromHex(h)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GradientStopsFromPalette spreads a resolved ramp evenly over
// offsets 0..1 as Scene IR gradient stops. Feed it the output of
// ResolveSequentialPaletteWithOpts and the stops already carry the
// channel's interpolation space baked in, which is what lets the
// renderers blend neighbours in plain sRGB.
func GradientStopsFromPalette(palette []*scene.Color) []scene.GradientStop {
	if len(palette) == 0 {
		return nil
	}
	if len(palette) == 1 {
		return []scene.GradientStop{{Offset: 0, Color: *palette[0]}}
	}
	out := make([]scene.GradientStop, len(palette))
	for i, c := range palette {
		out[i] = scene.GradientStop{
			Offset: float64(i) / float64(len(palette)-1),
			Color:  *c,
		}
	}
	return out
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
