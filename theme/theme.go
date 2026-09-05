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
//
// The legacy flat fields (AxisColor, GridColor, FontSans, …) stay
// for back-compat with theme JSON authored against the pre-v2
// shape. New tokens land under the nested blocks (Mark, Marks,
// Axis, Legend, Title, View, Range, Schemes, Style, States); the
// flat fields seed the nested blocks at registration time when the
// nested fields are absent (see flattenLegacy below).
type Theme struct {
	Name string `json:"name,omitempty"`
	Base string `json:"base,omitempty"` // optional registered base theme

	// Legacy flat palette (pre-v2).
	AxisColor       string `json:"axis_color,omitempty"`
	GridColor       string `json:"grid_color,omitempty"`
	TextColor       string `json:"text_color,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`

	// Legacy typography (pre-v2).
	FontSans          string  `json:"font_sans,omitempty"`
	FontMono          string  `json:"font_mono,omitempty"`
	FontSizeLabel     float64 `json:"font_size_label,omitempty"`
	FontSizeTitle     float64 `json:"font_size_title,omitempty"`
	FontSizeAxisTitle float64 `json:"font_size_axis_title,omitempty"`

	// Legacy color schemes (pre-v2).
	ColorSchemeCategorical []string `json:"color_scheme_categorical,omitempty"`
	ColorSchemeSequential  []string `json:"color_scheme_sequential,omitempty"`

	// v2 nested blocks. Each block is a pointer so JSON merges
	// remain sparse.
	Mark   *MarkStyle             `json:"mark,omitempty"`
	Marks  map[string]*MarkStyle  `json:"marks,omitempty"`
	Axis   *AxisStyle             `json:"axis,omitempty"`
	Legend *LegendStyle           `json:"legend,omitempty"`
	Title  *TitleStyle            `json:"title,omitempty"`
	View   *ViewStyle             `json:"view,omitempty"`
	Range  *Range                 `json:"range,omitempty"`
	States map[string]*StateStyle `json:"states,omitempty"`
	// Schemes is a per-theme named-scheme registry. Entries here
	// shadow the global catalogue and let theme authors add custom
	// named ramps (e.g. a brand palette). Lookup order: theme.Schemes
	// first, then SchemeByName fallback.
	Schemes map[string][]string `json:"schemes,omitempty"`
	// Style is the free-form named-style registry (Vega-Lite parity).
	// Marks reference an entry via "style" attr; renderers apply the
	// MarkStyle as an additional cascade layer.
	Style map[string]*MarkStyle `json:"style,omitempty"`

	// Filters is a named registry of raw SVG <filter> inner-content
	// bodies (e.g. a feGaussianBlur/feDropShadow chain), keyed by a
	// theme-author-chosen name. Style blocks (Mark/Marks/Style entries,
	// Axis, Legend, Title, View) reference an entry via their Filter
	// field. This is an escape hatch — theme JSON is developer-
	// authored and trusted the same as spec JSON; never route
	// untrusted/attacker-influenced theme JSON through this (see
	// docs/src/concepts/themes.md). Model + validation only in this
	// story; the SVG <filter> element / filter="" attribute emission
	// lands in E1-S2.
	Filters map[string]string `json:"filters,omitempty"`
	// RawCSS is a raw CSS string appended verbatim after the
	// generated `--prism-*` variable declarations in the emitted
	// <style> block (E1-S2 wires the emission). Same trust model as
	// Filters — developer-authored, not sanitized.
	RawCSS string `json:"raw_css,omitempty"`
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
	// Filter escape hatch (E1-S2): pass the raw filter-body registry
	// through verbatim, plus the resolved per-block Filter reference
	// for the four structural style blocks that don't cascade to a
	// per-element scene.Style (Mark/Marks do, via
	// encode.applyThemeMarkStyle → scene.Style.Filter instead).
	if len(t.Filters) > 0 {
		out.Filters = make(map[string]string, len(t.Filters))
		for k, v := range t.Filters {
			out.Filters[k] = v
		}
	}
	if t.Axis != nil {
		out.AxisFilter = t.Axis.Filter
		out.AxisLabelLineHeight = copyFloat(t.Axis.LabelLineHeight)
		out.AxisLabelLetterSpacing = copyFloat(t.Axis.LabelLetterSpacing)
		out.AxisTitleLineHeight = copyFloat(t.Axis.TitleLineHeight)
		out.AxisTitleLetterSpacing = copyFloat(t.Axis.TitleLetterSpacing)
	}
	if t.Legend != nil {
		out.LegendFilter = t.Legend.Filter
		out.LegendLabelLineHeight = copyFloat(t.Legend.LabelLineHeight)
		out.LegendLabelLetterSpacing = copyFloat(t.Legend.LabelLetterSpacing)
		out.LegendTitleLineHeight = copyFloat(t.Legend.TitleLineHeight)
		out.LegendTitleLetterSpacing = copyFloat(t.Legend.TitleLetterSpacing)
	}
	if t.Title != nil {
		out.TitleFilter = t.Title.Filter
		out.TitleLineHeight = copyFloat(t.Title.LineHeight)
		out.TitleLetterSpacing = copyFloat(t.Title.LetterSpacing)
	}
	if t.View != nil {
		out.ViewFilter = t.View.Filter
	}
	return out
}

// Clone returns a deep copy of the theme; lists, maps, and nested
// pointers are duplicated so sparse-override merges do not
// aliasing-leak.
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
	out.Mark = t.Mark.Clone()
	if t.Marks != nil {
		out.Marks = make(map[string]*MarkStyle, len(t.Marks))
		for k, v := range t.Marks {
			out.Marks[k] = v.Clone()
		}
	}
	if t.Axis != nil {
		v := *t.Axis
		if t.Axis.GridDash != nil {
			v.GridDash = append([]float64(nil), t.Axis.GridDash...)
		}
		out.Axis = &v
	}
	if t.Legend != nil {
		v := *t.Legend
		out.Legend = &v
	}
	if t.Title != nil {
		v := *t.Title
		out.Title = &v
	}
	if t.View != nil {
		v := *t.View
		out.View = &v
	}
	out.Range = t.Range.Clone()
	if t.States != nil {
		out.States = make(map[string]*StateStyle, len(t.States))
		for k, v := range t.States {
			if v == nil {
				out.States[k] = nil
				continue
			}
			cp := *v
			out.States[k] = &cp
		}
	}
	if t.Schemes != nil {
		out.Schemes = make(map[string][]string, len(t.Schemes))
		for k, v := range t.Schemes {
			out.Schemes[k] = append([]string(nil), v...)
		}
	}
	if t.Style != nil {
		out.Style = make(map[string]*MarkStyle, len(t.Style))
		for k, v := range t.Style {
			out.Style[k] = v.Clone()
		}
	}
	if t.Filters != nil {
		out.Filters = make(map[string]string, len(t.Filters))
		for k, v := range t.Filters {
			out.Filters[k] = v
		}
	}
	return &out
}

// MarkDefault returns the effective MarkStyle for markType after
// folding theme.Mark (global default) with theme.Marks[markType]
// (per-type override). Returns a fresh pointer that callers may
// mutate.
func (t *Theme) MarkDefault(markType string) *MarkStyle {
	if t == nil {
		return nil
	}
	base := t.Mark.Clone()
	if t.Marks == nil {
		return base
	}
	override, ok := t.Marks[markType]
	if !ok {
		return base
	}
	return MergeMarkStyle(base, override)
}
