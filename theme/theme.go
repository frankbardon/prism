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

	// DarkVariant names a registered counterpart theme used for
	// automatic light/dark rendering (E4). Setting it alone is the
	// opt-in — there is no separate flag. A non-empty value must name
	// a theme already present in the registry (or registered earlier
	// in the same init-time load batch — see theme/registry.go);
	// an unresolved name fails loudly with
	// PRISM_THEME_DARK_VARIANT_UNKNOWN at Register/LoadFile/LoadBytes
	// time, the same fail-loud posture as Filter/url(#name)
	// references (theme/validate.go). Model + validation only in this
	// story — the dual-palette chrome emission lands in E4-S2 and the
	// mark-color re-plumb in E4-S3.
	DarkVariant string `json:"dark_variant,omitempty"`

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
	Mark *MarkStyle `json:"mark,omitempty"`
	// Marks is keyed by mark type (bar, line, point, …) with one
	// intentional exception: a mark family that draws more than one
	// element per row claims an extra key per element, named
	// <mark>_<element>. MarksKeyProgressTrack is the first — see its
	// doc comment for why that beats a dedicated block on Theme.
	Marks map[string]*MarkStyle `json:"marks,omitempty"`
	// Axis is the shared axis block: every token set here applies to
	// both cartesian axes. AxisX / AxisY layer over it **per property**
	// for one axis only — a block that sets nothing but grid_color
	// leaves every other token inherited from Axis. nil means "inherit
	// Axis unchanged", which is why a theme that states neither is
	// byte-for-byte identical to one authored before they existed.
	//
	// Because the x axis draws the *vertical* grid lines and the y axis
	// the *horizontal* ones, these blocks are also how grid lines are
	// themed per orientation (E8-S1).
	//
	// Resolution lives in exactly one place — AxisFor — and the full
	// precedence chain is documented there.
	Axis   *AxisStyle             `json:"axis,omitempty"`
	AxisX  *AxisStyle             `json:"axis_x,omitempty"`
	AxisY  *AxisStyle             `json:"axis_y,omitempty"`
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

	// Gradients is a named registry of linear/radial gradient
	// definitions. Style blocks will reference an entry via
	// url(#name) once Fill/Stroke/Background resolution wires it up
	// (E3-S2); actual <linearGradient>/<radialGradient> SVG emission
	// lands in E3-S3. Model + validation only in this story.
	Gradients map[string]GradientDef `json:"gradients,omitempty"`
	// Patterns is a named registry of pattern fills — either a
	// built-in catalogue entry (see PatternTypes) tuned via
	// Color/Spacing/Size, or a raw-SVG Content body (same trust tier
	// as Filters/RawCSS). See Gradients for the resolution/emission
	// timeline.
	Patterns map[string]PatternDef `json:"patterns,omitempty"`

	// CategoryStyles is a theme-level data-driven style map: outer key
	// is a field name, inner key is a stringified field value, leaf is
	// a full MarkStyle applied automatically to marks whose bound
	// field/value matches an entry — without requiring a spec-level
	// `condition` block for the common case. Intentionally richer than
	// Range (which is color-only, keyed by scale role rather than an
	// actual data value): a category style can set any MarkStyle
	// token, not just a fill color.
	//
	// A nested field→value→style map is used instead of a flat
	// "field=value" string key to stay consistent with the project's
	// no-expression-language stance (see spec/predicate.go,
	// spec/calc_expr.go) — no string grammar to parse.
	//
	// Precedence: a spec's own `spec.Condition` targeting the same
	// field/value wins over the matching CategoryStyles entry
	// (explicit beats theme default). Applied at encode time by
	// encode.applyCategoryStyles, which runs before
	// encode.applyConditions so a later-applied condition naturally
	// overwrites whichever attrs it targets (encode/encode_category_styles.go).
	CategoryStyles map[string]map[string]*MarkStyle `json:"category_styles,omitempty"`
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
		// E3-S2: the tick-size / label-padding tokens drive real
		// geometry in render/svg, not just the CSS-variable manifest.
		out.AxisTickSize = copyFloat(t.Axis.TickSize)
		out.AxisLabelPadding = copyFloat(t.Axis.LabelPadding)
		// E8-S2: title_padding joins them. It was emitted as
		// --prism-axis-title-padding from the start but never read —
		// the title coordinate was hard-coded — so it is the same
		// species of dead geometry token the two above used to be.
		out.AxisTitlePadding = copyFloat(t.Axis.TitlePadding)
	}
	// E8-S1: the per-axis `axis_x` / `axis_y` overrides, narrowed to
	// the tokens a CSS variable cannot express. The colour/stroke half
	// of these blocks leaves via theme/css.go's scoped
	// `.prism-axis-x` / `.prism-axis-y` declarations instead.
	out.AxisX = sceneAxisTokens(t.AxisX)
	out.AxisY = sceneAxisTokens(t.AxisY)
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
		out.ViewBackgroundRef = t.ResolveFillRef(t.View.Background).DefID()
	}
	// Gradients/Patterns (E3-S3): pass the whole registry through,
	// pre-resolved to SVG-emission-ready shape, so render/svg can emit
	// one <linearGradient>/<radialGradient>/<pattern> def per entry —
	// same "emit the whole registry regardless of reference" approach
	// as Filters above.
	if len(t.Gradients) > 0 {
		out.Gradients = make(map[string]scene.Gradient, len(t.Gradients))
		for k, v := range t.Gradients {
			out.Gradients[k] = v.sceneDef()
		}
	}
	if len(t.Patterns) > 0 {
		out.Patterns = make(map[string]scene.Pattern, len(t.Patterns))
		for k, v := range t.Patterns {
			out.Patterns[k] = v.sceneDef()
		}
	}
	return out
}

