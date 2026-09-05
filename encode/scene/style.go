package scene

import (
	"fmt"
	"strconv"
)

// Style carries per-mark visual properties. Pointer-typed fields
// preserve the unset / explicit-zero distinction.
type Style struct {
	Fill   *Color `json:"fill,omitempty"`
	Stroke *Color `json:"stroke,omitempty"`
	// FillRef/StrokeRef carry a def id ("prism-gradient-<name>" or
	// "prism-pattern-<name>") when the theme resolved this Fill/Stroke
	// to a Theme.Gradients/Patterns entry via theme.Theme.ResolveFillRef
	// (a mark.fill/stroke value written as url(#name)) instead of a
	// literal color — see encode.applyThemeMarkStyle. When set, Fill/
	// Stroke above are left nil and the renderer emits
	// fill="url(#<FillRef>)" / stroke="url(#<StrokeRef>)" instead of
	// the literal-color CSS() attr. Empty means unset (the common
	// case): the renderer falls back to Fill/Stroke.
	FillRef     string    `json:"fill_ref,omitempty"`
	StrokeRef   string    `json:"stroke_ref,omitempty"`
	StrokeWidth float64   `json:"stroke_width,omitempty"`
	StrokeDash  []float64 `json:"stroke_dash,omitempty"`
	Opacity     float64   `json:"opacity,omitempty"`
	FontFamily  string    `json:"font_family,omitempty"`
	FontWeight  int       `json:"font_weight,omitempty"`
	Cursor      string    `json:"cursor,omitempty"`
	// LineHeight and LetterSpacing carry the resolved theme
	// MarkStyle.LineHeight / LetterSpacing typography tokens (E2-S2)
	// for text-mark glyphs. The renderer emits LetterSpacing as a
	// `letter-spacing` presentation attribute and LineHeight as a
	// `style="line-height:…"` declaration when non-nil; nil means
	// unset (no attribute emitted).
	LineHeight    *float64 `json:"line_height,omitempty"`
	LetterSpacing *float64 `json:"letter_spacing,omitempty"`
	// Filter names an entry in scene.Theme.Filters (mirrors
	// theme.MarkStyle.Filter — resolved at encode time via
	// theme.Theme.MarkDefault). The renderer emits
	// filter="url(#prism-filter-<name>)" on the mark element when
	// set. Empty means no filter.
	Filter string `json:"filter,omitempty"`
}

// Color is an 8-bit RGBA color. Gradient / pattern fills go through
// Style.FillRef/StrokeRef and Theme.Gradients/Patterns instead of a
// Color (see Style.FillRef).
type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

// ColorFromHex parses a #RRGGBB or #RRGGBBAA hex string. Missing
// alpha defaults to 255 (opaque).
func ColorFromHex(s string) (*Color, error) {
	if len(s) == 0 || s[0] != '#' {
		return nil, fmt.Errorf("color: missing leading '#' (got %q)", s)
	}
	body := s[1:]
	switch len(body) {
	case 6:
		r, err := strconv.ParseUint(body[0:2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad R component in %q: %w", s, err)
		}
		g, err := strconv.ParseUint(body[2:4], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad G component in %q: %w", s, err)
		}
		b, err := strconv.ParseUint(body[4:6], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad B component in %q: %w", s, err)
		}
		return &Color{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
	case 8:
		r, err := strconv.ParseUint(body[0:2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad R component in %q: %w", s, err)
		}
		g, err := strconv.ParseUint(body[2:4], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad G component in %q: %w", s, err)
		}
		b, err := strconv.ParseUint(body[4:6], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad B component in %q: %w", s, err)
		}
		a, err := strconv.ParseUint(body[6:8], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("color: bad A component in %q: %w", s, err)
		}
		return &Color{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
	}
	return nil, fmt.Errorf("color: expected 6 or 8 hex digits after '#', got %d in %q", len(body), s)
}

// Hex returns the color as #RRGGBB (alpha == 255) or #RRGGBBAA
// (alpha != 255). Matches the format ColorFromHex accepts.
func (c *Color) Hex() string {
	if c == nil {
		return ""
	}
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// CSS returns a CSS-renderable string for the color (`rgba(...)` when
// alpha != 255; #RRGGBB otherwise). Used by render/svg/style.go to
// emit theme + mark fills.
func (c *Color) CSS() string {
	if c == nil {
		return "transparent"
	}
	if c.A == 255 {
		return c.Hex()
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", c.R, c.G, c.B, float64(c.A)/255.0)
}
