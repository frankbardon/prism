package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/projection"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// buildProjection materialises a projection.Projection from spec.Projection
// + the plot rectangle. When the spec leaves Scale or Translate unset,
// the projection auto-fits to the requested tier's bbox so the chart
// renders sensibly without manual tuning.
func buildProjection(p *spec.Projection, plot scene.Rect) (projection.Projection, error) {
	name := ""
	if p != nil {
		name = p.Type
	}
	opts := projection.Options{}
	if p != nil {
		if p.Center != nil {
			opts.Center = *p.Center
		}
		if p.Rotate != nil {
			opts.Rotate = *p.Rotate
		}
		if p.Scale != nil {
			opts.Scale = *p.Scale
		}
		if p.Translate != nil {
			opts.Translate = *p.Translate
		}
	}
	proj, err := projection.New(name, opts)
	if err != nil {
		return nil, err
	}
	// Auto-fit when the spec didn't pin scale/translate.
	needFitScale := p == nil || p.Scale == nil
	needFitTranslate := p == nil || p.Translate == nil
	if !needFitScale && !needFitTranslate {
		return proj, nil
	}

	fitOpts := opts
	if needFitScale {
		fitOpts.Scale = canonicalScale(name, plot.W, plot.H)
	}
	if needFitTranslate {
		fitOpts.Translate = [2]float64{plot.X + plot.W/2, plot.Y + plot.H/2}
	}
	proj.Configure(fitOpts)
	return proj, nil
}

// canonicalScale returns the d3-geo canonical scale for a projection
// at the given plot rectangle. Each constant comes from d3-geo's
// reference 960×500 viewport (or 960×600 for cylindrical), so a Prism
// chart at any size gets a balanced world view that matches the
// conventions readers are used to from d3 / Vega-Lite world maps.
//
// We avoid fit-to-data here on purpose — auto-fitting world maps to
// the data bbox over-emphasises Antarctica (lat=-90) and yields a
// squashed equator. The SVG's overflow:hidden clips the south-pole
// overshoot so the visible result is a familiar "Atlas" framing.
func canonicalScale(name string, plotW, plotH float64) float64 {
	switch name {
	case "albers_usa", "albersUsa":
		// d3-geo: geoAlbersUsa default scale 1070 at 960×500.
		return math.Min(1070*plotW/960, 1070*plotH/500)
	case "orthographic":
		// Fits the unit sphere snugly into the plot square.
		return math.Min(plotW, plotH) * 0.45
	case "mercator":
		// d3-geo: geoMercator default scale 961/2π ≈ 152.96, for a
		// world view at 960×500. We crop poles via SVG clip.
		return math.Min(152.96*plotW/960, 152.96*plotH/500)
	case "equirectangular":
		// d3-geo: geoEquirectangular default scale 152.63 at 960×500.
		return math.Min(152.63*plotW/960, 152.63*plotH/500)
	case "naturalearth":
		// d3-geo: geoNaturalEarth1 default scale 158.94 at 960×500.
		return math.Min(158.94*plotW/960, 158.94*plotH/500)
	}
	// Unknown projection — fall back to a conservative cylindrical fit.
	return math.Min(150*plotW/960, 150*plotH/500)
}