// sceneAxisTokens projects a per-axis AxisStyle onto the Scene IR
// subset the renderer needs (geometry + SVG-attribute typography +
// the group filter). Returns nil when the block is nil or states
// nothing in that subset, so a theme whose `axis_x` is colour-only
// adds no bytes to the serialised scene.
func sceneAxisTokens(a *AxisStyle) *scene.AxisTokens {
	if a == nil {
		return nil
	}
	out := &scene.AxisTokens{
		TickSize:           copyFloat(a.TickSize),
		LabelPadding:       copyFloat(a.LabelPadding),
		TitlePadding:       copyFloat(a.TitlePadding),
		LabelLineHeight:    copyFloat(a.LabelLineHeight),
		LabelLetterSpacing: copyFloat(a.LabelLetterSpacing),
		TitleLineHeight:    copyFloat(a.TitleLineHeight),
		TitleLetterSpacing: copyFloat(a.TitleLetterSpacing),
		Filter:             a.Filter,
	}
	if *out == (scene.AxisTokens{}) {
		return nil
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
	out.Axis = cloneAxisStyle(t.Axis)
	out.AxisX = cloneAxisStyle(t.AxisX)
	out.AxisY = cloneAxisStyle(t.AxisY)
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
	if t.Gradients != nil {
		out.Gradients = make(map[string]GradientDef, len(t.Gradients))
		for k, v := range t.Gradients {
			out.Gradients[k] = v.Clone()
		}
	}
	if t.Patterns != nil {
		out.Patterns = make(map[string]PatternDef, len(t.Patterns))
		for k, v := range t.Patterns {
			out.Patterns[k] = v.Clone()
		}
	}
	out.CategoryStyles = cloneCategoryStyles(t.CategoryStyles)
	return &out
}

// Marks keys that are not mark types.
//
// A progress mark draws two elements per row — the value bar and the
// unfilled track behind it — and they need independent paint: a track
// that inherits the bar's fill is invisible, and a track hardcoded in
// the encoder is the bulletBandShade mistake (encode/marks/bullet.go
// interpolates fixed greys no theme can reach).
//
// The track therefore claims its own Marks key rather than a new
// block on Theme. Marks is already map[string]*MarkStyle, so the key
// inherits the whole existing pipeline unchanged — Clone, Merge,
// ApplyOverride, Validate, the theme.schema.json additionalProperties
// map, and the --prism-mark-progress_track-* CSS variables css.go
// emits for every entry. A dedicated block would have to re-earn all
// of it.
const (
	// MarksKeyProgress styles a progress mark's value bar.
	MarksKeyProgress = "progress"
	// MarksKeyProgressTrack styles a progress mark's unfilled track.
	// Unlike MarksKeyProgress it is NOT a mark type: no spec can set
	// mark.type to it (the schema's mark_type enum does not list it),
	// and encode never looks it up through MarkDefault, because the
	// theme.Mark global default is the data-mark fill and folding it
	// in is exactly how the track would end up the same colour as the
	// bar. The key is read directly; see encode.progressTrackStyle.
	MarksKeyProgressTrack = "progress_track"
	// MarksKeyBoxplotMedian styles the median line a boxplot draws
	// ACROSS its box. Like MarksKeyProgressTrack it is not a mark
	// type and is read directly rather than through MarkDefault --
	// and for the same reason, sharpened: the global theme.Mark fill
	// is the box's own colour, so folding it in paints the median the
	// colour of the shape it is drawn on top of, which is invisible
	// in a different way than having no stroke at all. The whiskers
	// and caps do NOT use this key; they sit outside the box, where
	// the box's colour is the correct and legible choice, and they
	// get it from marks.StrokeStyleFor.
	MarksKeyBoxplotMedian = "boxplot_median"
)

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
