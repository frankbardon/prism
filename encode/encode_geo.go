package encode

import (
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
	// Auto-fit when the spec didn't pin scale/translate. Use the
	// requested tier's bbox so admin-1 charts don't over-zoom.
	needFitScale := p == nil || p.Scale == nil
	needFitTranslate := p == nil || p.Translate == nil
	if needFitScale || needFitTranslate {
		tier := geodata.TierWorld110m
		if p != nil && p.Tier != "" {
			tier = geodata.Tier(p.Tier)
		}
		bbW, bbS, bbE, bbN := manifestBBox(tier)
		scale, translate := projection.SizeFromBBox(proj, bbW, bbS, bbE, bbN, plot.W, plot.H)
		fitOpts := opts
		if needFitScale {
			fitOpts.Scale = scale
		}
		if needFitTranslate {
			fitOpts.Translate = [2]float64{plot.X + translate[0], plot.Y + translate[1]}
		}
		proj.Configure(fitOpts)
	}
	return proj, nil
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
