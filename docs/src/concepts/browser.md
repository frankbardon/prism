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
├── prism.wasm           # cmd/prismwasm binary (GOOS=js GOARCH=wasm); ~14.5 MiB raw (Go) / ~6.9 MiB (TinyGo)
├── prism.wasm.gz        # gzipped binary (~3.5 MiB Go / ~2.2 MiB TinyGo) — what the loader fetches
├── wasm_exec.js         # toolchain-pinned WASM loader (matches the build toolchain)
├── prism.mjs            # thin bootstrapper + SceneHandle facade
├── prism-element.mjs    # <prism-chart> / <prism-dataset> / <prism-coordinator>
├── prism-resolver.mjs   # page-level dataset registry
├── prism-selection.mjs  # selection state + DOM event wiring
└── index.html           # minimal loader example
```

### Two build toolchains

Prism's WASM module builds two ways, and both write the same
`bin/prism.wasm` + `bin/wasm_exec.js` (paired loader — never mix a
binary from one toolchain with the loader from the other):

| Build | Command | Raw | Gzipped | Loader |
|---|---|---|---|---|
| Standard Go | `make build-wasm` | ~14.5 MiB | ~3.5 MiB | Go's `wasm_exec.js` |
| **TinyGo** (recommended) | `make build-wasm-tinygo` | ~6.9 MiB (7,239,767 B) | **~2.2 MiB (2,232,605 B)** | TinyGo's `wasm_exec.js` |

The **TinyGo build is the recommended browser artifact.** It links a
leaner runtime and GC, roughly halving both the raw and gzipped size
while producing byte-identical SVG to the standard-Go build (E5-S2
verified 16/16 parity). The standard-Go build stays fully supported
as the fallback for environments where TinyGo is unavailable, and it
remains the default `make build-wasm` target so `make build` needs no
extra toolchain. TinyGo 0.41.1+ is required for the TinyGo target
(`brew tap tinygo-org/tools && brew install tinygo`); it uses
`-stack-size=8MB` so the JSON-Schema shape validator's recursion does
not trap.

### Wire size and the raw/gzip gap

The WASM module is larger uncompressed than on the wire; the size you
actually pay depends entirely on compression:

- **The `prism static-bundle --wasm` bundle ships both `prism.wasm`
  and `prism.wasm.gz`.** The standalone loader fetches the `.gz` and
  decompresses it in-page via `DecompressionStream("gzip")`, so the
  gzipped payload is what crosses the wire even on a dumb static host
  that does no content-negotiation. The raw `prism.wasm` stays as a
  fallback (`WebAssembly.instantiateStreaming`) for environments
  without `DecompressionStream` or where the `.gz` is absent.
- **If you wire up your own loader**, either fetch `prism.wasm.gz`
  and decompress as above, or serve `prism.wasm` with
  `Content-Encoding: gzip`/`br` so the browser decompresses
  transparently. Do **not** serve the raw `prism.wasm` uncompressed.
  (nginx: add `application/wasm` to `gzip_types`; most CDNs negotiate
  automatically but some skip files over a size cap.)

### CI size gates

The standard-Go artifact is guarded by
`internal/gates/wasm_size_test.go`, which checks **both** the gzipped
size (`PRISM_WASM_MAX_BYTES`, 16 MiB) and the raw size
(`PRISM_WASM_RAW_MAX_BYTES`, 80 MiB) so the uncompressed artifact
cannot balloon unnoticed behind the gzipped check.

The TinyGo artifact has its own, much tighter gate in
`internal/gates/wasm_tinygo_size_test.go`
(`PRISM_WASM_TINYGO_MAX_BYTES`, 4 MiB gzipped;
`PRISM_WASM_TINYGO_RAW_MAX_BYTES`, 12 MiB raw). Because it needs the
TinyGo toolchain, the gate **skips cleanly when `tinygo` is not on
`PATH`** (default CI lanes stay green) and **runs when it is
present** — building a fresh TinyGo module into a temp directory so
the measurement is independent of whatever last populated `bin/`.

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

The spec carries its own rows: inline `data: {values: [...]}` /
`datasets.*.values`, or a `datasets` attribute on the element. For
lazy or large data, register a JS `DataResolver` via
`prism.setDataResolver(...)` and reference it with `data: {ref}`. Prism
never fetches or decodes a `.pulse` file in the browser — the host
materializes the rows and hands them to Prism. WASM then runs the full
pipeline and mounts the resulting SVG.

### Server compile (opt-in)

Hosts that prefer to offload the compile stage to a trusted backend
add a `compile-server` attribute:

```html
<prism-chart spec="/specs/brand_score.prism.json"
             compile-server="/prism/scene"></prism-chart>
