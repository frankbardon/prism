# Browser

Prism runs end-to-end in the browser via a single `prism.wasm`
artifact. The full six-stage pipeline (spec → validate → plan →
compile → encode → render) executes client-side; no server round
trip is required to produce SVG from a spec.

## What ships

`prism static-bundle --wasm <out-dir>` writes a self-contained
bundle:

```
<out-dir>/
├── prism.wasm           # cmd/prismwasm binary (GOOS=js GOARCH=wasm)
├── wasm_exec.js         # toolchain-pinned WASM loader (Go runtime)
├── prism.mjs            # thin bootstrapper + SceneHandle facade
├── prism-element.mjs    # <prism-chart> / <prism-dataset> / <prism-coordinator>
├── prism-resolver.mjs   # page-level dataset registry
├── prism-selection.mjs  # selection state + DOM event wiring
└── index.html           # minimal loader example
```

Total wire size at v1: ~12 MiB gzipped (`prism.wasm`) + ~17 KiB
(`wasm_exec.js`) + ~10 KiB (the four `.mjs` files). A static host
that serves the directory with `Content-Type: application/wasm`
gets streaming instantiation; everything else falls back to the
buffered `WebAssembly.instantiate` path automatically.

## Load modes

Three ways to put a chart on a page, each compatible with the
others on the same page:

### Server-rendered scene (zero client compile)

The host emits Scene IR JSON server-side (via `prism scene`) and
references it from a `<prism-chart src=…>`:

```html
<prism-chart src="/scenes/brand_score.json"></prism-chart>
```

Fastest path. The browser fetches the JSON and renders it via
WASM. No spec parsing or transform execution in the browser.

### Client spec compile (WASM default)

The host passes the spec inline or as a URL on the `spec`
attribute:

```html
<prism-chart spec='{"$schema":"urn:prism:schema:v1:spec",...}'></prism-chart>
<prism-chart spec="/specs/brand_score.prism.json"></prism-chart>
```

The browser fetches any referenced `.pulse` files via the
[fetch-backed afero.Fs](#fetch-backed-fs), runs the full pipeline
in WASM, and mounts the resulting SVG. Inline data
(`data: {values: [...]}`) skips the fetch path entirely.

### Server compile (opt-in)

Hosts that prefer to keep Pulse loading behind a trusted backend
add a `compile-server` attribute:

```html
<prism-chart spec="/specs/brand_score.prism.json"
             compile-server="/prism/scene"></prism-chart>
```

The browser POSTs the spec + dataset map to the server (`prism
serve` Twirp endpoint from P14) and gets back the resolved Scene
IR. WASM still does the final SVG render; the network round-trip
only covers compile.

## Fetch-backed Fs

Dataset references resolve through an `afero.Fs` adapter backed
by browser `fetch`. The first access to a `.pulse` URL issues a
`GET` and buffers the body in memory; subsequent opens reuse the
cached bytes for the page lifetime.

Inline data and short single-shard cohorts work out of the box.
Archive shard references (`archive.pulse#shard.pulse`) work when
the origin serves the archive in one response — random-access
range reads are a v2 enhancement (see
[BACKLOG.md](https://github.com/frankbardon/prism/blob/main/.planning/BACKLOG.md)).

Two error codes surface fetch problems:

- `PRISM_WASM_001` — fetch failure (CORS, network, non-2xx).
- `PRISM_WASM_002` — origin server rejects `Range:` requests
  (only matters once range support lands).

Both arrive in the JS bridge as standard `{ok:false, error}`
envelopes; `prism.mjs` rethrows them as `Error` instances with
`prismCode` + `prismFixups` attached.

## What's still in JS

The four `.mjs` files together total ~10 KiB. They handle the
DOM-side work that WASM can't reach across the bridge cheaply:

| File | Responsibility |
|---|---|
| `prism.mjs` | Load WASM, marshal JSON, mount SVG, expose `SceneHandle` |
| `prism-element.mjs` | `<prism-chart>` / `<prism-dataset>` / `<prism-coordinator>` custom elements |
| `prism-resolver.mjs` | Page-level dataset registry; dedupes fetches across charts |
| `prism-selection.mjs` | Pointer-event hit testing against `data-prism-*` attrs; URL-hash persistence |

JS-side scale resolution, axis layout, tick generation, palette
resolution, and number/time format are all gone — they used to
exist as a reimplementation of the Go pipeline in `prism.mjs` and
were deleted in P17 once the WASM path landed. There is one
implementation of every Prism stage now, written in Go.

## Animation

The spec [`animation`](spec.md#animation) block produces hints in the
emitted Scene IR (`scene.animation` + `mark.key`). SVG and PDF
renderers ignore these fields entirely; only the web component and
the WASM runtime tween between successive scenes.

Animation lives in JS, not Go — the WASM module emits the scene
hints, and the in-page JS animator (shipped in a future PR alongside
`prism-animator.mjs`) diffs old vs. new scene by `mark.key` and
interpolates per-attr via `requestAnimationFrame`. `prefers-reduced-motion: reduce`
disables the animator at the OS / browser level.

## Cross-implementation parity

The cross-impl harness (`internal/devtools/cross-impl-runner/`)
asserts byte-equal SVG between the host-native Go renderer and
the Go-compiled WASM module. Drift signals a non-deterministic
stage or a Go toolchain regression — not a JS port mistake.

Run locally:

```bash
make build-wasm
PRISM_CROSS_IMPL=1 go test ./internal/devtools/
```

The runner needs `node` on `PATH`; no `npm install` is required.

## Standalone HTML demo

`prism static-bundle --wasm ./public/prism` writes a working
`index.html` to the output directory. Open it directly with a
local static server (the browser refuses `file://` for WASM):

```bash
prism static-bundle --wasm ./public/prism
cd ./public/prism && python -m http.server 8000
# → open http://localhost:8000/
```

The demo loads `prism.wasm`, then renders any `<prism-chart>` it
finds. Replace the bundled `index.html` with your own page to
embed Prism in mdBook, Astro, Hugo, or any other static-site
generator.
