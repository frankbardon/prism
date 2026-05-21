# CLAUDE.md

## Project Overview

Prism is a visualization library for `.pulse` files. Ships as a Go library (`github.com/frankbardon/prism`) and a CLI binary (`cmd/prism/`). Library is primary; CLI is a thin adapter.

**Design principles:**

- **Library-first.** Every public surface in `spec/`, `validate/`, `plan/`, `compile/`, `encode/`, `render/`, `resolve/`, `theme/`, `rpc/`, and `mcp/` is reachable as a Go API. `cmd/prism/` (host CLI) and `cmd/prismwasm/` (browser entry, `//go:build js && wasm`) never contain business logic — parse flags / marshal JSON, construct library objects, format output.
- **Six-stage pipeline.** Spec (JSON) → Validate → Plan → Compile → Encode → Render → Bytes. Each stage is independently testable, and intermediate artifacts (Plan, Scene IR, Encoded bytes) are stable JSON shapes downstream consumers can pin.
- **Vega-Lite vocabulary, snake_case keys.** Single-word terms (`mark`, `encoding`, `transform`, `layer`, `facet`, `concat`, `repeat`) match Vega-Lite verbatim. Multi-word keys are snake_case throughout (`stroke_width`, `corner_radius`, `font_size`).
- **Pulse expression syntax** in `filter` predicates and `calculate` transforms. No `datum.` prefix, no JS function calls, no Vega expression eval. One expression language, executed by Pulse.
- **No-execute predict & validate.** `validate/` reads only the spec + optional schema (no row I/O); `plan` builds the DAG without executing it; `prism inspect` reads spec + Pulse headers only. Network and filesystem I/O happen only at `plan.Execute` time.
- **Pulse relationship.** Prism depends on `github.com/frankbardon/pulse` for `.pulse` decoding, request compilation, and data ops. Pulse has no dependency on Prism. Custom cohort-analytics aliases `lift` and `share` are implemented client-side in `compile/inmem/` until Pulse upstreams them; `wmean`, `ratio`, `ci0`, `ci1` are first-class Pulse `AGG_*` ops as of Pulse v0.10.0.

## The Update Demand

Any change to Prism code, configuration, spec vocabulary, schema bundle, or public surface MUST update the corresponding doc page(s) and CLAUDE.md in the same PR.

