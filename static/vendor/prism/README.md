# Prism browser vendored assets

Thin ESM shim over `prism.wasm`. No build pipeline. Every `.mjs`
file in this directory runs unmodified in browsers — what you see
committed is what the browser loads.

## File inventory

| File                  | Purpose                                                |
| --------------------- | ------------------------------------------------------ |
| `prism.mjs`           | WASM bootstrapper. Loads `prism.wasm` lazily, marshals JSON across the syscall/js bridge, mounts the Go-rendered SVG into the DOM. Exports `render`, `validate`, `executeSpec`, `SceneHandle`, `fmt`, `ensureWasmReady`. |
| `prism-element.mjs`   | `<prism-chart>` + `<prism-dataset>` + `<prism-coordinator>` custom elements, shadow-DOM lifecycle, URL-hash state persistence. |
| `prism-resolver.mjs`  | Page-level dataset registry (`register` / `unregister` / `fetch` with dedupe). |
| `prism-selection.mjs` | Pointer-event hit testing against `data-prism-*` attrs; `broadcast` / `listen` helpers for cross-chart coordination. |

`prism.wasm` and `wasm_exec.js` are produced by `make build-wasm`
and shipped alongside this tree by `prism static-bundle --wasm`.

## Public API surface

```js
import { render, validate, executeSpec, SceneHandle } from "./prism.mjs";
import { PrismResolver } from "./prism-resolver.mjs";
// Custom elements auto-register on import:
import "./prism-element.mjs";
```

```html
<script src="/static/vendor/prism/wasm_exec.js"></script>
<script type="module" src="/static/vendor/prism/prism-element.mjs"></script>

<prism-dataset name="current" src="/data/q1.pulse"></prism-dataset>
<prism-chart spec="/specs/brand_score.prism.json"></prism-chart>
```

`render(sceneDoc, target)` and `executeSpec(spec, datasets?, opts?)`
both return Promises — the WASM module lazy-loads on first use.

## Architecture

The browser pipeline runs in WASM. The compiled `prism.wasm`
exposes `prism.validate / prism.plan / prism.execute / prism.render /
prism.errorsLookup / prism.schemaBundle / prism.version` on
`globalThis` (see `cmd/prismwasm/main.go`). The `.mjs` files in
this directory handle only the DOM-side work that's awkward to do
through the syscall/js bridge: custom-element lifecycle, dataset
registry, pointer-event wiring against the rendered SVG, and
URL-hash state persistence.

The P12 JS-port renderer (scale resolution, axis layout, tick
generation, palette resolution, format helpers) was deleted in
P17 once the WASM path landed. One implementation of every Prism
stage now exists, written in Go.

## No-build-pipeline policy

- No `package.json` in this directory (the only one in the repo
  lives under `internal/devtools/cross-impl-runner/` for the
  parity-harness dev dep — happy-dom).
- No webpack, vite, rollup, esbuild, or tsc.
- Imports are relative paths only (D071); no import map needed.

## Parity guarantee

Drift between Go-native SVG and Go-via-WASM SVG is a build break.
The cross-impl test (`internal/devtools/cross_impl_test.go`)
gates byte-equality on five curated fixtures (`bar_basic`,
`line_basic`, `layer_actual_vs_benchmark`, `pie`,
`sankey_user_flow`). Run with:

```
make build-wasm
PRISM_CROSS_IMPL=1 go test ./internal/devtools/...
```

Skips cleanly without `PRISM_CROSS_IMPL=1` or without `node` on
PATH or without `bin/prism.wasm`.

## Distribution

```
prism static-bundle --wasm /path/to/out
```

`static-bundle` extracts this tree, builds `prism.wasm` (or
copies an existing `bin/prism.wasm`), copies `wasm_exec.js` from
the Go toolchain, and writes a working `index.html` loader. The
output directory is self-contained — drop it anywhere a static
server can reach it.
