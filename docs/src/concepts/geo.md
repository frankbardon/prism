# Geographic Marks

Prism ships two geo-aware marks for choropleth maps and georeferenced
overlays. Both consume a `projection` block on the spec and resolve
boundary geometry from an embedded geodata catalog — no external tile
server, no runtime network dependency for the host CLI.

## Marks

| Mark | Channels | Purpose |
|---|---|---|
| `geoshape` | `feature` (+ optional `color`) | Country / admin-1 polygon (choropleth). |
| `geopoint` | `longitude`, `latitude` (+ optional `color`, `size`) | Point overlay (cities, events, sensors). |

## Spec shape

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"source": "country_metrics.pulse"},
  "mark": "geoshape",
  "projection": {"type": "naturalearth"},
  "encoding": {
    "feature": {"field": "iso_a3", "type": "nominal"},
    "color":   {"field": "gdp_per_capita", "type": "quantitative"}
  }
}
```

The `feature` channel binds a table column whose values are feature
IDs from the geodata catalog. Admin-0 (countries) uses ISO 3166-1
alpha-3 (`USA`, `CAN`, `GBR`); admin-1 (states/provinces) uses
ISO 3166-2 (`US-CA`, `CA-ON`, `GB-ENG`).

## Projections

| Type | Use case |
|---|---|
| `mercator` | Classic web map default. Distorts area near poles; clips above ±85°. |
| `equirectangular` | Plate carrée. Linear lat/lon → x/y. Useful for heatmaps over geographic grids. |
| `naturalearth` | Tom Patterson's compromise projection. Smooth global view, low distortion. |
| `albers_usa` | Composite Albers covering CONUS + Alaska + Hawaii in inset panels. |
| `orthographic` | Globe view. Honours `rotate: [lambda, phi, gamma]` for the view direction. |

Per-projection parameters:

```json
{
  "projection": {
    "type": "albers_usa",
    "scale": 1200,
    "translate": [400, 250]
  }
}
```

Leave `scale` / `translate` unset and Prism auto-fits the projection
to the requested tier's bounding box inside the plot rectangle.

## Tiers

The geodata catalog ships three tiers:

| Tier | Coverage | Approx. on-disk size |
|---|---|---|
| `world-110m` | Countries (admin-0) at 1:110m. Default. | ~200 KB gz |
| `world-50m` | Countries (admin-0) at 1:50m. Smoother coastlines. | ~600 KB gz |
| `admin1-50m` | States / provinces (admin-1) at 1:50m. | ~5 MB gz |

Select the tier the encoder pulls from:

```json
{
  "projection": {"type": "mercator", "tier": "admin1-50m"}
}
```

The committed tier files carry 177 countries (110m), 242 countries
(50m), and 294 admin-1 regions (50m) sourced from Natural Earth via
`make geodata`. Tier files use a custom compact JSON shape with
3-decimal quantization; `geodata/decoder.go` documents the wire
format. `make build` itself requires no network — the committed
artifacts are the input.

## Host CLI vs WASM

**Host build (CLI / library):** every tier is embedded in the binary
via `//go:embed`. Geoshape charts work out-of-the-box; no sideload
required.

**WASM build (browser):** only the manifest is embedded (~100 KB). The
runtime fetches the tier file from `${origin}/static/prism/geodata/<tier>.geo.json`
on first encode. Set a custom URL via:

```js
prism.geo.setBundleURL("https://cdn.example.com/geodata/");
```

`prism static-bundle ./public/prism` always emits the geodata
artifacts under `<out>/geodata/` so the WASM runtime finds them.

For pages that inline the tier bytes:

```js
prism.geo.primeTier("world-110m", new Uint8Array(buffer));
```

Optional eager fetch:

```js
await prism.geo.preload("admin1-50m");
```

## Validation

The `PRISM_SPEC_021` rule fires when:

- `mark` is `geoshape` or `geopoint` but `projection` is missing or
  declares an unknown `type`.
- A geoshape spec lacks `encoding.feature.field`.
- A geopoint spec lacks `encoding.longitude.field` or
  `encoding.latitude.field`.
- `projection.tier` is set to a value outside the known tiers.

Runtime errors:

- `PRISM_GEO_001` — feature id in a row is not in the manifest tier.
- `PRISM_GEO_002` — bundle fetch failed (WASM) or embed missing (host).

## Custom maps

Custom feature sets live outside the v1 scope. The manifest +
`world-110m.geo.json` files use a small documented format
(`geodata/decoder.go`); future work surfaces a public loader so
downstream apps can ship their own admin levels (e.g. ZIP codes,
census tracts) via the same `feature` channel.
