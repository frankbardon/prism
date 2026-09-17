#!/usr/bin/env bash
# Build docs/versions.json from the version directories that actually
# exist in a staged site tree.
#
# Usage: build-versions-manifest.sh <site-dir>
#
# Output: JSON on stdout. The deploy workflow stages this at
# /prism/versions.json so the version-dropdown.js can fetch it from
# any /prism/<version>/ page.
#
# The manifest lists a version ONLY when <site-dir>/<version>/ is
# present. Listing every git tag instead used to advertise 20 versions
# while the site carried one, so every dropdown entry 404'd — the
# manifest must describe what is published, not what has been tagged.
# Deploys accumulate into a gh-pages storage branch (see
# .github/workflows/docs.yml), so a tag joins the list when its build
# lands and stays there.
#
# Membership AND order come from the directory listing alone — no git
# invocation. The workflow runs this from two different working
# directories (the repo checkout and the gh-pages clone, which is
# shallow and carries no tags), and a git-derived order silently
# dropped every version in the second case.

set -euo pipefail

site_dir=${1:-}
if [[ -z "$site_dir" ]]; then
  echo "usage: $(basename "$0") <site-dir>" >&2
  exit 2
fi
if [[ ! -d "$site_dir" ]]; then
  echo "$(basename "$0"): no such directory: $site_dir" >&2
  exit 2
fi

# Released versions, newest first. `sort -Vr` orders v0.16.0 above
# v0.9.0, which a lexicographic sort does not.
versions=$(
  find "$site_dir" -mindepth 1 -maxdepth 1 -type d -name 'v[0-9]*.[0-9]*.[0-9]*' \
    -exec basename {} \; 2>/dev/null | sort -Vr || true
)

echo '{'
echo '  "default": "latest",'
echo '  "versions": ['
first=1
emit() {
  local id=$1 label=$2
  if [[ $first -eq 1 ]]; then
    printf '    {"id": "%s", "label": "%s", "path": "/prism/%s/"}\n' "$id" "$label" "$id"
    first=0
  else
    printf '    ,{"id": "%s", "label": "%s", "path": "/prism/%s/"}\n' "$id" "$label" "$id"
  fi
}

if [[ -d "$site_dir/latest" ]]; then
  emit "latest" "latest (dev)"
fi
for v in $versions; do
  emit "$v" "$v"
done
echo '  ]'
echo '}'
