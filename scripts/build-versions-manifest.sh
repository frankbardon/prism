#!/usr/bin/env bash
# Build docs/versions.json from `latest` + every v*.*.* git tag.
#
# Output: JSON on stdout. The deploy workflow stages this at
# /prism/versions.json so the version-dropdown.js can fetch it from
# any /prism/<version>/ page.

set -euo pipefail

tags=$(git tag --list 'v*.*.*' --sort=-v:refname || true)

echo '{'
echo '  "default": "latest",'
echo '  "versions": ['
echo '    {"id": "latest", "label": "latest (dev)", "path": "/prism/latest/"}'
for tag in $tags; do
  printf '    ,{"id": "%s", "label": "%s", "path": "/prism/%s/"}\n' "$tag" "$tag" "$tag"
done
echo '  ]'
echo '}'