```

The browser POSTs the spec + dataset map to the server (`prism
serve` Twirp endpoint from P14) and gets back the resolved Scene
IR. WASM still does the final SVG render; the network round-trip
only covers compile.

## Compile-only mode

Callers (particularly programmatic ones constructing specs from
logic) can ask Prism "what would this render produce?" without
paying the cost of rasterising. The WASM module exposes a
`compile` export that returns the structured `CompiledPlan` —
the same intermediate representation the render stage consumes,
just exposed publicly:

```js
const planJSON = globalThis.prism.compile(specJSON, datasetsJSON, optsJSON);
const plan = JSON.parse(planJSON);
// plan.marks         — flattened mark summary (per layer)
// plan.scales        — resolved scales (channel, type, domain, range)
// plan.data          — dataset bindings (named + resolved)
// plan.layout        — width/height + grid rows/cols
// plan.diagnostics   — PRISM_WARN_* warnings
// plan.scene         — full Scene IR (same as `prism.execute` output)
```

Cost is dominated by aggregation over the materialized rows (the
executor); the flattened plan view itself is light. For specs whose data fits
in memory, compile-only typically runs 10–50× faster than a
full `prism.execute` + `prism.render` pair, since the encode +
SVG-emit stages are skipped.

The Go-native API exposes the same surface:

```go
plan, err := prism.Compile(ctx, spec, prism.CompileOptions{})
```

Use cases:

- **Programmatic introspection** — verify that the color
  encoding bound the field you expected.
- **Plan diffing** — compare two CompiledPlans to know what
  changed between spec edits without rendering both.
- **Pre-render previews** — show the user "3 marks across
  2 facets" before committing to a render.

## Fetch-backed assets

Prism never fetches or decodes data rows in the browser — the host
supplies them inline (`data.values` / `datasets.*.values`) or through a
JS `DataResolver` registered with `prism.setDataResolver(...)`. The only
assets Prism itself fetches are **geodata tiers** (`geoshape` / `geopoint`
marks), pulled from `${origin}/static/prism/geodata/` (override via
`prism.geo.setBundleURL(url)`), and any URL-referenced Scene JSON the
page loads directly. Those `GET`s go through a fetch adapter that
dedupes by URL and buffers the body for the page lifetime.

A failed asset fetch surfaces as `PRISM_WASM_001` (CORS, network, or
non-2xx). It arrives in the JS bridge as a standard `{ok:false, error}`
envelope; `prism.mjs` rethrows it as an `Error` with `prismCode` +
`prismFixups` attached.

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
emitted Scene IR (`scene.animation` + `mark.key`). The SVG
renderer ignores these fields entirely; only the web component and
the WASM runtime tween between successive scenes.

### How the animator works

When `<prism-chart>`'s `spec` or `src` attribute changes and the new
scene declares an `animation` block, the element holds the previous
`SceneHandle` alive and calls `handle.update(newSceneDoc)` instead of
the default clear-and-replace path.

`SceneHandle.update` defers to `PrismAnimator` (vendored in
`static/vendor/prism/prism-animator.mjs`):

1. The new scene is rendered through the WASM module into a detached
   SVG; its `visibility` is set to `hidden` so the user keeps seeing
   the live (previous) SVG.
2. `PrismAnimator` indexes both SVGs by `data-prism-mark-key` and
   partitions marks into **enter / update / exit** sets.
3. A `requestAnimationFrame` loop interpolates numeric attrs
   (`x`/`y`/`width`/`height`/`cx`/`cy`/`r`/`opacity`/...) on the
   **live** SVG, writing target values read from the staged SVG.
   Color attrs (`fill`, `stroke`) interpolate through OKLab via
   `oklab.mjs` for perceptually smooth transitions.
4. At `t = 1` the previous SVG is removed and the staged SVG becomes
   visible. The exit set fades to `opacity=0` along the way.

### Fallbacks

The animator skips and snaps to the new scene when any of the
following hold:

- `prefers-reduced-motion: reduce` is set by the OS / browser.
  (Silent — this is the correct UX, not a failure.)
- The previous scene is structurally incompatible with the new
  scene (different layer count, different mark family per layer,
  different axis count). `SceneHandle` dispatches a `prism:warn`
  CustomEvent carrying `{code: "PRISM_WARN_ANIM_FALLBACK", message}`
  on its root (the shadow root inside `<prism-chart>`, otherwise the
  host element). The event bubbles + composes through the shadow
  boundary so listeners on the host page receive it without extra
  plumbing.
- The `animate` option is explicitly `false`
  (`handle.update(doc, { animate: false })`). (Silent.)
- The previous handle does not exist yet (first render). (Silent.)

Listening for the warning:

```js
chart.addEventListener("prism:warn", (e) => {
  if (e.detail.code === "PRISM_WARN_ANIM_FALLBACK") {
    console.warn(`tween skipped: ${e.detail.message}`);
  }
});
```

### Public exports

`prism.mjs` re-exports the animator surface so embedders can drive a
tween on a bare SVG without going through `SceneHandle`:

```js
import {
  PrismAnimator,
  structurallyCompatible,
  prefersReducedMotion,
} from "/static/vendor/prism/prism.mjs";
```

The tween engine has zero dependencies beyond `oklab.mjs`. The WASM
binary size is unaffected — animation lives entirely in plain JS.

### Where to see it

- The [interactive playground](../playground/index.html) routes every
  edit through `SceneHandle.update()`. Pick the **Animation › Swap
  bars** example and change any score: the bars tween instead of
  snapping.
- The [`gallery/animation/`](../gallery/index.md#animation) entries
  ship spec + initial-frame SVG; live `<prism-chart>` cards on the
  gallery [`index.html`](../gallery/index.html) demonstrate the tween
  when the scene-doc swaps.

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

### TinyGo ↔ standard-Go float parity

Prism ships a second WASM build path (`make build-wasm-tinygo`)
that produces a much smaller module. TinyGo links its own
`strconv`, and **every** SVG coordinate funnels through the single
`render.FormatFloat` helper (`render/precision.go`, pinned to 3
decimals). If TinyGo rounded or stringified floats differently
from standard Go, the coordinate goldens would drift — this was
flagged as the highest risk of the TinyGo migration.

It does not drift. A dedicated parity harness proves it:

```bash
PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGo
```

- `TestTinyGoWasmSVGParity` builds a TinyGo `js/wasm` module from
  `cmd/prismwasm`, renders a float-diverse fixture corpus (bars,
  curves, trigonometric arcs, bezier ribbons, dense rect/box/violin
  layouts) under Node with TinyGo's paired `wasm_exec.js`, and
  diffs each SVG byte-for-byte against the committed standard-Go
  `go.svg`. All fixtures are byte-identical.
- `TestTinyGoFloatFormatParity` drives `render.FormatFloat` over an
  edge-case corpus (half-way rounding, trailing-zero trimming,
  negative zero, magnitude extremes, `NaN`/`±Inf`) in three builds —
  host-native, standard-Go wasm, and TinyGo wasm — and asserts all
  three agree. The host-side pin lives in
  `render/precision_test.go`.

Because parity holds unmodified, **no float-emission change was
needed**: standard-Go and TinyGo already produce identical bytes.
The harness is opt-in (mirroring `PRISM_CROSS_IMPL`) because it
needs both `node` and `tinygo` on `PATH`.

## Standalone HTML demo

`prism static-bundle --wasm ./public/prism` writes a working
`index.html` to the output directory. Open it directly with a
local static server (the browser refuses `file://` for WASM):

```bash
prism static-bundle --wasm ./public/prism
cd ./public/prism && python -m http.server 8000
# → open http://localhost:8000/
```

The demo fetches `prism.wasm.gz`, decompresses it in-page via
`DecompressionStream("gzip")` (falling back to the raw `prism.wasm`
when unavailable), then renders any `<prism-chart>` it finds.
Replace the bundled `index.html` with your own page to embed Prism
in mdBook, Astro, Hugo, or any other static-site generator — keep
the `.gz` + `DecompressionStream` pattern (or serve the raw `.wasm`
with `Content-Encoding`) so you ship ~12 MiB, not ~69 MiB.
