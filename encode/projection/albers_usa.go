package projection

import "math"

// AlbersUSA is the composite Albers equal-area projection that lays
// Alaska and Hawaii adjacent to CONUS in inset boxes. Matches
// d3-geo's geoAlbersUsa() layout (CONUS via lower48 parallels 29.5°
// / 45.5°, AK as a 35%-scaled inset bottom-left of CONUS, HI as a
// 100%-scaled inset to the right of AK).
//
// Per-vertex dispatch follows d3's stream pattern: try each
// sub-projection in turn, accept the first whose projected pixel lands
// inside the sub-projection's pixel-space extent. Vertices outside
// every extent return ok=false so the encoder drops them. This stops
// multipart features (e.g. the Aleutian chain crossing -180°) from
// half-projecting through the wrong sub-projection.
type AlbersUSA struct {
	opts  Options
	parts []albersPart
}

type albersPart struct {
	proj   *albers
	extent [4]float64 // [minX, minY, maxX, maxY] in pixel space
}

func (a *AlbersUSA) Name() string { return "albers_usa" }

func (a *AlbersUSA) Configure(opts Options) {
	a.opts = opts
	scale := opts.Scale
	if scale == 0 {
		scale = 1070
	}
	tx, ty := opts.Translate[0], opts.Translate[1]

	// Extents follow d3-geo's geoAlbersUsa clipExtent layout, expressed
	// as offsets from translate scaled by `scale`. The numbers come
	// directly from d3's source — they're the four corners of each
	// sub-projection's clip rectangle in a unit (scale=1) viewport.
	lower48 := newAlbers(albersParams{
		parallels: [2]float64{29.5, 45.5},
		rotate:    [2]float64{96, 0},
		center:    [2]float64{-0.6, 38.7},
		scale:     scale,
		translate: [2]float64{tx, ty},
	})
	alaska := newAlbers(albersParams{
		parallels: [2]float64{55, 65},
		rotate:    [2]float64{154, 0},
		center:    [2]float64{-2, 58.5},
		scale:     scale * 0.35,
		translate: [2]float64{tx - scale*0.307, ty + scale*0.201},
	})
	hawaii := newAlbers(albersParams{
		parallels: [2]float64{8, 18},
		rotate:    [2]float64{157, 0},
		center:    [2]float64{-3, 19.9},
		scale:     scale,
		translate: [2]float64{tx - scale*0.205, ty + scale*0.212},
	})

	a.parts = []albersPart{
		// CONUS clip extent (d3's lower48.clipExtent in unit-scale).
		{proj: lower48, extent: clipRect(tx, ty, scale,
			-0.455, -0.238, 0.455, 0.238)},
		// Alaska inset extent: a ~0.165×0.140 rect at the lower-left.
		{proj: alaska, extent: clipRect(tx, ty, scale,
			-0.425, 0.120, -0.214, 0.234)},
		// Hawaii inset extent: a ~0.094×0.063 rect between AK and CONUS.
		{proj: hawaii, extent: clipRect(tx, ty, scale,
			-0.214, 0.166, -0.115, 0.234)},
	}
}

// clipRect maps unit-viewport corners (relative to translate, in
// scale-1 units) to absolute pixel coordinates.
func clipRect(tx, ty, scale, x0, y0, x1, y1 float64) [4]float64 {
	return [4]float64{
		tx + scale*x0,
		ty + scale*y0,
		tx + scale*x1,
		ty + scale*y1,
	}
}

func (a *AlbersUSA) Project(lon, lat float64) (float64, float64, bool) {
	for _, part := range a.parts {
		x, y, ok := part.proj.project(lon, lat)
		if !ok {
			continue
		}
		if x >= part.extent[0] && x <= part.extent[2] &&
			y >= part.extent[1] && y <= part.extent[3] {
			return x, y, true
		}
	}
	return 0, 0, false
}

func applyAlbersUSADefaults(opts Options) Options {
	if opts.Scale == 0 {
		opts.Scale = 1070
	}
	return opts
}

// albers is the conic-equal-area projection underlying AlbersUSA. Not
// exported because the only consumer is the composite — callers
// wanting a free-standing Albers should add a top-level type later.
type albers struct {
	p         albersParams
	n, c, rho float64
}

type albersParams struct {
	parallels [2]float64
	rotate    [2]float64
	center    [2]float64
	scale     float64
	translate [2]float64
}

func newAlbers(p albersParams) *albers {
	phi1 := degToRad(p.parallels[0])
	phi2 := degToRad(p.parallels[1])
	sinPhi1 := math.Sin(phi1)
	n := sinPhi1
	if phi1 != phi2 {
		n = (sinPhi1 + math.Sin(phi2)) / 2
	}
	c := math.Cos(phi1)*math.Cos(phi1) + 2*n*sinPhi1
	rho0 := math.Sqrt(c-2*n*math.Sin(degToRad(p.center[1]))) / n
	return &albers{p: p, n: n, c: c, rho: rho0}
}

func (a *albers) project(lon, lat float64) (float64, float64, bool) {
	lambda := degToRad(lon + a.p.rotate[0])
	phi := degToRad(lat)
	rho := math.Sqrt(a.c-2*a.n*math.Sin(phi)) / a.n
	x := rho * math.Sin(a.n*lambda) * a.p.scale
	y := (a.rho - rho*math.Cos(a.n*lambda)) * a.p.scale
	return x + a.p.translate[0], y + a.p.translate[1], true
}
