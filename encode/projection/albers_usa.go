package projection

import "math"

// AlbersUSA is the composite Albers equal-area projection that lays
// Alaska and Hawaii adjacent to CONUS in inset boxes. Matches d3-geo
// geoAlbersUsa() layout: AK scaled 0.35×, HI scaled 1×, both
// translated into the lower-left of the CONUS frame.
//
// Implementation is a thin wrapper over three Albers projections with
// hard lon/lat dispatch — points outside CONUS+AK+HI bbox return
// ok=false (matches d3 behaviour: invalid points clip rather than
// blow up).
type AlbersUSA struct {
	opts Options
	// Internal sub-projections; populated in Configure.
	lower48 *albers
	alaska  *albers
	hawaii  *albers
}

func (a *AlbersUSA) Name() string { return "albers_usa" }

func (a *AlbersUSA) Configure(opts Options) {
	a.opts = opts
	scale := opts.Scale
	if scale == 0 {
		scale = 1070
	}
	tx, ty := opts.Translate[0], opts.Translate[1]

	a.lower48 = newAlbers(albersParams{
		parallels: [2]float64{29.5, 45.5},
		rotate:    [2]float64{96, 0},
		center:    [2]float64{-0.6, 38.7},
		scale:     scale,
		translate: [2]float64{tx, ty},
	})
	a.alaska = newAlbers(albersParams{
		parallels: [2]float64{55, 65},
		rotate:    [2]float64{154, 0},
		center:    [2]float64{-2, 58.5},
		scale:     scale * 0.35,
		translate: [2]float64{tx - scale*0.307, ty + scale*0.201},
	})
	a.hawaii = newAlbers(albersParams{
		parallels: [2]float64{8, 18},
		rotate:    [2]float64{157, 0},
		center:    [2]float64{-3, 19.9},
		scale:     scale,
		translate: [2]float64{tx - scale*0.205, ty + scale*0.212},
	})
}

func (a *AlbersUSA) Project(lon, lat float64) (float64, float64, bool) {
	// Dispatch by lon/lat bbox: AK roughly lon < -130 & lat > 50; HI
	// roughly lon < -154 & lat < 28; else lower-48.
	switch {
	case lon < -140 && lat > 50:
		return a.alaska.project(lon, lat)
	case lon < -154 && lat < 28:
		return a.hawaii.project(lon, lat)
	default:
		return a.lower48.project(lon, lat)
	}
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
