package projection

// Equirectangular is the plate-carrée projection. Linear in both
// dimensions: x = scale * (lon - centerLon), y = -scale * (lat - centerLat).
// Has no domain restrictions.
type Equirectangular struct{ opts Options }

func (e *Equirectangular) Name() string        { return "equirectangular" }
func (e *Equirectangular) Configure(o Options) { e.opts = o }

func (e *Equirectangular) Project(lon, lat float64) (float64, float64, bool) {
	x := degToRad(lon-e.opts.Center[0]) * e.opts.Scale
	y := -degToRad(lat-e.opts.Center[1]) * e.opts.Scale
	return x + e.opts.Translate[0], y + e.opts.Translate[1], true
}

func applyEquirectDefaults(opts Options) Options {
	if opts.Scale == 0 {
		opts.Scale = 150
	}
	return opts
}
