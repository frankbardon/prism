package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/projection"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/geodata"
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
	switch name {
	case "albers_usa", "albersUsa":
		// AlbersUSA is a composite projection with hard-coded layout
		// proportions (CONUS + AK + HI in a canonical ~960x500 viewport
		// → scale 1070 per d3-geo). Generic bbox-fit destroys the
		// internal scale relationships, so use the canonical aspect.
		if needFitScale {
			fitOpts.Scale = albersUSAScale(plot.W, plot.H)
		}
		if needFitTranslate {
			fitOpts.Translate = [2]float64{plot.X + plot.W/2, plot.Y + plot.H/2}
		}
	case "orthographic":
		// Globe view — fit a unit sphere into the plot square.
		if needFitScale {
			fitOpts.Scale = math.Min(plot.W, plot.H) * 0.45
		}
		if needFitTranslate {
			fitOpts.Translate = [2]float64{plot.X + plot.W/2, plot.Y + plot.H/2}
		}
	default:
		// Cylindrical / pseudocylindrical projections fit naturally
		// from the data's lon/lat bbox.
		tier := geodata.TierWorld110m
		if p != nil && p.Tier != "" {
			tier = geodata.Tier(p.Tier)
		}
		bbW, bbS, bbE, bbN := manifestBBox(tier)
		scale, translate := projection.SizeFromBBox(proj, bbW, bbS, bbE, bbN, plot.W, plot.H)
		if needFitScale {
			fitOpts.Scale = scale
		}
		if needFitTranslate {
			fitOpts.Translate = [2]float64{plot.X + translate[0], plot.Y + translate[1]}
		}
	}
	proj.Configure(fitOpts)
	return proj, nil
}

// albersUSAScale returns the canonical AlbersUSA scale for a given
// plot rectangle. d3-geo's default is scale=1070 for a 960×500
// viewport, so we scale linearly with whichever axis is the binding
// constraint.
func albersUSAScale(plotW, plotH float64) float64 {
	const refScale = 1070.0
	const refW = 960.0
	const refH = 500.0
	return math.Min(refScale*plotW/refW, refScale*plotH/refH)
}

// manifestBBox returns the union bounding box of every feature in the
// manifest matching the given tier. Used to auto-fit projections.
func manifestBBox(tier geodata.Tier) (w, s, e, n float64) {
	m, err := geodata.LoadManifest()
	if err != nil || m == nil {
		return -180, -90, 180, 90
	}
	w, s, e, n = 181, 91, -181, -91
	any := false
	for _, f := range m.Features {
		if f.Tier != tier {
			continue
		}
		any = true
		if f.BBox.West < w {
			w = f.BBox.West
		}
		if f.BBox.South < s {
			s = f.BBox.South
		}
		if f.BBox.East > e {
			e = f.BBox.East
		}
		if f.BBox.North > n {
			n = f.BBox.North
		}
	}
	if !any {
		return -180, -90, 180, 90
	}
	return w, s, e, n
}