| If you change... | You MUST also update... |
|---|---|
| A mark in `encode/marks/` | `docs/src/concepts/marks.md` + add a gallery entry under `docs/src/gallery/<family>/` if user-visible |
| An encoding channel | `docs/src/concepts/encoding.md` + `schema/v1/` JSON Schema for the channel shape |
| A transform (`filter`, `aggregate`, `bin`, `calculate`, `join`, `pivot`, `sample`, `sort`, `unpivot`, `window`) | `docs/src/concepts/spec.md` (transform section) + add a Plan node under `plan/nodes/` + add a `Spec*Transform` union variant in `spec/transform_union.go` |
| A composition operator (`layer`, `concat`, `hconcat`, `vconcat`, `facet`, `repeat`) | `docs/src/concepts/composition.md` + composite encoder under `encode/encode_composite.go` |
| A scale type | `docs/src/concepts/encoding.md` (scale section) + `encode/scale/` implementation + tick generator under `encode/ticks*.go` |
| A theme (or built-in theme value) | `docs/src/concepts/themes.md` + `theme/<name>.go` + token entry in `theme/css.go` |
| A semantic validation rule | `validate/RULES.md` + new rule file under `validate/rules/` + register in `validate/semantic.go` + new `PRISM_SPEC_NNN` row in `errors/codes.go` |
| A `PRISM_*` error code (added / removed / renamed) | `errors/codes.go` (canonical `Code`, `Message`, at least one fixup template or `SeeAlso`) + reachable via `prism errors lookup` |
| A renderer backend (SVG / PDF / Canvas) | `docs/src/concepts/themes.md` (rendering notes if visual) + `render/<backend>/` + dispatch in `render/render.go` |
| Anything reachable from `cmd/prismwasm/main.go` (WASM entry) | `docs/src/concepts/browser.md` + size-budget gate `internal/gates/wasm_size_test.go` + `cmd/prismwasm/wasm_smoke_test.go` if behaviour changes |
| A new package import in the WASM entry, OR a new file under a `!js`-gated subtree (`render/pdf/`, `rpc/`, `mcp/`, `cmd/prism/cmd_serve.go`, `cmd_mcp.go`, `cmd_static_bundle.go`, `cmd_init.go`) | Re-run `make build-wasm` locally; CI gates verify (a) the WASM entry still compiles and (b) the gzipped binary is under `PRISM_WASM_MAX_BYTES` |
| A CLI leaf (added / removed / flag added) | `cmd/prism/cmd_<name>.go` + `docs/src/getting-started.md` if user-visible + smoke test in `cmd/prism/*_smoke_test.go` |
| The schema bundle (`schema/v1/`) | `schema/embed.go` (the `//go:embed` directives) + bump bundle version if breaking + `docs/src/concepts/spec.md` (`$schema` reference) |
| A built-in dataset registry shape | `resolve/registry_dataset.go` + `docs/src/concepts/multi-source.md` + `PRISM_DATASETS` env var documentation below |
| A Twirp RPC method | `rpc/service.proto` → regenerate via `make proto` → `rpc/server.go` + `cmd/prism/cmd_serve.go` HTTP shim + smoke test under `cmd/prism/twirp_roundtrip_test.go` |
| An MCP tool | `mcp/server.go` + `docs/src/cookbook/mcp-agent-integration.md` |
| An environment variable | This file ("Build / Env" section) + `internal/limits/limits.go` (defaults + parser) if numeric |
| An optimizer pass | `plan/passes/register.go` + `docs/src/concepts/spec.md` (Plan section) + entry in `plan/optimize.go` |
| A `prism init` template (`.prism/`) | `cmd/prism/templates/` + smoke test in `cmd/prism/init_test.go` + `docs/src/getting-started.md` editor-setup paragraph |
| `prism static-bundle` output shape | `cmd/prism/cmd_static_bundle.go` + `static/staticfs.go` + smoke test |
| A playground example (`docs/src/playground/examples/`) | `docs/src/playground/examples/manifest.json` (id + title + file) + the new `<id>.json` spec file. Specs must use inline `data.values` or `datasets.*.values` — the playground has no `.pulse` fetch path |

If you find yourself wanting to defer the doc update to "a follow-up PR," stop. The follow-up will not happen, the next Claude Code session will read stale guidance and produce wrong code. Update in the same PR or do not merge.

## Architecture

