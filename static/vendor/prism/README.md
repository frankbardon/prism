# Prism browser vendored assets

Pure-ESM browser renderer + web component. No build pipeline. Every
`.mjs` file in this directory runs unmodified in browsers — what you
see committed is what the browser loads.

## File inventory

| File                  | Purpose                                                |
| --------------------- | ------------------------------------------------------ |
| `prism.mjs`           | SceneDoc → SVG renderer, `SceneHandle` class, `render` / `validate` exports. |
| `prism-element.mjs`   | `<prism-chart>` + `<prism-dataset>` custom elements, shadow-DOM lifecycle. |
| `prism-resolver.mjs`  | Page-level dataset registry (`register` / `unregister` / `fetch` with dedupe). |
| `prism-selection.mjs` | Selection state shape + `broadcast` / `listen` helpers. **Stub in P12**: `getSelection` returns null; full hit-testing wiring lands in P13. |
| `d3/`                 | Vendored D3 ESM modules (D070). See `d3/README.md` for the update protocol. |

## Public API surface

```js
import { render, validate, SceneHandle } from "./prism.mjs";
import { PrismResolver } from "./prism-resolver.mjs";
// Custom elements auto-register on import:
import "./prism-element.mjs";
```

```html
<script type="module" src="/static/vendor/prism/prism-element.mjs"></script>

<prism-dataset name="current" src="/data/q1.pulse"></prism-dataset>
<prism-chart src="/scenes/brand_score.json"></prism-chart>
```

## SceneDoc JSON contract

The browser renderer consumes the same Scene IR JSON that
`prism scene <spec>` emits on the Go side (see
`.planning/design/06-scene-ir.md`). Byte-equality between Go SVG
and JS SVG is enforced via `TestCrossImplSVGParity` (D076) on a
curated fixture set.

## No-build-pipeline policy

- No `package.json` in this directory (the only one in the repo
  lives under `internal/devtools/cross-impl-runner/` for the
  parity-harness dev dep — happy-dom).
- No webpack, vite, rollup, esbuild, or tsc.
- D3 modules are vendored ESM bundles from jsdelivr (D070); see
  `d3/README.md` for the update procedure (manual PR only — no
  auto-update).
- Imports are relative paths only (D071); no import map needed.
- Float coordinates pin to 3-decimal precision via the module-
  local `fmt(n)` helper, mirroring Go's `render.FormatFloat`
  (D072).

## Parity guarantee

Drift between Go SVG and JS SVG is a build break. The cross-impl
test gates byte-equality on five curated fixtures (`bar_basic`,
`line_basic`, `layer_actual_vs_benchmark`, `pie`,
`sankey_user_flow`). Run with:

```
PRISM_CROSS_IMPL=1 go test ./internal/devtools/... -run TestCrossImplSVGParity
```

Skips cleanly without `PRISM_CROSS_IMPL=1` or without `node` on PATH.

## Distribution

To copy these assets into a downstream deployment:

```
prism static-bundle /path/to/out
```

The `static-bundle` subcommand embeds + extracts the entire
`static/vendor/prism/` tree (including `d3/`) preserving relative
paths so the internal imports keep resolving after extraction.
