// Package theme owns the resolved-theme registry, JSON loader, and
// sparse-override engine. Three built-in themes (Light, Dark, Print)
// register at init time; user themes load via LoadFile from a JSON
// blob whose shape mirrors the Theme struct (with an optional `base`
// field for sparse overrides on top of a registered theme).
//
// scene.Theme is the wire-stable subset embedded in SceneDoc for
// JSON round-trip parity (D044); theme.Theme is the full struct
// renderers consume. Convert via ToSceneTheme / FromSceneTheme.
package theme

import "github.com/frankbardon/prism/encode/scene"

// Theme is the full resolved theme. Fields use named primitives
// (string for hex, float for font-size) so JSON merges sparsely.
type Theme struct {
	Name string `json:"name,omitempty"`
	Base string `json:"base,omitempty"` // optional registered base theme

	// Core palette.
	AxisColor       string `json:"axis_color,omitempty"`
	GridColor       string `json:"grid_color,omitempty"`
	TextColor       string `json:"text_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`

	// Typography.
	FontSans          string  `json:"font_sans,omitempty"`
	FontMono          string  `json:"font_mono,omitempty"`
	FontSizeLabel     float64 `json:"font_size_label,omitempty"`
	FontSizeTitle     float64 `json:"font_size_title,omitempty"`
	FontSizeAxisTitle float64 `json:"font_size_axis_title,omitempty"`

	// Color schemes (categorical + sequential).
	ColorSchemeCategorical []string `json:"color_scheme_categorical,omitempty"`
	ColorSchemeSequential  []string `json:"color_scheme_sequential,omitempty"`
}

// ToSceneTheme converts a Theme into the wire-stable scene.Theme
// shape. Used by the encoder to embed the resolved theme into
// SceneDoc.Theme. Renderers that need richer fields look up the full
// theme registry via Get(name).
func (t *Theme) ToSceneTheme() *scene.Theme {
	if t == nil {
		return nil
	}
	out := &scene.Theme{
		Background: t.BackgroundColor,
		FontSans:   t.FontSans,
		FontMono:   t.FontMono,
	}
	if c, err := scene.ColorFromHex(t.AxisColor); err == nil {
		out.ColorAxis = c
	}
	if c, err := scene.ColorFromHex(t.GridColor); err == nil {
		out.ColorGrid = c
	}
	if c, err := scene.ColorFromHex(t.TextColor); err == nil {
		out.ColorText = c
	}
	return out
}

// Clone returns a deep copy of the theme; lists are duplicated so
// sparse-override merges do not aliasing-leak.
func (t *Theme) Clone() *Theme {
	if t == nil {
		return nil
	}
	out := *t
	if t.ColorSchemeCategorical != nil {
		out.ColorSchemeCategorical = append([]string(nil), t.ColorSchemeCategorical...)
	}
	if t.ColorSchemeSequential != nil {
		out.ColorSchemeSequential = append([]string(nil), t.ColorSchemeSequential...)
	}
	return &out
}
