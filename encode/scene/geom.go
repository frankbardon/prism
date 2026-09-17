package scene

// CurveType is the line / area interpolation discriminator.
type CurveType string

const (
	CurveLinear     CurveType = "linear"
	CurveMonotone   CurveType = "monotone"
	CurveStep       CurveType = "step"
	CurveStepBefore CurveType = "step-before"
	CurveStepAfter  CurveType = "step-after"
	CurveCardinal   CurveType = "cardinal"
)

// PointShape is the point mark's symbol discriminator.
type PointShape string

const (
	ShapeCircle   PointShape = "circle"
	ShapeSquare   PointShape = "square"
	ShapeTriangle PointShape = "triangle"
	ShapeCross    PointShape = "cross"
	ShapeDiamond  PointShape = "diamond"
)

// PointShapes is the whole drawable shape vocabulary, in the order
// the JSON Schema publishes it. It is the single list both the
// encoder (legend.symbol_type resolution) and the validator
// (PRISM_SPEC_052) read, so neither can drift from what
// render/svg/symbols.go can actually emit.
var PointShapes = []PointShape{
	ShapeCircle,
	ShapeSquare,
	ShapeTriangle,
	ShapeCross,
	ShapeDiamond,
}

// TextAnchor controls horizontal text anchoring.
type TextAnchor string

const (
	AnchorStart  TextAnchor = "start"
	AnchorMiddle TextAnchor = "middle"
	AnchorEnd    TextAnchor = "end"
)

// TextBaseline controls vertical text alignment.
type TextBaseline string

const (
	BaselineAlphabetic TextBaseline = "alphabetic"
	BaselineMiddle     TextBaseline = "middle"
	BaselineHanging    TextBaseline = "hanging"
	BaselineTop        TextBaseline = "top"
	BaselineBottom     TextBaseline = "bottom"
)

// RectGeom is the geometry for a bar / rect mark.
type RectGeom struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	CornerR float64 `json:"corner_r,omitempty"`
}

// LineGeom is the geometry for a line mark (one polyline per mark).
//
// Tension parameterises CurveCardinal only (0–1, d3 semantics: the
// cardinal control-point scale is (1-Tension)/6). Every other curve
// ignores it. Zero is both the unset value and the d3/Vega-Lite
// default, so it stays out of the JSON unless an author sets it.
type LineGeom struct {
	Points  [][2]float64 `json:"points"`
	Dash    []float64    `json:"dash,omitempty"`
	Curve   CurveType    `json:"curve,omitempty"`
	Tension float64      `json:"tension,omitempty"`
}

// AreaGeom is the geometry for an area mark. Lower=nil → baseline 0.
// Curve applies to both the upper and the (reversed) lower edge, so a
// stacked band keeps parallel boundaries. Tension parameterises
// CurveCardinal only — see LineGeom.
type AreaGeom struct {
	Upper   [][2]float64 `json:"upper"`
	Lower   [][2]float64 `json:"lower,omitempty"`
	Curve   CurveType    `json:"curve,omitempty"`
	Tension float64      `json:"tension,omitempty"`
}

// PointGeom is the geometry for a point / scatter mark.
type PointGeom struct {
	Cx    float64    `json:"cx"`
	Cy    float64    `json:"cy"`
	R     float64    `json:"r"`
	Shape PointShape `json:"shape,omitempty"`
}

// RuleGeom is the geometry for a rule mark (horizontal or vertical line).
type RuleGeom struct {
	X1   float64   `json:"x1"`
	Y1   float64   `json:"y1"`
	X2   float64   `json:"x2"`
	Y2   float64   `json:"y2"`
	Dash []float64 `json:"dash,omitempty"`
}

// ArcGeom is the geometry for arc / pie / donut marks. PadAngle is
// the angular gap (radians) the renderer opens between this sector
// and its neighbours; the encoder carries the mark_def value through
// unchanged and render/svg's arcPath does the inset (E4-S1).
type ArcGeom struct {
	Cx         float64 `json:"cx"`
	Cy         float64 `json:"cy"`
	StartAngle float64 `json:"start_angle"`
	EndAngle   float64 `json:"end_angle"`
	InnerR     float64 `json:"inner_r,omitempty"`
	OuterR     float64 `json:"outer_r"`
	PadAngle   float64 `json:"pad_angle,omitempty"`
}

// TextGeom is the geometry for a text mark.
type TextGeom struct {
	X        float64      `json:"x"`
	Y        float64      `json:"y"`
	Content  string       `json:"content"`
	Anchor   TextAnchor   `json:"anchor,omitempty"`
	Baseline TextBaseline `json:"baseline,omitempty"`
	Angle    float64      `json:"angle,omitempty"`
	FontSize float64      `json:"font_size,omitempty"`
	// Dx and Dy offset the glyph from its anchor point
	// (spec.MarkDef.dx / dy, E4-S1). They are applied *after* Angle,
	// in the rotated frame — the renderer emits them as the SVG
	// `dx` / `dy` presentation attributes on <text>, which the
	// element's own rotate() transform has already rotated. That
	// matches Vega's text mark, whose transform is
	// translate(x,y) rotate(a) translate(dx,dy).
	Dx float64 `json:"dx,omitempty"`
	Dy float64 `json:"dy,omitempty"`
}

// PathGeom is the SVG-passthrough escape hatch for shapes Prism does
// not have first-class.
type PathGeom struct {
	D string `json:"d"`
}

// ImageGeom is the geometry for an image mark.
type ImageGeom struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Href string  `json:"href"`
}

// PolygonGeom is the geometry for a geoshape mark. Points are already
// in plot-space pixels (the projection has been applied upstream).
// Outer is the outer ring; Holes are inner rings (rendered with the
// SVG fill-rule cutout). Multipolygon features emit one PolygonGeom
// per disjoint piece on its own scene.Mark.
type PolygonGeom struct {
	Outer [][2]float64   `json:"outer"`
	Holes [][][2]float64 `json:"holes,omitempty"`
}
