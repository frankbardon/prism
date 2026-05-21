package spec

// Projection configures the lon/lat → pixel mapping used by the
// geoshape and geopoint marks. Mirrors d3-geo's projection API but
// pared down to the parameters Prism's projection package actually
// honours.
//
// Fields are pointer-typed where a meaningful zero (e.g. scale=0
// meaning "auto-fit") must be distinguishable from "unset". Strings
// hold no semantic zero — empty means default.
type Projection struct {
	Type      string      `json:"type,omitempty"`
	Scale     *float64    `json:"scale,omitempty"`
	Center    *[2]float64 `json:"center,omitempty"`
	Rotate    *[3]float64 `json:"rotate,omitempty"`
	Translate *[2]float64 `json:"translate,omitempty"`
	// Tier is the geodata tier the encoder looks up feature geometry
	// from. Defaults to "world-110m" for admin-0 charts; admin-1 charts
	// must request "admin1-50m" explicitly.
	Tier string `json:"tier,omitempty"`
}
