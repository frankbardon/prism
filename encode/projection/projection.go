package projection

import (
	"fmt"
	"math"
)

// Options carries the parameters every projection accepts. Fields not
// applicable to a given projection are ignored (e.g. Mercator ignores
// Rotate's roll axis). Defaults documented per-projection in New.
type Options struct {
	// Scale is the radius the projection treats as one unit of distance
	// on the plot. For a unit-sphere projection (mercator, equirect),
	// Scale ≈ pixels-per-radian / (2π). The encoder sizes Scale from
	// the plot rectangle when unset.
	Scale float64
	// Center is the [lon, lat] focal point of the projection in degrees.
	// Equirectangular and Mercator translate so Center maps to the
	// plot's geometric centre.
	Center [2]float64
	// Rotate is the [lambda, phi, gamma] rotation in degrees applied
	// before projection. Used by orthographic (defaults to {0,0,0})
	// and by composite Albers presets. Mercator ignores it.
	Rotate [3]float64
	// Translate is the [px, py] pixel-space origin the projected
	// coordinates anchor on. Defaults to the plot rectangle's centre.
	Translate [2]float64
	// ClipExtent is the optional [[minX, minY], [maxX, maxY]] rectangle
	// outputs are clipped to. Zero value disables clipping; callers
	// (encoder) set it to the plot rect.
	ClipExtent [2][2]float64
}

// Projection projects geographic coordinates into pixel space.
type Projection interface {
	// Project maps (lon, lat) degrees to (x, y) pixels. ok=false means
	// the input is outside the projection's valid domain (e.g. ±90°
	// latitude under Mercator) and callers should drop the segment.
	Project(lon, lat float64) (x, y float64, ok bool)
	// Configure replaces the projection's options.
	Configure(opts Options)
	// Name returns the canonical projection identifier
	// ("mercator", "equirectangular", ...).
	Name() string
}

// New constructs a projection by name. Returns an error when name is
// unknown. Pass an empty Options to accept defaults.
func New(name string, opts Options) (Projection, error) {
	switch name {
	case "", "mercator":
		p := &Mercator{}
		p.Configure(applyMercatorDefaults(opts))
		return p, nil
	case "equirectangular":
		p := &Equirectangular{}
		p.Configure(applyEquirectDefaults(opts))
		return p, nil
	case "naturalearth":
		p := &NaturalEarth{}
		p.Configure(applyNaturalEarthDefaults(opts))
		return p, nil
	case "albers_usa", "albersUsa":
		p := &AlbersUSA{}
		p.Configure(applyAlbersUSADefaults(opts))
		return p, nil
	case "orthographic":
		p := &Orthographic{}
		p.Configure(applyOrthographicDefaults(opts))
		return p, nil
	default:
		return nil, fmt.Errorf("projection: unknown name %q (try mercator|equirectangular|naturalearth|albers_usa|orthographic)", name)
	}
}

// Available returns the canonical projection names in stable order.
// Used by validate error messages.
func Available() []string {
	return []string{"mercator", "equirectangular", "naturalearth", "albers_usa", "orthographic"}
}

// degToRad converts degrees to radians.
func degToRad(d float64) float64 { return d * math.Pi / 180 }

// SizeFromBBox returns a Scale + Translate pair that fits the given
// lon/lat bbox into the supplied pixel rectangle for a unit-sphere
// projection (mercator / equirectangular / naturalearth). The encoder
// calls this when the spec leaves Scale/Translate unset, so charts
// auto-fit their data extent.
func SizeFromBBox(p Projection, bboxW, bboxS, bboxE, bboxN, pxW, pxH float64) (scale float64, translate [2]float64) {
	p.Configure(Options{Scale: 1, Translate: [2]float64{0, 0}})
	x0, y0, _ := p.Project(bboxW, bboxN)
	x1, y1, _ := p.Project(bboxE, bboxS)
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	w := x1 - x0
	h := y1 - y0
	if w <= 0 || h <= 0 {
		return 1, [2]float64{pxW / 2, pxH / 2}
	}
	scale = math.Min(pxW/w, pxH/h)
	translate = [2]float64{
		pxW/2 - scale*(x0+x1)/2,
		pxH/2 - scale*(y0+y1)/2,
	}
	return scale, translate
}
