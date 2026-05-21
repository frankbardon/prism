package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeGeopoint emits one scene.Point per row whose (longitude,
// latitude) projects to a valid pixel. Drops rows whose lat/lon falls
// outside the projection's domain (e.g. ±90° under Mercator).
func encodeGeopoint(in Inputs) ([]scene.Mark, error) {
	if in.Longitude.Field == "" || in.Latitude.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"geopoint mark requires encoding.longitude and encoding.latitude bindings.",
			map[string]any{"Field": "<longitude|latitude>", "Source": "<spec>", "Available": "longitude.field|latitude.field"},
		)
	}
	if in.Projection == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"geopoint mark requires a projection (spec.projection.type).",
			map[string]any{"Field": "<projection>", "Source": "<spec>"},
		)
	}
	lons, err := readField(in.Table, in.Longitude.Field)
	if err != nil {
		return nil, err
	}
	lats, err := readField(in.Table, in.Latitude.Field)
	if err != nil {
		return nil, err
	}
	if len(lons) != len(lats) {
		return nil, fmt.Errorf("encodeGeopoint: column length mismatch (lon=%d, lat=%d)", len(lons), len(lats))
	}
	radius := 4.0
	if in.Mark != nil && in.Mark.Size != nil {
		// size = area; r = sqrt(size/π)
		s := *in.Mark.Size
		if s > 0 {
			radius = sqrtPi(s)
		}
	}

	out := make([]scene.Mark, 0, len(lons))
	for i := range lons {
		lon, ok := toFloat(lons[i])
		if !ok {
			continue
		}
		lat, ok := toFloat(lats[i])
		if !ok {
			continue
		}
		x, y, ok := in.Projection.Project(lon, lat)
		if !ok {
			continue
		}
		out = append(out, scene.Mark{
			Type:  scene.MarkPoint,
			ID:    fmt.Sprintf("geopoint-%d", i),
			Style: in.Style,
			Point: &scene.PointGeom{Cx: x, Cy: y, R: radius, Shape: scene.ShapeCircle},
		})
	}
	return out, nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

func sqrtPi(area float64) float64 {
	// r = sqrt(area / pi). Math import avoided to keep file tight.
	r := area / 3.141592653589793
	// Newton's method, 6 iterations, plenty for IEEE doubles.
	x := r / 2
	for range 6 {
		x = (x + r/x) / 2
	}
	return x
}
