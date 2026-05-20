// Package staticfs exposes the vendored Prism browser assets
// (prism.mjs, prism-element.mjs, prism-resolver.mjs,
// prism-selection.mjs, d3/*.mjs, the README files) as an embed.FS
// so commands compiled from cmd/prism can extract or serve them
// without depending on the repo layout at runtime.
//
// Lives at the repo root next to the embedded tree so the go:embed
// pattern resolves cleanly (Go forbids embed paths from traversing
// upward via "..").
package staticfs

import "embed"

// Tree is the full vendored static tree (everything under
// static/vendor/prism/, recursively). The `all:` prefix includes
// directories whose names begin with "_" or "." per the embed
// docs — defensive in case a vendored file ever uses one.
//
//go:embed all:vendor/prism
var Tree embed.FS

// BundleRoot is the directory inside Tree that maps to the
// distributable bundle. `prism static-bundle <out>` extracts every
// file under this prefix and writes it (preserving relative paths)
// into <out>.
const BundleRoot = "vendor/prism"
