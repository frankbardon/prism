// Package projection maps geographic coordinates (longitude / latitude
// in degrees) to plot-space pixels for the geoshape mark family.
//
// Every implementation satisfies the Projection interface:
//
//	type Projection interface {
//	    Project(lon, lat float64) (x, y float64, ok bool)
//	    Configure(opts Options)
//	}
//
// Coordinates outside the projection's valid domain (e.g. the poles
// for Mercator) return ok=false; the encoder treats them as a clip
// signal and drops the offending segment.
//
// Built-in projections (P18):
//
//	mercator         — Web Mercator (EPSG:3857), classic web-map default.
//	equirectangular  — Plate carrée; lon/lat → x/y linearly.
//	naturalearth     — Natural Earth pseudocylindrical (smooth, balanced).
//	albers_usa       — Composite Albers projection covering CONUS + AK + HI.
//	orthographic     — Globe view from infinity along the configured Rotate.
//
// Pixel coordinates pass through render.FormatFloat (3-decimal pin) at
// SVG emission time. Projection math runs in float64; rounding lives
// at the renderer boundary, not here.
package projection
