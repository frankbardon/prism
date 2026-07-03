# Prism cross-implementation parity harness

This directory holds the Node entry point + helpers that exercise the
JS port (`static/vendor/prism/prism.mjs`) against committed Scene IR
fixtures and produce SVG output diffed against the Go renderer's SVG
output (D076).

## One-time setup (local dev only)

```
cd internal/devtools/cross-impl-runner/
npm install                    # installs happy-dom
export PRISM_CROSS_IMPL=1      # unlocks the gated Go tests
```

`happy-dom` is the only npm dev dep in the repo. `node_modules/` +
`package-lock.json` are `.gitignore`-d — never commit them.

## Run the parity gate

```
PRISM_CROSS_IMPL=1 go test ./internal/devtools/... -run TestCrossImplSVGParity -v
```

Skips cleanly when `PRISM_CROSS_IMPL=1` is not set or when `node` is
not on PATH (CI without node deps stays green).

## Refresh fixtures

After a Scene IR change (encoder bugfix, new mark type, etc.) the
committed `testdata/cross_impl/<fixture>/scene.json` + `go.svg`
files need refreshing:

```
PRISM_CROSS_IMPL=1 PRISM_CROSS_IMPL_REGEN=1 \
  go test ./internal/devtools/... -run TestCrossImplSVGParity
```

The regen step shells out to `bin/prism scene` + `bin/prism plot`
for each curated fixture, overwriting the inputs. Re-run the test
without REGEN to confirm parity, commit the refreshed files.

## Curated fixtures

The harness gates the following Scene IR fixtures (P12 launch set):

| Fixture                         | Marks       | Notes                              |
| ------------------------------- | ----------- | ---------------------------------- |
| `bar_basic`                     | rect        | Simplest single-layer chart        |
| `line_basic`                    | line        | Polyline points                    |
| `layer_actual_vs_benchmark`     | rect + line | Two-layer composition              |
| `pie`                           | arc         | Donut-less pie sectors             |
| `sankey_user_flow`              | rect + path | Composite layout + cubic beziers   |

Adding a fixture: drop a new directory under `testdata/cross_impl/`,
add the fixture name to the `curatedFixtures` slice in
`cross_impl_test.go`, run with `PRISM_CROSS_IMPL_REGEN=1` to
populate `scene.json` + `go.svg`, then re-run without REGEN to
confirm JS parity.

## TinyGo ↔ standard-Go float parity

`make build-wasm-tinygo` produces a smaller WASM module built by TinyGo
(its own `strconv`). Because every SVG coordinate funnels through the
single `render.FormatFloat` helper (`render/precision.go`, pinned to 3
decimals), TinyGo's float stringification must match standard Go's
byte-for-byte or the coordinate goldens drift — the top risk of the
TinyGo migration. `tinygo_parity_test.go` proves it does not:

```
PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGo -v
```

- `TestTinyGoWasmSVGParity` — builds `cmd/prismwasm` with TinyGo,
  renders a float-diverse fixture corpus under Node via `probe-runner.mjs`
  (with TinyGo's paired `wasm_exec.js`), and diffs each SVG against the
  committed standard-Go `go.svg`.
- `TestTinyGoFloatFormatParity` — drives `render.FormatFloat` over an
  edge-case corpus (`floatcorpus/`) in host / Go-wasm / TinyGo-wasm and
  asserts all three agree.

Skips cleanly unless `PRISM_CROSS_IMPL_TINYGO=1` and both `node` and
`tinygo` are on PATH.

## Files

| File                              | Purpose                                          |
| --------------------------------- | ------------------------------------------------ |
| `package.json`                    | One npm dep declaration (happy-dom).             |
| `main.mjs`                        | Runs `bin/prism.wasm` against scene.json → wasm.svg. |
| `probe-runner.mjs`                | Parametric wasm driver (any wasm + paired exec); render or read-global mode. Used by the TinyGo parity harness. |
| `web-component-lifecycle.mjs`     | Asserts connect/disconnect/re-render cycle.      |
| `dataset-registry-dedupe.mjs`     | Asserts fetch memoisation by URL.                |
| `README.md`                       | This file.                                       |
