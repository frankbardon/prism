# build_geodata

Fetches Natural Earth GeoJSON from the public-domain
[nvkelso/natural-earth-vector](https://github.com/nvkelso/natural-earth-vector)
GitHub mirror, quantizes to 3-decimal precision, strips properties to
`{id, name, iso_a2}`, and writes the Prism geo-bundle artifacts under
`geodata/`.

## Usage

```sh
make geodata
```

Run from the repo root. Requires network access. Refreshes:

| File | Source | Approx. size |
|---|---|---|
| `geodata/world-110m.geo.json` | `ne_110m_admin_0_countries.geojson` | 160 KB |
| `geodata/world-50m.geo.json` | `ne_50m_admin_0_countries.geojson` | 1.5 MB |
| `geodata/admin1-50m.geo.json` | `ne_50m_admin_1_states_provinces.geojson` | 1.0 MB |
| `geodata/manifest.json` | aggregated from the three tier files | 125 KB |

`make build` itself never needs network — the committed `geodata/*.json`
files are the input, and the geodata package embeds them via
`//go:embed`. Re-run `make geodata` only when:

- Natural Earth ships a new release upstream.
- A new admin tier or extra property needs to land in the manifest.
- The quantization factor changes (currently 3 — matches
  `render/precision.go`).

## ID conventions

- Admin-0 features: ISO 3166-1 alpha-3 (`USA`, `CAN`, `GBR`), falling
  back to Natural Earth's `ADM0_A3` for entities like Kosovo and
  Antarctic territories.
- Admin-1 features: ISO 3166-2 (`US-CA`, `CA-ON`, `GB-ENG`).

Features without a usable ID are dropped — they wouldn't be addressable
from a spec anyway. The 50m admin-1 source carries 294 features
(covering every US state + DC, every Canadian province, and major
admin-1 regions worldwide). Switching to the 10m admin-1 layer would
multiply that to ~4 000 entries; the trade-off is binary growth + slower
host CLI startup, so we ship the 50m tier as the default.
