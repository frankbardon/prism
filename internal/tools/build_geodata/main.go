// Command build_geodata fetches Natural Earth GeoJSON from the
// public-domain nvkelso/natural-earth-vector GitHub mirror and writes
// the Prism geo-bundle artifacts under geodata/:
//
//	world-110m.geo.json  — admin-0 (countries) at 1:110m
//	world-50m.geo.json   — admin-0 (countries) at 1:50m
//	admin1-50m.geo.json  — admin-1 (states/provinces) at 1:50m
//	manifest.json        — index of every feature (id, bbox, centroid)
//
// Run via `make geodata`. Requires network access. `make build` does
// NOT need this tool — the committed artifacts are the input.
//
// Property stripping: keeps {id, name, iso_a2} only. ID comes from
// ISO_A3 (admin-0) or ISO_3166_2 (admin-1); features without an ID
// are dropped (they wouldn't be addressable from a spec anyway).
//
// Quantization: coordinates are rounded to 3 decimals (lon/lat × 1000
// as int32) to match render/precision.go's stability contract.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	urlAdmin0_110m = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_admin_0_countries.geojson"
	urlAdmin0_50m  = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_50m_admin_0_countries.geojson"
	urlAdmin1_50m  = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_50m_admin_1_states_provinces.geojson"
	quantizeFactor = 3
)

func main() {
	outDir := flag.String("out", "geodata", "output directory (relative to repo root)")
	timeout := flag.Duration("timeout", 60*time.Second, "per-fetch HTTP timeout")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal("mkdir %s: %v", *outDir, err)
	}

	client := &http.Client{Timeout: *timeout}

	world110, err := buildWorld(client, urlAdmin0_110m, "world-110m")
	if err != nil {
		fatal("fetch admin-0 110m: %v", err)
	}
	world50, err := buildWorld(client, urlAdmin0_50m, "world-50m")
	if err != nil {
		fatal("fetch admin-0 50m: %v", err)
	}
	admin1, err := buildAdmin1(client, urlAdmin1_50m)
	if err != nil {
		fatal("fetch admin-1 50m: %v", err)
	}

	writeBundle(filepath.Join(*outDir, "world-110m.geo.json"), world110.bundle)
	writeBundle(filepath.Join(*outDir, "world-50m.geo.json"), world50.bundle)
	writeBundle(filepath.Join(*outDir, "admin1-50m.geo.json"), admin1.bundle)

	manifest := buildManifest(world110.metas, admin1.metas)
	writeManifest(filepath.Join(*outDir, "manifest.json"), manifest)

	fmt.Printf("build_geodata: wrote %d countries (110m), %d countries (50m), %d admin-1 features\n",
		len(world110.bundle.Features), len(world50.bundle.Features), len(admin1.bundle.Features))
	fmt.Printf("build_geodata: manifest carries %d entries total\n", len(manifest.Features))
}

type ingestResult struct {
	bundle bundleV1
	metas  []featureMeta
}

func buildWorld(client *http.Client, url, tier string) (*ingestResult, error) {
	raw, err := fetchJSON(client, url)
	if err != nil {
		return nil, err
	}
	var fc featureCollection
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}
	return processFeatures(&fc, tier, extractAdmin0Props), nil
}

func buildAdmin1(client *http.Client, url string) (*ingestResult, error) {
	raw, err := fetchJSON(client, url)
	if err != nil {
		return nil, err
	}
	var fc featureCollection
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}
	return processFeatures(&fc, "admin1-50m", extractAdmin1Props), nil
}

type extractFn func(props map[string]any) (id, name, isoA2, parent string)

func extractAdmin0Props(p map[string]any) (string, string, string, string) {
	id := str(p, "ISO_A3")
	if id == "" || id == "-99" {
		id = str(p, "ADM0_A3")
	}
	if id == "" || id == "-99" {
		return "", "", "", ""
	}
	name := str(p, "NAME")
	if name == "" {
		name = str(p, "ADMIN")
	}
	return id, name, str(p, "ISO_A2"), ""
}

func extractAdmin1Props(p map[string]any) (string, string, string, string) {
	id := str(p, "iso_3166_2")
	if id == "" || id == "-99" {
		return "", "", "", ""
	}
	name := str(p, "name")
	if name == "" {
		name = str(p, "name_en")
	}
	parent := str(p, "adm0_a3")
	return id, name, "", parent
}

func processFeatures(fc *featureCollection, tier string, extract extractFn) *ingestResult {
	scale := math.Pow(10, float64(quantizeFactor))
	out := &ingestResult{
		bundle: bundleV1{Version: 1, Tier: tier, Quantize: quantizeFactor},
	}
	for _, f := range fc.Features {
		id, name, isoA2, parent := extract(f.Properties)
		if id == "" {
			continue
		}
		polysRaw, bbox, centroid := walkGeometry(f.Geometry, scale)
		if len(polysRaw) == 0 {
			continue
		}
		out.bundle.Features = append(out.bundle.Features, bundleFeature{
			ID:       id,
			Name:     name,
			Polygons: polysRaw,
		})
		out.metas = append(out.metas, featureMeta{
			ID:       id,
			Name:     name,
			ISOA2:    isoA2,
			Centroid: centroid,
			BBox:     bbox,
			Tier:     tier,
			Parent:   parent,
		})
	}
	sort.Slice(out.bundle.Features, func(i, j int) bool {
		return out.bundle.Features[i].ID < out.bundle.Features[j].ID
	})
	sort.Slice(out.metas, func(i, j int) bool { return out.metas[i].ID < out.metas[j].ID })
	return out
}

