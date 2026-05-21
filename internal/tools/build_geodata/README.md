# build_geodata

Regenerates the committed `geodata/*.geo.json` + `geodata/manifest.json`
artifacts from upstream Natural Earth source data. The committed
artifacts are the only inputs `make build` needs; this tool exists to
refresh them when:

- Natural Earth ships a new release.
- An additional admin level (e.g. ZIP codes, census tracts) is
  promoted from prototype to a committed tier.
- The quantization factor or stripped-property set changes.

## Manual procedure (v1)

1. Download the Natural Earth shapefiles you want at the resolution
   tier you target. Public domain, no attribution required:
   - <https://www.naturalearthdata.com/downloads/110m-cultural-vectors/110m-admin-0-countries/>
   - <https://www.naturalearthdata.com/downloads/50m-cultural-vectors/50m-admin-0-countries/>
   - <https://www.naturalearthdata.com/downloads/50m-cultural-vectors/50m-admin-1-states-provinces/>

2. Convert each shapefile to GeoJSON. Mapshaper (npm) is the
   reference tool:
   ```sh
   npx mapshaper ne_110m_admin_0_countries.shp \
     -filter-fields ISO_A3,NAME,ISO_A2 \
     -clean \
     -o format=geojson world-110m.geojson
   ```

3. Quantize to 3-decimal precision (lon/lat × 1000 → int32) and
   convert to the Prism geo-bundle format. The bundle shape lives in
   `geodata/decoder.go` (`bundleV1` + `bundleFeature`).

   For a single tier:
   ```sh
   # Pseudocode — real implementation pending.
   prism-geodata-quantize \
     --in world-110m.geojson \
     --tier world-110m \
     --id-field ISO_A3 \
     --name-field NAME \
     --out geodata/world-110m.geo.json
   ```

4. Regenerate the manifest by aggregating every tier's bbox +
   centroid + parent (for admin-1):
   ```sh
   prism-geodata-manifest \
     --in geodata/world-110m.geo.json,geodata/world-50m.geo.json,geodata/admin1-50m.geo.json \
     --out geodata/manifest.json
   ```

5. Commit the regenerated artifacts. `make test` runs the geo smoke
   test which catches mismatches between manifest IDs and bundle
   feature IDs.

## Automated pipeline (post-v1)

A future PR replaces this README with a Go-based pipeline:

- Fetch upstream shapefiles from `naturalearthdata.com`.
- Decode shapefiles via a pure-Go reader.
- Strip properties to `{id, name, iso_a2}` only.
- Quantize coordinates.
- Emit bundle + manifest in lockstep.

The committed artifacts that ship today are a curated 16-country
sample (admin-0) + 5-state sample (admin-1) sufficient for the gallery
and smoke tests; downstream consumers wanting full Natural Earth
coverage run the manual procedure above until the automated pipeline
lands.
