# Geographic Marks

Prism ships two geo-aware marks for choropleth maps and georeferenced
overlays. Both consume a `projection` block on the spec and resolve
boundary geometry from a geodata catalog — no external tile server.
The lightweight manifest (feature ids + bounding boxes) is embedded in
every build, so validate / plan / inspect work with no setup. The heavier
tier geometry is loaded at runtime: the host CLI reads it from a directory
you point it at (`--geodata-dir` / `PRISM_GEODATA`), and the WASM build
fetches it from a configurable URL. See
[Host CLI vs WASM](#host-cli-vs-wasm) below.

## Marks

| Mark | Channels | Purpose |
|---|---|---|
| `geoshape` | `feature` (+ optional `color`) | Country / admin-1 polygon (choropleth). |
| `geopoint` | `longitude`, `latitude` (+ optional `color`, `size`) | Point overlay (cities, events, sensors). |

## Spec shape

A basemap (every country in the catalog, no data binding) is one
line of data:

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"feature_collection": {"tier": "world-110m"}},
  "mark": "geoshape",
  "projection": {"type": "naturalearth"},
  "encoding": {"feature": {"field": "id", "type": "nominal"}}
}
```

`data.feature_collection` synthesizes one row per feature in the
tier — the resulting table carries `id`, `name`, and `parent` columns
(parent is the admin-0 ISO 3166-1 alpha-3 for admin-1 entries, empty
otherwise). Combine with a filter transform to subset:

```json
{
  "data": {"feature_collection": {"tier": "admin1-50m"}},
  "transform": [{"filter": "parent == \"USA\""}],
  "mark": "geoshape",
  "projection": {"type": "albers_usa", "tier": "admin1-50m"},
  "encoding": {"feature": {"field": "id", "type": "nominal"}}
}
```

For a choropleth, bind your own data and a color channel:

```json
{
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
ISO 3166-2 (`US-CA`, `CA-ON`, `GB-ENG`). Rows whose `id` doesn't
match a manifest entry raise `PRISM_GEO_001`.

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

**Host build (CLI / library):** only the manifest (~128 KB) is embedded.
Tier geometry is loaded at runtime from a directory you supply — the host
binary no longer embeds the three tier files. Point the loader at that
directory with the `--geodata-dir` flag or the `PRISM_GEODATA` environment
variable:

```
prism plot world.json --geodata-dir ./geodata > world.svg
# or
PRISM_GEODATA=./geodata prism plot world.json > world.svg
```

The directory must contain the tier files named `<tier>.geo.json`
(`world-110m.geo.json`, `world-50m.geo.json`, `admin1-50m.geo.json`). The
flag is available on the leaves that materialise geometry — `plot`,
`scene`, `serve`, `mcp`, and `static-bundle`. The no-execute leaves
(`validate`, `plan`, `inspect`) and the data-only `execute` leaf use only
the embedded manifest and do not take the flag.

Rendering a `geoshape` / `geopoint` mark hard-fails when geometry cannot
be resolved:

- `PRISM_GEODATA_DIR_UNSET` — no directory was configured (neither
  `--geodata-dir` nor `PRISM_GEODATA` is set) and a geo mark needs a tier.
- `PRISM_GEODATA_TIER_MISSING` — a directory is configured but it does not
  contain the requested `<tier>.geo.json` file.

### Getting the tier files

The committed tiers ship in the repo's `geodata/` directory, so a checkout
already has them: `--geodata-dir ./geodata`. For a standalone binary,
download the files from the docs site and point `--geodata-dir` at the
folder you saved them in:

```
mkdir geodata && cd geodata
curl -O https://frankbardon.github.io/prism/static/prism/geodata/world-110m.geo.json
curl -O https://frankbardon.github.io/prism/static/prism/geodata/world-50m.geo.json
curl -O https://frankbardon.github.io/prism/static/prism/geodata/admin1-50m.geo.json
cd ..
prism plot world.json --geodata-dir ./geodata > world.svg
```

Download only the tiers your specs reference — `world-110m` alone is
enough for a country basemap. You can also emit the files locally with
`prism static-bundle` (see below).

**WASM build (browser):** only the manifest is embedded (~128 KB). The
runtime fetches the tier file from `${origin}/static/prism/geodata/<tier>.geo.json`
on first encode. Set a custom URL via:

```js
prism.geo.setBundleURL("https://cdn.example.com/geodata/");
```

`prism static-bundle --geodata-dir ./geodata ./public/prism` emits the
geodata artifacts under `<out>/geodata/` so the WASM runtime finds them.
Because the host build no longer embeds the tiers, `static-bundle` sources
them from the `--geodata-dir` directory; if it is unset, the command fails
with `PRISM_GEODATA_DIR_UNSET`.

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
- `PRISM_GEO_002` — bundle fetch failed (WASM).
- `PRISM_GEODATA_DIR_UNSET` — host render of a geo mark with no
  `--geodata-dir` / `PRISM_GEODATA` configured.
- `PRISM_GEODATA_TIER_MISSING` — the configured directory does not contain
  the requested `<tier>.geo.json` file.

## Custom maps

Custom feature sets live outside the v1 scope. The manifest +
`world-110m.geo.json` files use a small documented format
(`geodata/decoder.go`); future work surfaces a public loader so
downstream apps can ship their own admin levels (e.g. ZIP codes,
census tracts) via the same `feature` channel.