// walkGeometry handles GeoJSON Polygon + MultiPolygon. Returns the
// quantized polygons, lon/lat bbox, and centroid (area-weighted for
// MultiPolygon; outer-ring centroid for Polygon).
func walkGeometry(g geometry, scale float64) ([]polygonRaw, bbox, [2]float64) {
	var polys [][][][]float64 // [poly][ring][point][lon|lat]
	switch g.Type {
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(g.Coordinates, &rings); err != nil {
			return nil, bbox{}, [2]float64{}
		}
		polys = append(polys, rings)
	case "MultiPolygon":
		var multi [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &multi); err != nil {
			return nil, bbox{}, [2]float64{}
		}
		polys = append(polys, multi...)
	default:
		return nil, bbox{}, [2]float64{}
	}

	out := make([]polygonRaw, 0, len(polys))
	bb := bbox{West: 181, South: 91, East: -181, North: -91}
	totalArea := 0.0
	cxAcc, cyAcc := 0.0, 0.0
	for _, polyRings := range polys {
		quantized := make(polygonRaw, 0, len(polyRings))
		for _, ring := range polyRings {
			q := make(ringRaw, 0, len(ring))
			for _, pt := range ring {
				lonQ := int32(math.Round(pt[0] * scale))
				latQ := int32(math.Round(pt[1] * scale))
				q = append(q, [2]int32{lonQ, latQ})
				updateBBox(&bb, pt[0], pt[1])
			}
			quantized = append(quantized, q)
		}
		out = append(out, quantized)
		if len(polyRings) > 0 {
			a := signedArea(polyRings[0])
			cx, cy := centroidOfRing(polyRings[0])
			absA := math.Abs(a)
			totalArea += absA
			cxAcc += cx * absA
			cyAcc += cy * absA
		}
	}
	var centroid [2]float64
	if totalArea > 0 {
		centroid = [2]float64{round3(cxAcc / totalArea), round3(cyAcc / totalArea)}
	}
	return out, bb, centroid
}

func signedArea(ring [][]float64) float64 {
	n := len(ring)
	if n < 3 {
		return 0
	}
	a := 0.0
	for i := range n {
		x1, y1 := ring[i][0], ring[i][1]
		x2, y2 := ring[(i+1)%n][0], ring[(i+1)%n][1]
		a += x1*y2 - x2*y1
	}
	return a / 2
}

func centroidOfRing(ring [][]float64) (float64, float64) {
	n := len(ring)
	if n < 3 {
		return 0, 0
	}
	a := signedArea(ring) * 6
	if a == 0 {
		// Degenerate ring; fall back to vertex mean.
		var sx, sy float64
		for _, p := range ring {
			sx += p[0]
			sy += p[1]
		}
		return sx / float64(n), sy / float64(n)
	}
	cx, cy := 0.0, 0.0
	for i := range n {
		x1, y1 := ring[i][0], ring[i][1]
		x2, y2 := ring[(i+1)%n][0], ring[(i+1)%n][1]
		f := x1*y2 - x2*y1
		cx += (x1 + x2) * f
		cy += (y1 + y2) * f
	}
	return cx / a, cy / a
}

func updateBBox(b *bbox, lon, lat float64) {
	if lon < b.West {
		b.West = round3(lon)
	}
	if lon > b.East {
		b.East = round3(lon)
	}
	if lat < b.South {
		b.South = round3(lat)
	}
	if lat > b.North {
		b.North = round3(lat)
	}
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func str(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func fetchJSON(client *http.Client, url string) ([]byte, error) {
	fmt.Printf("build_geodata: fetching %s\n", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ---------------------------------------------------------------- //
// Output types (mirror geodata/decoder.go's wire shape)
// ---------------------------------------------------------------- //

type bundleV1 struct {
	Version  int             `json:"version"`
	Tier     string          `json:"tier"`
	Quantize int             `json:"quantize"`
	Features []bundleFeature `json:"features"`
}

type bundleFeature struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Polygons []polygonRaw `json:"polygons"`
}

type polygonRaw = []ringRaw
type ringRaw = [][2]int32

// ---------------------------------------------------------------- //
// GeoJSON input types
// ---------------------------------------------------------------- //

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

type feature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   geometry       `json:"geometry"`
}

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// ---------------------------------------------------------------- //
// Manifest types (mirror geodata/types.go)
// ---------------------------------------------------------------- //

type bbox struct {
	West  float64 `json:"w"`
	South float64 `json:"s"`
	East  float64 `json:"e"`
	North float64 `json:"n"`
}

type featureMeta struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	ISOA2    string     `json:"iso_a2,omitempty"`
	Centroid [2]float64 `json:"c,omitempty"`
	BBox     bbox       `json:"bb"`
	Tier     string     `json:"t"`
	Parent   string     `json:"p,omitempty"`
}

type manifestV1 struct {
	Version  int                     `json:"version"`
	Features map[string]*featureMeta `json:"features"`
}

func buildManifest(worldMetas, admin1Metas []featureMeta) *manifestV1 {
	m := &manifestV1{Version: 1, Features: map[string]*featureMeta{}}
	for i := range worldMetas {
		fm := worldMetas[i]
		m.Features[fm.ID] = &fm
	}
	for i := range admin1Metas {
		fm := admin1Metas[i]
		m.Features[fm.ID] = &fm
	}
	return m
}

// ---------------------------------------------------------------- //
// Output writers
// ---------------------------------------------------------------- //

func writeBundle(path string, b bundleV1) {
	body, err := json.Marshal(b)
	if err != nil {
		fatal("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
	fmt.Printf("build_geodata: wrote %s (%d bytes)\n", path, len(body))
}

func writeManifest(path string, m *manifestV1) {
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fatal("marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
	fmt.Printf("build_geodata: wrote %s (%d bytes)\n", path, len(body))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "build_geodata: "+format+"\n", args...)
	os.Exit(1)
}
