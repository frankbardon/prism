package projection

import "math"

// NaturalEarth implements Tom Patterson's "Natural Earth" projection
// (a pseudocylindrical compromise between distortion and aesthetics).
// Polynomial form per Šavrič et al. 2011.
type NaturalEarth struct{ opts Options }

func (n *NaturalEarth) Name() string        { return "naturalearth" }
func (n *NaturalEarth) Configure(o Options) { n.opts = o }

func (n *NaturalEarth) Project(lon, lat float64) (float64, float64, bool) {
	lambda := degToRad(lon - n.opts.Center[0])
	phi := degToRad(lat - n.opts.Center[1])
	phi2 := phi * phi
	phi4 := phi2 * phi2
	// Šavrič 2011 polynomials.
	xFactor := 0.8707 - 0.131979*phi2 - phi4*(0.013791+phi2*(0.003971-0.001529*phi2))
	yFactor := phi * (1.007226 + phi2*(0.015085-phi4*(0.044475-0.028874*phi2-0.005916*phi4)))
	x := n.opts.Scale * lambda * xFactor
	y := -n.opts.Scale * yFactor
	return x + n.opts.Translate[0], y + n.opts.Translate[1], true
}

func applyNaturalEarthDefaults(opts Options) Options {
	if opts.Scale == 0 {
		opts.Scale = 150
	}
	return opts
}

var _ = math.Pi // silence import lint when polynomial form removes math calls
