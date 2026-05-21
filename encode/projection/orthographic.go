package projection

import "math"

// Orthographic projects onto a plane tangent to the rotated globe.
// Hemisphere clipping is enforced by ok=false for points on the far
// side. Rotate[0]=lambda, Rotate[1]=phi are honoured; Rotate[2] (roll)
// is ignored — Prism currently has no UI gesture that exposes roll.
type Orthographic struct{ opts Options }

func (o *Orthographic) Name() string           { return "orthographic" }
func (o *Orthographic) Configure(opts Options) { o.opts = opts }

func (o *Orthographic) Project(lon, lat float64) (float64, float64, bool) {
	lambda := degToRad(lon - o.opts.Rotate[0])
	phi := degToRad(lat)
	phi0 := degToRad(o.opts.Rotate[1])
	cosc := math.Sin(phi0)*math.Sin(phi) + math.Cos(phi0)*math.Cos(phi)*math.Cos(lambda)
	if cosc < 0 {
		return 0, 0, false
	}
	x := o.opts.Scale * math.Cos(phi) * math.Sin(lambda)
	y := -o.opts.Scale * (math.Cos(phi0)*math.Sin(phi) - math.Sin(phi0)*math.Cos(phi)*math.Cos(lambda))
	return x + o.opts.Translate[0], y + o.opts.Translate[1], true
}

func applyOrthographicDefaults(opts Options) Options {
	if opts.Scale == 0 {
		opts.Scale = 240
	}
	return opts
}
