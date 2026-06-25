package spec

// MarkDef carries all per-mark visual properties. Fields are pointer-typed
// where a meaningful zero (e.g. 0 stroke width vs unset) must be preserved.
type MarkDef struct {
	Type          string    `json:"type"`
	Fill          string    `json:"fill,omitempty"`
	Stroke        string    `json:"stroke,omitempty"`
	StrokeWidth   *float64  `json:"stroke_width,omitempty"`
	StrokeDash    []float64 `json:"stroke_dash,omitempty"`
	Opacity       *float64  `json:"opacity,omitempty"`
	FillOpacity   *float64  `json:"fill_opacity,omitempty"`
	StrokeOpacity *float64  `json:"stroke_opacity,omitempty"`
	CornerRadius  *float64  `json:"corner_radius,omitempty"`
	Size          *float64  `json:"size,omitempty"`
	Shape         string    `json:"shape,omitempty"`
	Interpolate   string    `json:"interpolate,omitempty"`
	Tension       *float64  `json:"tension,omitempty"`
	Orient        string    `json:"orient,omitempty"`
	Align         string    `json:"align,omitempty"`
	Baseline      string    `json:"baseline,omitempty"`
	Font          string    `json:"font,omitempty"`
	FontSize      *float64  `json:"font_size,omitempty"`
	FontWeight    any       `json:"font_weight,omitempty"`
	FontStyle     string    `json:"font_style,omitempty"`
	Angle         *float64  `json:"angle,omitempty"`
	Dx            *float64  `json:"dx,omitempty"`
	Dy            *float64  `json:"dy,omitempty"`
	Tooltip       any       `json:"tooltip,omitempty"`
	InnerRadius   *float64  `json:"inner_radius,omitempty"`
	OuterRadius   *float64  `json:"outer_radius,omitempty"`
	// InnerRadiusRatio (P10) is the donut hole's inner radius as a
	// fraction of OuterR (0–1). When set, takes precedence over the
	// default donut ratio (0.55). Ignored when InnerRadius is also
	// set (InnerRadius is absolute pixels, wins).
	InnerRadiusRatio *float64 `json:"inner_radius_ratio,omitempty"`
	PadAngle         *float64 `json:"pad_angle,omitempty"`
	URL              string   `json:"url,omitempty"`
	Path             string   `json:"path,omitempty"`
	// Maxbins (P10) caps the bin count for histogram marks. nil = use
	// Sturges' rule default (ceil(log2(n) + 1)).
	Maxbins *int `json:"maxbins,omitempty"`
	// ViolinResolution (P10) sets the number of KDE sample points per
	// violin group. nil = 64 (the D061 default).
	ViolinResolution *int `json:"violin_resolution,omitempty"`
	// LinkShape (tree / dendrogram, tier1-04) — "step" | "curve" |
	// "straight". Default "step".
	LinkShape string `json:"link_shape,omitempty"`
	// NodeShape (tree / network) — "circle" | "rect" | "none".
	// Default "circle".
	NodeShape string `json:"node_shape,omitempty"`
	// NodeSize (tree / network) — base node radius / side length.
	// Default 6.
	NodeSize *float64 `json:"node_size,omitempty"`
	// Layout (network) — "force" | "random". Default "force".
	Layout string `json:"layout,omitempty"`
	// Iterations (network) — force iterations. Default 200, cap 2000.
	Iterations *int `json:"iterations,omitempty"`
	// LinkDistance (network) — preferred edge length. Default 30.
	LinkDistance *float64 `json:"link_distance,omitempty"`
	// Charge (network) — repulsion strength. Default -30.
	Charge *float64 `json:"charge,omitempty"`
	// Seed (network) — deterministic seed for the force layout.
	// Default 42.
	Seed *int64 `json:"seed,omitempty"`
	// Bullet KPI mark (E3) carries its inputs as mark-def fields
	// rather than encoding channels, mirroring histogram / violin /
	// sankey.
	//
	// Target is the reference value to beat. It may be a literal
	// number or a string naming a data field to resolve per row.
	Target any `json:"target,omitempty"`
	// Bands is an ordered list of qualitative range bounds (e.g.
	// poor / ok / good thresholds), ascending.
	Bands []float64 `json:"bands,omitempty"`
	// Comparative is a secondary measure value (e.g. prior period).
	// Like Target it may be a literal number or a data-field name.
	Comparative any `json:"comparative,omitempty"`
	// Orientation selects the bullet layout direction —
	// "horizontal" | "vertical". Default "horizontal".
	Orientation string `json:"orientation,omitempty"`
}

// Mark is the discriminated mark form: string shorthand or full mark_def
// object. The discriminator is the JSON type of the input.
// UnmarshalJSON is implemented in mark_union.go (T01.14).
type Mark struct {
	// Shorthand is the bare mark type string ("bar", "line", ...).
	Shorthand string
	// Def is the full mark definition. When Shorthand is set, Def is nil
	// and vice versa.
	Def *MarkDef
}

// TypeName returns the effective mark type regardless of input form.
func (m *Mark) TypeName() string {
	if m == nil {
		return ""
	}
	if m.Shorthand != "" {
		return m.Shorthand
	}
	if m.Def != nil {
		return m.Def.Type
	}
	return ""
}