```
prism/
├── cmd/prism/              # Host CLI binary (gated `//go:build !js` where needed)
│   ├── main.go             # urfave/cli/v3 wiring
│   ├── cmd_*.go            # one file per CLI leaf
│   ├── templates/          # `prism init` payload (schemas + examples + editor configs)
│   └── *_smoke_test.go     # per-command end-to-end checks
├── cmd/prismwasm/          # WASM entry — `//go:build js && wasm`
│   ├── main.go             # exports validate/plan/execute/render/errorsLookup/schemaBundle/version on globalThis.prism via syscall/js
│   └── wasm_smoke_test.go  # Node + wasm_exec runner against committed fixtures
├── spec/                   # Spec types + decoders (Mark, Encoding, Transform, Selection, Composition)
├── validate/               # Shape + semantic validation (no row I/O)
│   ├── shape.go            # Schema-aware structural checks
│   ├── semantic.go         # Rule registry runner
│   ├── lookup.go           # Field/dataset lookup (pulse-backed + static)
│   ├── RULES.md            # PRISM_SPEC_NNN rule catalogue
│   └── rules/              # One file per semantic rule
├── plan/                   # DAG builder + sequential/parallel executor
│   ├── dag.go              # Node graph + topological sort
│   ├── builder.go          # Spec → DAG
│   ├── execute.go          # Bounded worker pool, partial failure
│   ├── cache.go cache_lru.go # Table cache (LRU)
│   ├── optimize.go passes/ # DedupSources, FilterPushdown, ProjectionPruning, AggregateFusion, PulseChainFusion, SampleInjection
│   ├── render.go           # Plan diagnostics (text / dot / json)
│   └── nodes/              # Source, Filter, Bin, Calculate, GroupAggregate, Join, Limit, Pivot, Project, Sample, Sort, Union, Unpivot, Window, Inline
├── compile/                # Plan/transform → Pulse request
│   ├── aggregates.go       # Friendly alias → AGG_* table
│   ├── expression.go       # Pulse expression compiler
│   └── inmem/              # In-memory backend for hash join + client-side aggregates
├── encode/                 # Scene IR + scales + axis + legend + palette
│   ├── encode.go           # Main spec → scene encoder
│   ├── encode_composite.go # layer / concat / facet / repeat
│   ├── encode_facet.go encode_repeat.go encode_selection*.go
│   ├── layout.go scale.go palette.go ticks*.go axis_build.go legend_build.go
│   ├── selection_build.go  # Selection materialisation
│   ├── marks/              # Per-mark encoders (bar, line, area, point, rule, text, tick, rect, arc, pie, donut, histogram, heatmap, boxplot, violin, sankey, funnel, sparkline, image, path)
│   ├── scale/              # linear, log, pow, sqrt, time, band, point, ordinal
│   ├── scene/              # Scene IR types (Mark, Geom, Axis, Legend, Theme, Selection, Annotation, …)
│   ├── resolve/            # Cross-layer domain + scheme resolution
│   └── format/             # d3-format subset
├── render/                 # Bytes
│   ├── render.go           # Backend dispatch
│   ├── precision.go        # Pinned 3-decimal coordinate quantisation
│   ├── svg/                # Go SVG renderer (canonical)
│   ├── pdf/                # `signintech/gopdf` with embedded Inter + JetBrains Mono fonts
│   └── canvas/             # Vendored ESM web component bridge (see `static/`)
├── resolve/                # Data source resolution
│   ├── default.go          # Pulse-backed + file / archive / shard
│   ├── registry_dataset.go # `datasets` block + `PRISM_DATASETS` env
│   └── resolver.go         # Resolver interface
├── theme/                  # Theme registry + loader
│   ├── light.go dark.go print.go
│   ├── css.go              # CSS variable manifest
│   └── loader.go override.go
├── schema/v1/              # JSON Schema bundle (`urn:prism:schema:v1:spec`)
├── errors/                 # PRISM_* code catalogue + AppError envelope
├── rpc/                    # Twirp service (proto + generated + server)
├── mcp/                    # MCP server (stdio)
├── static/                 # Vendored ESM bundle for `prism static-bundle`
├── table/                  # In-memory tabular intermediate
├── testdata/               # Golden fixtures + cross-impl artifacts
├── docs/                   # mdBook source (GitHub Pages)
├── internal/
│   ├── devtools/           # Cross-impl runner (Go vs JS scene IR)
│   ├── gates/              # Repo-wide structural / hygiene tests
│   ├── limits/             # Env-driven memory ceilings (PRISM_*_MAX_*)
│   ├── observability/      # Logging / metrics shims
│   ├── tools/              # One-off codegen / maintenance
│   └── validatorutil/      # Shared validate helpers
```

`cmd/prism` commands map 1:1 to public CLI leaves: `version`, `validate`, `errors lookup`, `plan`, `execute`, `plot`, `scene`, `serve`, `mcp`, `inspect`, `examples`, `schema`, `init`, `static-bundle`. Internal commands (none today) live behind hidden flags.

`mcp/` exposes the agent surface over stdio. `rpc/` exposes the same surface over Twirp HTTP behind `prism serve`.

Documentation lives in `docs/` (mdBook, published to <https://frankbardon.github.io/prism/>). The schema bundle in `schema/v1/` is the machine-readable contract loaded by editors (via `prism init`) and by `validate/` at runtime.

## Code Conventions

### Naming

- All identifiers, comments, docs are Prism-native. Module path: `github.com/frankbardon/prism`.
- `PRISM_*` is reserved for error codes and environment variables. Use `PRISM_<DOMAIN>_NNN` (`PRISM_SPEC_001`) for numbered codes and `PRISM_<DOMAIN>_<DESCRIPTOR>` (`PRISM_RENDER_FORMAT_UNAVAILABLE`, `PRISM_JOIN_MAX_ROWS`) for descriptor-style codes. Warnings use the `PRISM_WARN_*` prefix.
- Spec field keys are snake_case (`stroke_width`, `corner_radius`, `font_size`). Single-word Vega-Lite vocabulary (`mark`, `encoding`, `transform`, `layer`, `facet`) stays as-is. Channel names (`x`, `y`, `x2`, `y2`, `color`, `size`, `shape`, `opacity`, `text`, `tooltip`, `href`, `theta`, `radius`) stay verbatim from Vega-Lite.
- Pulse aggregate aliases mirror Vega-Lite: `count`, `sum`, `mean`, `median`, `min`, `max`, `stdev`, `variance`, `q1`, `q3`, `ci0`, `ci1`. Prism adds `distinct`, `mode`. Cohort-analytics extensions are `wmean`, `ratio`, `lift`, `share`.
- Mark names are bare nouns: `bar`, `line`, `area`, `point`, `rule`, `text`, `tick`, `rect`, `arc`, `pie`, `donut`, `histogram`, `heatmap`, `boxplot`, `violin`, `sankey`, `funnel`, `sparkline`, `image`, `path`.

### Error handling

Six error domains live under `errors/codes.go` (`Codes` map): `SPEC`, `RESOLVE`, `PLAN`, `COMPILE`, `ENCODE`, `RENDER`, plus per-feature descriptor codes (`JOIN`, `SERVE`, …) and the `PRISM_WARN_*` warning family. Every code carries:

- `Code` — canonical `PRISM_*` identifier.
- `Message` — terse one-liner.
- At least one of: `Fixups` template list OR a non-empty `SeeAlso` cross-reference.

`errors.New(code, message, details)` builds the `AppError` envelope. CLI surfaces JSON envelopes via `--json`; human output is `<CODE>: <message>` plus rendered fixups. Reactive lookup is `prism errors lookup CODE` (CLI) and the equivalent MCP tool.

Validation rules live one-per-file under `validate/rules/`. Each rule implements `Rule` from `validate/semantic.go`:

```go
type Rule interface {
    Code() string                     // canonical PRISM_SPEC_NNN
    Apply(ctx Context) []*AppError    // emit zero or more errors
}
```

Rules register through `validate/rules/register.go` (loaded via `init()`). Add a new rule by dropping a file and registering it — do not modify existing rule files.

### Output Format Contract

- **No `fmt.Sprintf`-built JSON.** All structured output goes through `encoding/json`. CLI envelopes are built explicitly so missing fields fail at compile time.
- **Stable Scene IR.** `encode/scene/` types serialise to a stable JSON shape consumed by the JS-side renderer. Field additions are additive; renames or removals require a version bump and a JS-side migration.
- **Pinned coordinate precision.** SVG and PDF renderers round coordinates via `render.precision.go` to 3 decimal places. Adding a new geometric primitive MUST route through the precision helper so cross-impl goldens stay stable.
- **Golden parity.** SVG goldens live under `render/svg/testdata/` and `cmd/prism/templates/` smoke fixtures. JS-side comparison fixtures live under `testdata/cross_impl/` — `scene.json` + `go.svg` are committed; `js.svg` + `diff.txt` regenerate per run (gitignored).

### Plan + Execute

`plan.Build(spec, registry) (*Plan, error)` constructs the DAG without executing. `plan.Execute(ctx, p, opts)` runs it. Topological order with bounded worker fan-out per `ExecOpts.Workers` (0 ⇒ `PRISM_QUERY_WORKERS` env ⇒ `runtime.NumCPU()`; 1 ⇒ serial). Partial-failure policy controlled by `ExecOpts.FailFast` (defaults true). Optimizer passes run between Build and Execute in this order: `DedupSources`, `FilterPushdown`, `ProjectionPruning`, `AggregateFusion`, `PulseChainFusion`, `SampleInjection`. `PulseChainFusion` collapses a source-rooted linear chain (`Filter`/`Calculate`/`GroupAggregate`/`Sort`) into one `pulse.ProcessChain` call so Pulse pushes filters down at the cohort reader and Prism never materialises the source `table.Table`; it requires a `GroupAggregate` (win condition) and falls back to per-node execution with `PRISM_PLAN_CHAIN_NOT_MERGEABLE` if a stage trips Pulse's chain gate. Add new passes via `plan/passes/register.go`.

### Composition

`encode/encode_composite.go` handles `layer`, `concat`, `hconcat`, `vconcat`, `facet`, `repeat`. Cross-layer scale resolution defaults to **shared** for matching channel + field pairs; opt-out via `resolve: "independent"` per scale. `facet`/`repeat` expand into per-cell child scenes whose absolute positions land via `encode/layout.go`.

### Multi-source

The `datasets` block in a spec declares named cohorts. Per-layer / per-mark `data` overrides bind to a dataset by name. Hash join is a transform (`{join: {left, right, on, kind}, as}`) with kinds `inner`, `left`, `outer`, `anti`. Cardinality is bounded by `PRISM_JOIN_MAX_ROWS` (default 5,000,000); overflow returns `PRISM_JOIN_001` with the offending product in `details`. Server-side registry: `resolve.DatasetRegistry` (loaded from `--datasets-config` JSON file + `PRISM_DATASETS` env, chained file → env). Browser-side: declared via `<prism-chart datasets="…">` attribute on the web component (see `static/`).

### Selections

`spec.Selection` (point + interval) compiles to `encode/scene.Selection` and is rendered as either client-resolved overlays (web component) or server-resolved derived datasets (Twirp / MCP). The two modes share the same selection grammar; mode is chosen by the renderer backend.

### Theming

Three built-in themes ship: `light` (default), `dark`, `print`. Each lives in `theme/<name>.go` and supplies a `theme.Tokens` struct (colors, fonts, sizes). The renderer materialises tokens as CSS variables in the SVG output via `theme/css.go` — downstream consumers can theme post-hoc by overriding variables. Custom themes load from `theme.json` via `theme/loader.go`; sparse spec-level overrides merge through `theme/override.go`. Adding a token requires updating every built-in theme and `theme/css.go`'s manifest emitter.

## Build / Env

`make build` (default), `make build-wasm`, `make test`, `make test-race`, `make fmt`, `make fmt-check`, `make vet`, `make lint`, `make cover`, `make clean`, `make proto`, `make docs`, `make docs-serve`, `make docs-clean`. A `.env` at repo root is auto-loaded by the Makefile.

`make build-wasm` produces `bin/prism.wasm` from `cmd/prismwasm` under `GOOS=js GOARCH=wasm -ldflags="-s -w" -trimpath -buildvcs=false`. The companion `wasm_exec.js` is copied from `$(go env GOROOT)/lib/wasm/wasm_exec.js` and asserted byte-identical by `cmd/prism/static_bundle_smoke_test.go`.

**Environment variables:**

- `PRISM_DATASETS` — semicolon-separated `name=ref` list registering named datasets for `data.source` lookup. `ref` is a `.pulse` path, an archive shard ref (`archive.pulse#shard.pulse`), or a future-supported source URL. Layered behind `--datasets-config` JSON file (file wins). Defined in `resolve/registry_dataset.go` (`EnvDatasetVar`).
- `PRISM_TABLE_MAX_ROWS` — cap on any single materialised `table.Table`. Default 50,000,000. Defined in `internal/limits/limits.go`.
- `PRISM_JOIN_MAX_ROWS` — cap on left × right product for the hash-join node. Default 5,000,000. Overflow → `PRISM_JOIN_001`.
- `PRISM_RENDER_MAX_MARKS` — cap on the number of marks the renderer emits before auto-`Sample` injection by the `SampleInjection` optimizer pass. Default 100,000.
- `PRISM_QUERY_WORKERS` — bounded executor worker count for `plan.Execute`. 0 (or unset) ⇒ `runtime.NumCPU()`. 1 ⇒ serial. Positive integers cap the fan-out.
- `PRISM_TABLE_CACHE_SIZE` — LRU capacity for the plan-level table cache. Default 256 entries.
- `PRISM_CROSS_IMPL` — set to `1` to opt into the cross-implementation parity tests under `internal/devtools/`. After P17, the harness compares Go-native SVG vs Go-via-wasm SVG (was Go vs JS port). Needs `node` on `PATH`.
- `PRISM_CROSS_IMPL_REGEN` — set to `1` to regenerate the WASM-side scene fixtures during a cross-impl run.
- `PRISM_WASM_MAX_BYTES` — gzipped size ceiling for `bin/prism.wasm` enforced by `internal/gates/wasm_size_test.go`. Default 16,777,216 (16 MB); soft warning at 12 MB. Defined in `internal/limits/limits.go`.

