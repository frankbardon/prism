package scene

// CurveType is the line / area interpolation discriminator.
type CurveType string

const (
	CurveLinear   CurveType = "linear"
	CurveMonotone CurveType = "monotone"
	CurveStep     CurveType = "step"
	CurveCardinal CurveType = "cardinal"
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
type LineGeom struct {
	Points [][2]float64 `json:"points"`
	Dash   []float64    `json:"dash,omitempty"`
	Curve  CurveType    `json:"curve,omitempty"`
}

// AreaGeom is the geometry for an area mark. Lower=nil → baseline 0.
type AreaGeom struct {
	Upper [][2]float64 `json:"upper"`
	Lower [][2]float64 `json:"lower,omitempty"`
	Curve CurveType    `json:"curve,omitempty"`
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

// ArcGeom is the geometry for arc / pie / donut marks (declared for
// JSON stability; encoder emits PRISM_WARN_MARK_NOT_IMPLEMENTED in P05).
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
