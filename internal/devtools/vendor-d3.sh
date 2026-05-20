#!/usr/bin/env bash
# vendor-d3.sh — fetch pinned D3 ESM modules from jsdelivr and
# write them under static/vendor/prism/d3/.
#
# Run once locally; the resulting .mjs files are committed verbatim
# per D070. Update protocol:
#   1. Bump the version table below.
#   2. Re-run this script.
#   3. Regenerate the manifest: `go run ./internal/devtools/vendor-d3-manifest.go`
#   4. Verify: `go test ./internal/devtools/... -run TestPrismD3VendoredPinned`
#   5. Commit + open PR.
#
# Auto-updates are forbidden; this script never runs in CI.

set -euo pipefail

DEST="$(cd "$(dirname "$0")/../../static/vendor/prism/d3" && pwd)"

# Module name → version. Mirror design/08-browser.md + D070.
declare -a MODS=(
  "d3-array=3.2.4"
  "d3-axis=3.0.0"
  "d3-brush=3.0.0"
  "d3-format=3.1.0"
  "d3-scale=4.0.2"
  "d3-shape=3.2.0"
  "d3-time-format=4.1.0"
  "d3-zoom=3.0.0"
)

for entry in "${MODS[@]}"; do
  name="${entry%%=*}"
  ver="${entry##*=}"
  url="https://cdn.jsdelivr.net/npm/${name}@${ver}/+esm"
  out="${DEST}/${name}.mjs"
  curl -fsSL --max-time 30 "$url" -o "$out"
  echo "fetched ${name}@${ver} -> ${out} ($(wc -c <"$out" | tr -d ' ') bytes)"
done

echo
echo "Done. Next steps:"
echo "  go run ./internal/devtools/vendor-d3-manifest.go > static/vendor/prism/d3/VERSIONS.json"
echo "  go test ./internal/devtools/... -run TestPrismD3VendoredPinned"
