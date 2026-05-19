package encode

import "github.com/frankbardon/prism/encode/scene"

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