Numeric env vars parse loudly: a non-empty value that fails to parse, or that resolves to non-positive, is rejected by the lookup helpers in `internal/limits/limits.go` (returns default + `ok=false`). Callers may surface a config error or silently fall back via the `Must*` helpers.

Hermetic testing: `afero.NewMemMapFs()` is the default for tests under `validate/`, `resolve/`, `plan/`, `compile/`. No disk I/O in unit tests outside the goldens path.

## Spec Format ($schema)

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data":    {"source": "cohort.pulse"},
  "mark":    {"type": "bar"},
  "encoding": {
    "x": {"field": "Origin", "type": "nominal"},
    "y": {"aggregate": "mean", "field": "Horsepower", "type": "quantitative"}
  }
}
```

- `$schema` is the URN form `urn:prism:schema:v1:spec`. Schema bundle lives in `schema/v1/` (`//go:embed`'d into the binary). `prism init` writes the JSON Schema files into `.prism/schemas/` for editor autocomplete.
- `data.source` is the Pulse ref (single-file `.pulse`, archive-shard anchor, or `PRISM_DATASETS`/`datasets`-registered name). Vega-Lite's `data.url` is **not** accepted — port via `prism validate --fix-suggestions`.
- `type` is required on every channel encoding (Prism is strict — Vega-Lite's inference is not implemented).
- Vega-Lite's `params` / signals and per-encoding `condition` blocks are not implemented in v1.

Bump the schema bundle version (`schema/v1/` → `schema/v2/`) only on backwards-incompatible spec shape changes. Additive fields stay on `v1`. Bump triggers an update of `cmd/prism/cmd_init.go` (templates) and every `$schema` reference in `docs/src/`.

## Non-Skippable Gates

These tests live in `internal/gates/` and per-package `*_test.go` files. CI is configured to fail the build on any of them:

- **Format / vet / staticcheck** — `make fmt-check && make lint`. CI runs both jobs (`test` + `lint`) on every PR.
- **Race detector** — `make test-race`. Spec validation, plan execution, and the table cache are concurrent paths; the race detector catches data-race regressions before they ship.
- **Golden parity** — `render/svg/goldens_test.go` and `validate/golden_test.go` compare against committed SVG and JSON envelopes; mismatches fail the build. Regenerate via the per-package `-update` flag, never hand-edit.
- **Cross-impl parity (opt-in)** — `PRISM_CROSS_IMPL=1` enables Go-vs-JS scene IR comparison under `internal/devtools/`. Off by default in CI (requires `npm install`); run locally before changes to `encode/scene/` or `render/svg/`.
- **Smoke tests** — `cmd/prism/*_smoke_test.go` covers every CLI leaf end-to-end against fixtures. New CLI leaves require a smoke test.
- **Gallery freshness** — `docs/src/gallery/` SVGs are regenerated by `render/svg/goldens_test.go` outputs; gallery changes require a matching test fixture update.

## What NOT to Do

- **Do not put business logic in `cmd/prism/`.** CLI is a thin adapter — parse flags, construct library objects, call methods, format output. Smoke tests gate this discipline.
- **Do not bypass `afero.Fs`** for file access — defeats hermetic testing and the in-memory `prism serve` path.
- **Do not hand-edit golden files.** Regenerate via the per-package `-update` flag, and review the diff before committing.
- **Do not introduce JS expression evaluation.** Filter / calculate transforms use Pulse expression syntax. Inline JS evaluation is not — and will not be — supported.
- **Do not import `service/` or `processing/` from `descriptor/`.** (Pulse rule; mirrored here: do not import `plan/` or `render/` from `validate/`, `compile/`, or `resolve/`. Predict / validate / inspect are no-execute by structural ban.)
- **Do not `fmt.Sprintf` JSON or SVG.** Use `encoding/json` and the scene → SVG emitters in `render/svg/`. Hand-formatted output drifts from goldens within one release.
- **Do not skip `make fmt-check && make lint` before committing.** CI will reject the PR — fix locally first.
- **Do not defer doc updates to a follow-up PR.** The follow-up will not happen. Update `docs/src/`, `schema/v1/`, and this file in the same PR per the Update Demand.
- **Do not add a transform without registering a Plan node.** `spec.Transform*` decoders, the union dispatcher in `spec/transform_union.go`, and the `plan/nodes/<name>.go` file must land together. Tests in `plan/dag_test.go` and `plan/builder.go` will fail otherwise.
- **Do not add an env var without updating "Build / Env"** above. New ceilings also land their default constant and parser in `internal/limits/limits.go`.
