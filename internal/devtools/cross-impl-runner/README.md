# Prism cross-implementation parity harness

This directory holds the Node entry points + helpers that load the
Prism WASM module (TinyGo-built — the sole wasm target) and exercise it
against committed Scene IR fixtures, diffing its SVG output against the
host Go renderer's output and driving the selection/web-component
behaviours under `happy-dom`.

## One-time setup (local dev only)

```
cd internal/devtools/cross-impl-runner/
npm install                    # installs happy-dom
export PRISM_CROSS_IMPL=1      # unlocks the gated selection/web-component tests
```

`happy-dom` is the only npm dev dep in the repo. `node_modules/` +
`package-lock.json` are `.gitignore`-d — never commit them.

The `PRISM_CROSS_IMPL=1` gate unlocks the happy-dom selection +
web-component harnesses (`web_component_test.go`, `selection_*_test.go`),
which load `bin/prism.wasm` + `bin/wasm_exec.js` (TinyGo build). Run
`make build-wasm-tinygo` first so those artifacts exist.

## TinyGo ↔ host float / SVG parity

`make build-wasm-tinygo` produces a WASM module built by TinyGo (its own
`strconv`). TinyGo is now the only wasm build — the standard-Go js/wasm
target was retired — so host↔TinyGo is the single cross-toolchain parity
axis. Because every SVG coordinate funnels through the single
`render.FormatFloat` helper (`render/precision.go`, pinned to 3
decimals), TinyGo's float stringification must match the host Go build's
byte-for-byte or the coordinate goldens drift — the top risk of the
TinyGo migration. `tinygo_parity_test.go` proves it does not:

```
PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGo -v
```

- `TestTinyGoWasmSVGParity` — builds `cmd/prismwasm` with TinyGo,
  renders a float-diverse fixture corpus under Node via `probe-runner.mjs`
  (with TinyGo's paired `wasm_exec.js`), and diffs each SVG against the
  committed `go.svg` (the host-Go reference, produced by `prism plot` and
  auto-bootstrapped on first run for a new fixture).
- `TestTinyGoFloatFormatParity` — drives `render.FormatFloat` over an
  edge-case corpus (`floatcorpus/`) in host and TinyGo-wasm builds and
  asserts they agree byte-for-byte.

Skips cleanly unless `PRISM_CROSS_IMPL_TINYGO=1` and both `node` and
`tinygo` are on PATH.

The fixture corpus lives under `testdata/cross_impl/<fixture>/`
(`scene.json` + `go.svg`); the fixture names are the
`tinyGoParityFixtures` slice in `tinygo_parity_test.go`. Adding a fixture:
drop the spec under `examples/specs/`, append its name to that slice, and
run the test — the host-side `scene.json` + `go.svg` self-bootstrap from
`bin/prism`.

## TinyGo end-to-end render smoke

Float parity proves TinyGo stringifies coordinates identically; it does
not prove the TinyGo binary renders a chart *from a spec*. That is the
job of `tinygo_render_test.go` (`TestTinyGoWasmRenderPipeline`), which
drives the full exported pipeline — `validate` → `plan` → `execute`
(in-memory transforms over inline `data.values` or a JS-supplied
`DataResolver`) → `render` — through the TinyGo `globalThis.prism`
surface via `probe-runner.mjs pipeline`, and asserts each fixture yields
a non-empty, well-formed `<svg>` with the expected mark geometry.

```
PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGoWasmRenderPipeline -v
```

Fixtures live under `testdata/tinygo_render/`. They span the transform
families this closes end-to-end: structured `filter` + `calculate` (E2),
the previously-Pulse `crosstab` and `regression` (E3, now pure-Go), and a
`data: {ref}` spec whose rows arrive through `prism.setDataResolver`. A
`<name>.sidecar.json` supplies the optional `{"datasets": …,
"resolver": …}` pair (a `crosstab`/`regression` chain needs a
source-rooted input, reached here by a `datasets`-registered `data:
{name}`). Assertions are structural, not byte goldens — byte parity is
already covered above. Same opt-in gate as the parity tests.

## Files

| File                              | Purpose                                          |
| --------------------------------- | ------------------------------------------------ |
| `package.json`                    | One npm dep declaration (happy-dom).             |
| `probe-runner.mjs`                | Parametric wasm driver (any wasm + paired exec); `render`, `global`, or `pipeline` (full validate→plan→execute→render) mode. Used by the TinyGo parity + render smoke. |
| `web-component-lifecycle.mjs`     | Asserts connect/disconnect/re-render cycle.      |
| `dataset-registry-dedupe.mjs`     | Asserts fetch memoisation by URL.                |
| `README.md`                       | This file.                                       |
