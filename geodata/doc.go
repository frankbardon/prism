// Package geodata embeds vector boundary data (countries + admin-1
// regions) for Prism's geoshape mark. It exposes two surfaces:
//
//   - Manifest: a lightweight catalog of feature IDs, names, ISO codes,
//     and bounding boxes. Embedded into every build (host + WASM) so
//     validate/inspect/plan stay no-execute. ~100 KB ungzipped.
//
//   - Store: feature geometries (polygon rings, projected later by
//     encode/projection). For the host build the full TopoJSON tier
//     archives are embedded via //go:embed (~6 MB on disk, acceptable
//     for a CLI binary). For the WASM build the embeds are absent and
//     the loader auto-fetches the tier from `prism static-bundle`'s
//     geodata/ directory on first access.
//
// Tier conventions:
//
//	world-110m  — Natural Earth admin-0 at 1:110m (countries; coarse).
//	world-50m   — Natural Earth admin-0 at 1:50m (countries; standard).
//	admin1-50m  — Natural Earth admin-1 at 1:50m (states/provinces).
//
// Feature IDs follow ISO conventions:
//
//	admin-0 features: ISO 3166-1 alpha-3 (USA, CAN, GBR, …)
//	admin-1 features: ISO 3166-2 (US-CA, CA-ON, GB-ENG, …)
//
// Source of truth: Natural Earth Data (public domain, naturalearthdata.com).
// Regenerated via `make geodata`, which runs internal/tools/build_geodata.
// The committed *.topo + manifest.bin are the inputs `make build`
// honours — no network required at build time.
package geodata
