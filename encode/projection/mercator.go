package projection

import "math"

// Mercator is the conformal Web-Mercator projection. Domain is
// (lon ∈ [-180, 180], lat ∈ (-85.0511, 85.0511)). Latitudes outside
// the bounded strip map to ok=false (the encoder drops them); this
// matches d3-geo's behaviour and prevents the ±∞ blowup at the poles.
type Mercator struct {
	opts Options
}

const mercatorLatLimit = 85.05112878

// Name implements Projection.
func (m *Mercator) Name() string { return "mercator" }

// Configure implements Projection.
func (m *Mercator) Configure(opts Options) { m.opts = opts }

// Project implements Projection.
func (m *Mercator) Project(lon, lat float64) (float64, float64, bool) {
	if lat >= mercatorLatLimit || lat <= -mercatorLatLimit {
		return 0, 0, false
	}
	lambda := degToRad(lon - m.opts.Center[0])
	phi := degToRad(lat)
	x := m.opts.Scale * lambda
	y := -m.opts.Scale * math.Log(math.Tan(math.Pi/4+phi/2))
	// Center.Y shift so center[1] lands at translate.
	centerPhi := degToRad(m.opts.Center[1])
	yCenter := -m.opts.Scale * math.Log(math.Tan(math.Pi/4+centerPhi/2))
	y -= yCenter
	return x + m.opts.Translate[0], y + m.opts.Translate[1], true
}

func applyMercatorDefaults(opts Options) Options {
	if opts.Scale == 0 {
		opts.Scale = 150
	}
	return opts
}
