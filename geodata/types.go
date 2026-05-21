package geodata

// Tier names the resolution + admin level a feature set belongs to.
type Tier string

const (
	TierWorld110m  Tier = "world-110m"
	TierWorld50m   Tier = "world-50m"
	TierAdmin1_50m Tier = "admin1-50m"
)

// AllTiers returns every tier the manifest knows about.
func AllTiers() []Tier {
	return []Tier{TierWorld110m, TierWorld50m, TierAdmin1_50m}
}

// BBox is a longitude/latitude bounding box. West/East may straddle
// the antimeridian (West > East) for features that cross 180°.
type BBox struct {
	West  float64 `json:"w"`
	South float64 `json:"s"`
	East  float64 `json:"e"`
	North float64 `json:"n"`
}

// FeatureMeta is the manifest-resident record for one feature. Carries
// only what validate / inspect / plan need: ID, name, ISO codes, and
// the rough geographic envelope. Geometry lives in the per-tier topo
// archive and is loaded lazily via Store.
type FeatureMeta struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	ISOA2    string     `json:"iso_a2,omitempty"`
	Centroid [2]float64 `json:"c,omitempty"` // [lon, lat]
	BBox     BBox       `json:"bb"`
	Tier     Tier       `json:"t"`
	// Parent links an admin-1 feature to its admin-0 ISO alpha-3 (e.g.
	// US-CA's Parent = "USA"). Empty for admin-0 features.
	Parent string `json:"p,omitempty"`
}

// Manifest is the catalog embedded in every build. Indexed by feature
// ID for O(1) lookup.
type Manifest struct {
	Version  int                     `json:"version"`
	Features map[string]*FeatureMeta `json:"features"`
}

// Feature is the resolved geometry payload returned by Store.Lookup.
// Coordinates are raw longitude/latitude (degrees); projection happens
// in encode/projection. Rings follow the GeoJSON winding convention:
// outer rings counter-clockwise, holes clockwise.
type Feature struct {
	ID       string
	Name     string
	Polygons []Polygon
}

// Polygon is one outer ring plus zero or more holes. A feature with
// multiple disjoint pieces (e.g. an archipelago) is represented as
// multiple Polygon entries on Feature.
type Polygon struct {
	Outer Ring
	Holes []Ring
}

// Ring is a closed sequence of [lon, lat] pairs. The first and last
// point are NOT required to match — renderers close the ring.
type Ring [][2]float64
