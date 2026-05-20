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

// Tree carries the vendored ESM browser surface (the four .mjs
// files plus README). `prism.wasm` and `wasm_exec.js` are NOT
// embedded — they are produced by `make build-wasm`, can be
// dropped into static/vendor/prism/ at docs-build time for mdBook
// to pick up, and shipped by `prism static-bundle --wasm` from
// their build artefact locations. Keeping them out of the embed
// FS prevents the host CLI binary from inflating by ~70 MiB.
//
//go:embed vendor/prism/*.mjs vendor/prism/*.md
var Tree embed.FS

// BundleRoot is the directory inside Tree that maps to the
// distributable bundle. `prism static-bundle <out>` extracts every
// file under this prefix and writes it (preserving relative paths)
// into <out>.
const BundleRoot = "vendor/prism"
