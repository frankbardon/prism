# Vendored D3 modules

Pinned per D070. Each `.mjs` file is the Rollup-bundled ESM build
from jsdelivr (`https://cdn.jsdelivr.net/npm/<name>@<version>/+esm`),
committed verbatim. The bundles inline every internal `d3-*`
dependency so no peer resolution happens at runtime.

## Inventory

See `VERSIONS.json` for the authoritative manifest (name, version,
source URL, sha256, byte size). `TestPrismD3VendoredPinned`
(`internal/devtools/d3_pinning_test.go`) recomputes sha256 on every
test run and asserts the digest matches — catches accidental edits,
partial downloads, and supply-chain tampering.

Modules:

| Module          | Version | Purpose                                |
| --------------- | ------- | -------------------------------------- |
| d3-array        | 3.2.4   | quantiles, ticks, bisect               |
| d3-axis         | 3.0.0   | axis tick generation                   |
| d3-brush        | 3.0.0   | interval selection (P13)               |
| d3-format       | 3.1.0   | number formatting                      |
| d3-scale        | 4.0.2   | scale resolution (when client computes) |
| d3-shape        | 3.2.0   | line / area curve interpolation        |
| d3-time-format  | 4.1.0   | date formatting                        |
| d3-zoom         | 3.0.0   | pan / zoom (post-v1)                   |

## Update protocol

Auto-updates are forbidden (D070). To bump a module:

1. Edit both `internal/devtools/vendor-d3.sh` and
   `internal/devtools/vendor-d3-manifest.go` so their version
   tables match the desired versions.
2. Run `bash internal/devtools/vendor-d3.sh` (downloads from
   jsdelivr).
3. Regenerate the manifest:
   `go run ./internal/devtools/vendor-d3-manifest.go > static/vendor/prism/d3/VERSIONS.json`.
4. Verify the digests:
   `go test ./internal/devtools/... -run TestPrismD3VendoredPinned`.
5. Run the cross-impl harness (`PRISM_CROSS_IMPL=1 go test
   ./internal/devtools/... -run TestCrossImplSVGParity`) to catch
   any breaking changes in axis / scale / shape behaviour.
6. Commit the new `.mjs` files + the updated `VERSIONS.json` +
   the bumped version tables in one PR. Include the upstream
   release notes in the PR description.

## Why not npm?

The whole point of the no-build-pipeline policy
(`design/08-browser.md`) is that what's committed is what runs.
`npm install` introduces a moving target (transitive deps,
lockfile drift, install-time scripts) inside the rendering
hot-path. Vendoring + sha256 pinning gives us byte-identical
reproducibility across machines, CI, and air-gapped deployments.
