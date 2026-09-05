package spec

// Facet binds row/column facet channels for small multiples.
type Facet struct {
	Row    *FacetChannel `json:"row,omitempty"`
	Column *FacetChannel `json:"column,omitempty"`
}

// Repeat lists field names to repeat over.
type Repeat struct {
	Row    []string `json:"row,omitempty"`
	Column []string `json:"column,omitempty"`
	Layer  []string `json:"layer,omitempty"`
}

// Resolve maps per-channel modes for scale/axis/legend resolution.
type Resolve struct {
	Scale  *ResolveChannelMap `json:"scale,omitempty"`
	Axis   *ResolveChannelMap `json:"axis,omitempty"`
	Legend *ResolveChannelMap `json:"legend,omitempty"`
}

// ResolveChannelMap holds per-channel "shared" or "independent" tokens.
type ResolveChannelMap struct {
	X       string `json:"x,omitempty"`
	Y       string `json:"y,omitempty"`
	X2      string `json:"x2,omitempty"`
	Y2      string `json:"y2,omitempty"`
	Theta   string `json:"theta,omitempty"`
	Radius  string `json:"radius,omitempty"`
	Color   string `json:"color,omitempty"`
	Fill    string `json:"fill,omitempty"`
	Stroke  string `json:"stroke,omitempty"`
	Opacity string `json:"opacity,omitempty"`
	Size    string `json:"size,omitempty"`
	Shape   string `json:"shape,omitempty"`
}

// ThemeOverride is a sparse override on top of a registered theme.
// Name selects the registered base (light/dark/print/etc.); the
// other fields layer over it. The nested blocks mirror theme.Theme
// 1:1 — see spec/theme.go for the typed shape.
//
// Legacy flat fields (Background, Font, FontSize, Color, Palette,
// Scheme, Padding) remain for back-compat with v1 specs; they seed
// the nested blocks via theme.ApplyOverride.
type ThemeOverride struct {
	Name string `json:"name,omitempty"`
	// DarkVariant mirrors theme.Theme.DarkVariant — names a
	// registered counterpart theme for automatic light/dark
	// rendering (E4). Model + validation only in this story.
	DarkVariant string   `json:"dark_variant,omitempty"`
	Background  string   `json:"background,omitempty"`
	Font        string   `json:"font,omitempty"`
	FontSize    float64  `json:"font_size,omitempty"`
	Color       string   `json:"color,omitempty"`
	Palette     []string `json:"palette,omitempty"`
	Scheme      string   `json:"scheme,omitempty"`
	Padding     *Padding `json:"padding,omitempty"`

	// v2 nested blocks. Each is a pointer so JSON merges sparsely.
	Mark    *MarkStyle             `json:"mark,omitempty"`
	Marks   map[string]*MarkStyle  `json:"marks,omitempty"`
	Axis    *AxisStyle             `json:"axis,omitempty"`
	Legend  *LegendStyle           `json:"legend,omitempty"`
	Title   *TitleStyle            `json:"title,omitempty"`
	View    *ViewStyle             `json:"view,omitempty"`
	Range   *Range                 `json:"range,omitempty"`
	States  map[string]*StateStyle `json:"states,omitempty"`
	Schemes map[string][]string    `json:"schemes,omitempty"`
	Style   map[string]*MarkStyle  `json:"style,omitempty"`

	// Filters mirrors theme.Theme.Filters — a named registry of raw
	// SVG <filter> inner-content bodies. Style blocks above reference
	// an entry via their Filter field. RawCSS mirrors theme.Theme.RawCSS
	// — raw CSS appended verbatim to the emitted <style> block. Both
	// are an escape hatch: this JSON is developer-authored and trusted
	// the same as the rest of the spec — never route untrusted content
	// through here.
	Filters map[string]string `json:"filters,omitempty"`
	RawCSS  string            `json:"raw_css,omitempty"`

	// Gradients mirrors theme.Theme.Gradients — a named registry of
	// linear/radial gradient definitions. Patterns mirrors
	// theme.Theme.Patterns — a named registry of pattern fills (built-
	// in catalogue or raw-SVG Content, same trust tier as Filters/
	// RawCSS). A Fill/Stroke/Background value written as url(#name)
	// resolves against these registries (theme.Theme.ResolveFillRef)
	// and the SVG renderer emits a matching
	// <linearGradient>/<radialGradient>/<pattern> def.
	Gradients map[string]GradientDef `json:"gradients,omitempty"`
	Patterns  map[string]PatternDef  `json:"patterns,omitempty"`
}
