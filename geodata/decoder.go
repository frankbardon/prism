package geodata

import (
	"encoding/json"
	"fmt"
)

// bundleV1 is the on-disk shape produced by internal/tools/build_geodata
// and consumed at runtime. The custom format keeps the decoder small
// (no TopoJSON arc indirection) and lets the build tool fully control
// quantization and property stripping.
//
// Coordinates are stored as int32 scaled by Quantize (factor of 10).
// A Quantize value of 3 means the on-disk values are degrees × 1000;
// the decoder divides them back to float64 degrees on read.
type bundleV1 struct {
	Version  int             `json:"version"`
	Tier     string          `json:"tier"`
	Quantize int             `json:"quantize"`
	Features []bundleFeature `json:"features"`
}

type bundleFeature struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Polygons []bundlePolygonRaw `json:"polygons"`
}

// bundlePolygonRaw is a polygon as written to disk: an array of rings
// where the first ring is the outer boundary and remaining rings are
// holes. Each ring is an array of [lon_q, lat_q] integer pairs.
type bundlePolygonRaw [][][2]int32

// decodeBundle parses one tier-bundle's bytes into the map shape the
// memory store caches.
func decodeBundle(raw []byte) (map[string]*Feature, error) {
	var b bundleV1
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("decode bundle JSON: %w", err)
	}
	if b.Version != 1 {
		return nil, fmt.Errorf("unsupported bundle version %d", b.Version)
	}
	if b.Quantize <= 0 || b.Quantize > 9 {
		return nil, fmt.Errorf("invalid quantize factor %d (expected 1-9)", b.Quantize)
	}
	scale := pow10(b.Quantize)
	out := make(map[string]*Feature, len(b.Features))
	for _, f := range b.Features {
		feat := &Feature{
			ID:       f.ID,
			Name:     f.Name,
			Polygons: make([]Polygon, 0, len(f.Polygons)),
		}
		for _, polyRaw := range f.Polygons {
			if len(polyRaw) == 0 {
				continue
			}
			poly := Polygon{Outer: dequantizeRing(polyRaw[0], scale)}
			if len(polyRaw) > 1 {
				poly.Holes = make([]Ring, 0, len(polyRaw)-1)
				for _, h := range polyRaw[1:] {
					poly.Holes = append(poly.Holes, dequantizeRing(h, scale))
				}
			}
			feat.Polygons = append(feat.Polygons, poly)
		}
		out[f.ID] = feat
	}
	return out, nil
}

func dequantizeRing(in [][2]int32, scale float64) Ring {
	out := make(Ring, len(in))
	for i, p := range in {
		out[i] = [2]float64{float64(p[0]) / scale, float64(p[1]) / scale}
	}
	return out
}

func pow10(n int) float64 {
	out := 1.0
	for range n {
		out *= 10
	}
	return out
}
