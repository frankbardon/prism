# CLAUDE.md

## Project Overview

Prism is a visualization library for materialized tabular data. Ships as a Go library (`github.com/frankbardon/prism`) and a CLI binary (`cmd/prism/`). Library is primary; CLI is a thin adapter.

**Design principles:**

- **Library-first.** Every public surface in `spec/`, `validate/`, `plan/`, `compile/`, `encode/`, `render/`, `resolve/`, `theme/`, `rpc/`, and `mcp/` is reachable as a Go API. `cmd/prism/` (host CLI) and `cmd/prismwasm/` (browser entry, `//go:build js && wasm`) never contain business logic — parse flags / marshal JSON, construct library objects, format output.
- **Six-stage pipeline.** Spec (JSON) → Validate → Plan → Compile → Encode → Render → Bytes. Each stage is independently testable, and intermediate artifacts (Plan, Scene IR, Encoded bytes) are stable JSON shapes downstream consumers can pin.
- **Vega-Lite vocabulary, snake_case keys.** Single-word terms (`mark`, `encoding`, `transform`, `layer`, `facet`, `concat`, `repeat`) match Vega-Lite verbatim. Multi-word keys are snake_case throughout (`stroke_width`, `corner_radius`, `font_size`).
- **Structured, expression-free transforms.** Prism has NO expression language. `filter` predicates, `calculate` derived columns, and condition `test` clauses are structured JSON built-ins (`spec/predicate.go` + `spec/calc_expr.go`): comparison / set / range / null-check leaves plus `and`/`or`/`not` combinators for filters; `op`/`fn`/`concat`/`case` nodes for calculate. A raw string where a predicate or expression is expected is rejected at decode. No `datum.` prefix, no operators, no JS function calls, no Vega expression eval. Anything richer is precomputed upstream by the caller.
- **No-execute predict & validate.** `validate/` reads only the spec + optional schema (no row I/O); `plan` builds the DAG without executing it. Network and filesystem I/O happen only at `plan.Execute` time.
- **Pulse-free — consumes materialized data.** Prism has NO dependency on `github.com/frankbardon/pulse` and never reads `.pulse` files (the dependency was evicted in epic E4; a `go list -deps` firewall keeps it out — see Non-Skippable Gates). Prism renders **already-materialized rows**: inline `data.values` / `datasets.*.values`, or lazily via a caller-supplied `resolve.DataResolver` (`data: {ref}`). The downstream/consuming library owns 100% of Pulse — it runs the Pulse query and converts the response into a Prism spec with rows inlined. Every transform (including aggregates, `crosstab`, and `regression`) executes in-memory over `table.Table` in `compile/inmem/`; the aggregate aliases (`count`, `sum`, `mean`, …, `wmean`, `ratio`, `lift`, `share`, `ci0`, `ci1`) are all client-side computations with no backend op constant.

## The Update Demand

Any change to Prism code, configuration, spec vocabulary, schema bundle, or public surface MUST update the corresponding doc page(s) and CLAUDE.md in the same PR.

| If you change... | You MUST also update... |
|---|---|
| A mark in `encode/marks/` | `docs/src/concepts/marks.md` + add a gallery entry under `docs/src/gallery/<family>/` if user-visible |
| An encoding channel | `docs/src/concepts/encoding.md` + `schema/v1/` JSON Schema for the channel shape |
| A transform (`filter`, `aggregate`, `bin`, `calculate`, `join`, `pivot`, `sample`, `sort`, `unpivot`, `window`, `crosstab`, `regression`, `timeunit`) | `docs/src/concepts/spec.md` (transform section) + add a Plan node under `plan/nodes/` + add a `Spec*Transform` union variant in `spec/transform_union.go` + register `transformDiscriminators` + `transformAsName` switch in `plan/build/build.go` + JSON Schema variant in `schema/v1/transform.schema.json` (`oneOf` entry + `$def`). The `filter` predicate grammar lives in `spec/predicate.go` and `calculate` in `spec/calc_expr.go` (structured built-ins, no expression string) — extending either operator/function set also touches those types, the `predicate_*` / `calc_expr` `$defs` in `schema/v1/transform.schema.json`, the `compile/inmem/filter.go` / `calculate.go` evaluators, and the `PRISM_SPEC_037` / `PRISM_SPEC_038` validate rules |
| A source-rooted analytic transform that pivots/fits the whole materialized table (e.g. `crosstab`, `regression`) | New leaf plan node under `plan/nodes/` (mirror `crosstab.go` / `regression.go` — consumes the upstream `SourceNode`'s materialized `table.Table`, computed pure-Go in `compile/inmem/`) + matching spec transform variant + validate rule constraining position (must be the first transform on the chain — there is no in-memory cohort constructor for a derived alias) + `PRISM_PLAN_<NAME>_REQUIRES_SOURCE` error code + plan-build dispatch in `plan/build/build.go` that resolves the immediate input to a `*nodes.SourceNode` (inline `data.values` binds an `InlineNode`, so such a fixture is validate-only unless a `datasets`-registered source ref is bound) |
| A composition operator (`layer`, `concat`, `hconcat`, `vconcat`, `facet`, `repeat`) | `docs/src/concepts/composition.md` + composite encoder under `encode/encode_composite.go` |
| A scale type | `docs/src/concepts/encoding.md` (scale section) + `encode/scale/` implementation + tick generator under `encode/ticks*.go` |
| A theme (or built-in theme value) | `docs/src/concepts/themes.md` + `theme/<name>.go` + register in `theme/registry.go` + token entry in `theme/css.go` + gallery fixture in `docs/src/gallery/themes/` + gallery index row in `docs/src/gallery/index.md` |
| A theme token (nested block field on `theme.Theme`) | `theme/style.go` (struct field) + matching spec wire field in `spec/theme.go` + override copy helper in `theme/override.go` + JSON Schema property in `schema/v1/theme.schema.json` + CSS variable emitter in `theme/css.go` + clone/merge in `theme/loader.go` + every built-in theme that should set the new token |
| A named color scheme | `theme/schemes.go` (`builtinSchemes` entry — name, kind, hex list) + `docs/src/concepts/themes.md` scheme catalogue + (if accessibility-relevant) consider seeding it as the default in `theme/colorblind.go` or `theme/high_contrast.go` |
| A `theme.Range` slot | `theme/range.go` (`Range` struct + `Clone` + `MergeRange`) + matching spec wire field in `spec/theme.go` + override copy in `theme/override.go` (`copyRange`) + JSON Schema property in `schema/v1/theme.schema.json` (`range_block`) + scale-resolution call site in `encode/palette.go` (or marks/heatmap.go for sequential) + every built-in theme that should default the slot |
| A semantic validation rule | `validate/RULES.md` + new rule file under `validate/rules/` + register in `validate/semantic.go` + new `PRISM_SPEC_NNN` row in `errors/codes.go` |
| A `PRISM_*` error code (added / removed / renamed) | `errors/codes.go` (canonical `Code`, `Message`, at least one fixup template or `SeeAlso`) + reachable via `prism errors lookup` |
| A renderer backend (SVG / Canvas) | `docs/src/concepts/themes.md` (rendering notes if visual) + `render/<backend>/` + dispatch in `render/render.go` |
| Anything reachable from `cmd/prismwasm/main.go` (WASM entry) | `docs/src/concepts/browser.md` + size-budget gate `internal/gates/wasm_tinygo_size_test.go` + `cmd/prismwasm/wasm_smoke_test.go` if behaviour changes |
| A new package import in the WASM entry, OR a new file under a `!js`-gated subtree (`rpc/`, `mcp/`, `cmd/prism/cmd_serve.go`, `cmd_mcp.go`, `cmd_static_bundle.go`, `cmd_init.go`) | Re-run `make build-wasm-tinygo` locally; CI gates verify (a) the WASM entry still compiles under TinyGo and (b) the gzipped binary is under `PRISM_WASM_TINYGO_MAX_BYTES` |
| A CLI leaf (added / removed / flag added) | `cmd/prism/cmd_<name>.go` + `docs/src/getting-started.md` if user-visible + smoke test in `cmd/prism/*_smoke_test.go` |
| The schema bundle (`schema/v1/`) | `schema/embed.go` (the `//go:embed` directives) + bump bundle version if breaking + `docs/src/concepts/spec.md` (`$schema` reference) |
| A built-in dataset registry shape | `resolve/registry_dataset.go` + `docs/src/concepts/multi-source.md` + `PRISM_DATASETS` env var documentation below |
| A Twirp RPC method | `rpc/service.proto` → regenerate via `make proto` → `rpc/server.go` + `cmd/prism/cmd_serve.go` HTTP shim + smoke test under `cmd/prism/twirp_roundtrip_test.go` |
| An MCP tool | `mcp/server.go` (typed I/O structs + `func(ctx, *rpc.PrismServer, In) (Out, error)` handler) + `mcp/catalog.go` (register a `ToolDescriptor` in `Tools(cfg)` — name, description, `reflectSchema[In]`/`reflectSchema[Out]`, and an `Invoke` closure bridging to the handler; schemas are reflected from struct tags via jsonschema-go, never hand-written) + `mcp/gosdk/gosdk.go` (the go-sdk adapter mounts the catalog — usually no change unless the transport binding changes) + `examples/` if the tool reads the embedded spec corpus + `docs/src/cookbook/mcp-agent-integration.md`. Keep typed output flat (jsonschema-go panics on recursive types — type nested/recursive fields as `any`). The `mcp/` core must stay SDK-free (`internal/gates/mcp_firewall_test.go`) |
| An environment variable | This file ("Build / Env" section) + `internal/limits/limits.go` (defaults + parser) if numeric |
| An optimizer pass | `plan/passes/register.go` + `docs/src/concepts/spec.md` (Plan section) + entry in `plan/optimize.go` |
| A `prism init` template (`.prism/`) | `cmd/prism/templates/` + smoke test in `cmd/prism/init_test.go` + `docs/src/getting-started.md` editor-setup paragraph |
| `prism static-bundle` output shape | `cmd/prism/cmd_static_bundle.go` + `static/staticfs.go` + smoke test |
| A playground example (`docs/src/playground/examples/`) | `docs/src/playground/examples/manifest.json` (id + title + file) + the new `<id>.json` spec file. Specs must use inline `data.values` or `datasets.*.values` — the playground has no `.pulse` fetch path |
| A projection in `encode/projection/` or a feature in `geodata/` | `docs/src/concepts/geo.md` + `schema/v1/projection.schema.json` (the `type` enum) + manifest regeneration via `make geodata` when admin-level data changes |
| A `data` block variant (`name`, `ref`, `values`, `feature_collection`) | `spec/data.go` (struct field + UnmarshalJSON discriminator) + `schema/v1/data.schema.json` (oneOf entry) + `plan/build/build.go` registerDataset case + `docs/src/concepts/geo.md` for geo-specific variants + `docs/src/concepts/multi-source.md` for runtime-ref variants (caller-supplied `DataResolver` in `resolve/data_resolver.go`). The external `source` (`.pulse`) variant was removed in E4-S3 — Prism never reads `.pulse`; a `source` key is rejected at decode with `PRISM_SPEC_039`. `spec.Data.Source` survives only as an internal (`json:"-"`) binding target populated by the server-side `DatasetRegistry` (a `name` reference resolving through `PRISM_DATASETS`/`--datasets-config`) and by test harnesses. |
| An easing in `spec.AnimationEasings` or any field in `spec.Animation` | `spec/animation.go` (const list + struct) + `schema/v1/animation.schema.json` (enum / properties) + `docs/src/concepts/spec.md` (Animation table) + `static/vendor/prism/prism-animator.mjs` (`EASINGS` table + `_normaliseAnimation`) + `internal/devtools/cross-impl-runner/animator-tween.mjs` if behaviour changes |
| A numeric or color SVG attr the animator should tween | `static/vendor/prism/prism-animator.mjs` (`NUMERIC_ATTRS` or `COLOR_ATTRS`) + `docs/src/concepts/browser.md` (Animation section) + `internal/devtools/cross-impl-runner/animator-tween.mjs` fixture coverage |
| A gallery fixture under `docs/src/gallery/animation/` | new `<name>.prism.json` + golden `<name>.svg` (regen via `UPDATE_GOLDENS=1 go test ./cmd/prism/ -run TestPrismGalleryFixtures`) + entry in `docs/src/gallery/index.md` Animation section + `<prism-chart>` card in `docs/src/gallery/index.html`. `.scene.json` is built by `make docs-scenes` and gitignored |
| A condition on an encoded channel | `spec/condition.go` (a condition `test` is a structured `spec.Predicate` — the same grammar `filter` uses, from `spec/predicate.go`, not an expression string) + `spec/encoding.go` (`ChannelCommon.Condition`) + `schema/v1/encoding.schema.json` (`condition_test` $def referencing the `filter_predicate` grammar + `condition` ref on `channel_base`/`position_channel`/`mark_channel`) + `validate/rules/condition_selection_ref.go`/`condition_test_parses.go` (reuses the shared `checkPredicate` helper under `PRISM_SPEC_026`)/`condition_value_or_binding.go` + `errors/codes.go` (`PRISM_SPEC_025/026/027`) + `encode/scene/condition.go` (`ConditionalAttr`) + `encode/encode_condition.go` (evaluates the predicate row-by-row) + `static/vendor/prism/prism-selection.mjs` (`applyConditions`) + `docs/src/concepts/encoding.md` (Conditions section) |
| Per-column null support for a new aggregate, scale, mark, or transform | the new code + `table.Column.IsNull` / `NullCount` consultation + `PRISM_WARN_NULL_*` emission when applicable (`errors/codes.go`) + `docs/src/concepts/multi-source.md` null table |
| A new mark family that needs a layout algorithm | `encode/marks/<family>.go` + `encode/marks/layout/<algo>.go` + `spec/mark.go` (`MarkDef` fields) + `schema/v1/mark.schema.json` (`mark_type` enum + `mark_def` properties) + `validate/rules/channel_for_mark.go` allowlist + dedicated validate rule under `validate/rules/<family>_*.go` if structural invariants apply + `errors/codes.go` + `docs/src/concepts/marks.md` section + 2-4 gallery fixtures under `docs/src/gallery/<family>/` |

If you find yourself wanting to defer the doc update to "a follow-up PR," stop. The follow-up will not happen, the next Claude Code session will read stale guidance and produce wrong code. Update in the same PR or do not merge.

## Architecture

```
prism/
├── prism.go                # Root Go API: Compile, RenderPlan, CompiledPlan
├── patch.go scene.go       # RFC 6902 patch (Patch/PatchOp/ApplyPatch/DiffSpecs) + Scene wrapper
├── selection/              # Structured selection event (Event, SelectedMark, DataRowRef)
├── cmd/prism/              # Host CLI binary (gated `//go:build !js` where needed)
│   ├── main.go             # urfave/cli/v3 wiring
│   ├── cmd_*.go            # one file per CLI leaf
│   ├── templates/          # `prism init` payload (schemas + examples + editor configs)
│   └── *_smoke_test.go     # per-command end-to-end checks
├── cmd/prismwasm/          # WASM entry — `//go:build js && wasm`
│   ├── main.go             # exports validate/plan/execute/compile/render/errorsLookup/schemaBundle/version/setDataResolver/applyPatch/diffSpecs on globalThis.prism via syscall/js
│   └── wasm_smoke_test.go  # Node + wasm_exec runner against committed fixtures
├── spec/                   # Spec types + decoders (Mark, Encoding, Transform, Selection, Composition)
├── validate/               # Shape + semantic validation (no row I/O)
│   ├── shape.go            # Schema-aware structural checks
│   ├── semantic.go         # Rule registry runner
│   ├── lookup.go           # Field/dataset lookup (native schema shim + static inline)
│   ├── RULES.md            # PRISM_SPEC_NNN rule catalogue
│   └── rules/              # One file per semantic rule
├── plan/                   # DAG builder + sequential/parallel executor
│   ├── dag.go              # Node graph + topological sort
│   ├── builder.go          # Spec → DAG
│   ├── execute.go          # Bounded worker pool, partial failure
│   ├── cache.go cache_lru.go # Table cache (LRU)
│   ├── optimize.go passes/ # DedupSources, FilterPushdown, ProjectionPruning, AggregateFusion, SampleInjection
│   ├── render.go           # Plan diagnostics (text / dot / json)
│   └── nodes/              # Source, Inline, Filter, Bin, Calculate, GroupAggregate, Join, Limit, Pivot, Project, Sample, Sort, TimeUnit, Union, Unpivot, Window, Crosstab, Regression
├── compile/                # Transform → in-memory execution over table.Table
│   ├── aggregates.go       # Friendly aggregate-alias catalogue (names only; all client-side)
│   └── inmem/              # In-memory backend: filter/calculate/aggregate, hash join, crosstab pivot, OLS regression
├── encode/                 # Scene IR + scales + axis + legend + palette
│   ├── encode.go           # Main spec → scene encoder
│   ├── encode_composite.go # layer / concat / facet / repeat
│   ├── encode_facet.go encode_repeat.go encode_selection*.go
│   ├── layout.go scale.go palette.go ticks*.go axis_build.go legend_build.go
│   ├── selection_build.go  # Selection materialisation
│   ├── marks/              # Per-mark encoders (bar, line, area, point, rule, text, tick, rect, arc, pie, donut, histogram, heatmap, boxplot, violin, sankey, funnel, sparkline, sparkbar, winloss, sparkarea, bullet, image, path, geoshape, geopoint)
│   ├── scale/              # linear, log, pow, sqrt, time, band, point, ordinal
│   ├── projection/         # Geographic projections (mercator, equirect, naturalearth, albers_usa, orthographic)
│   ├── scene/              # Scene IR types (Mark, Geom, Axis, Legend, Theme, Selection, Annotation, …)
│   ├── resolve/            # Cross-layer domain + scheme resolution
│   └── format/             # d3-format subset
├── render/                 # Bytes
│   ├── render.go           # Backend dispatch
│   ├── precision.go        # Pinned 3-decimal coordinate quantisation
│   ├── svg/                # Go SVG renderer (canonical)
│   └── canvas/             # Vendored ESM web component bridge (see `static/`)
├── resolve/                # Data source resolution
│   ├── default.go          # Inline-rows resolver (InlineResolver seam; no `.pulse` I/O)
│   ├── registry_dataset.go # `datasets` block + `PRISM_DATASETS` env
│   ├── data_resolver.go    # DataResolver interface + Dataset (runtime `data: {ref}` variant)
│   └── resolver.go         # Resolver interface
├── theme/                  # Theme registry + loader
│   ├── light.go dark.go print.go
│   ├── css.go              # CSS variable manifest
│   └── loader.go override.go
├── geodata/                # Manifest (countries + admin-1 IDs / bbox) + tier bundles (TopoJSON-lite)
├── schema/v1/              # JSON Schema bundle (`urn:prism:schema:v1:spec`)
├── errors/                 # PRISM_* code catalogue + AppError envelope
├── rpc/                    # Twirp service (proto + generated + server)
├── mcp/                    # SDK-free MCP core: typed I/O + handlers + ToolDescriptor catalog (`Tools(cfg)`); imports NO MCP SDK
│   └── gosdk/              # go-sdk adapter — the ONLY MCP-SDK importer; `Register(server, facade, cfg)` mounts the catalog + example corpus
├── examples/               # Stdlib-pure `//go:embed`'d spec corpus (List/Get/Search/Invalid/All); shared by `prism examples` + `examples_search`
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

`cmd/prism` commands map 1:1 to public CLI leaves: `version`, `validate`, `errors lookup`, `plan`, `execute`, `plot`, `scene`, `serve`, `mcp`, `examples`, `schema`, `init`, `static-bundle`. Internal commands (none today) live behind hidden flags.

`mcp/` is the SDK-free agent core: it defines the typed tool I/O, the typed handlers, and the `ToolDescriptor` catalog (`Tools(cfg)`), reflecting JSON Schemas from struct tags via jsonschema-go. `mcp/gosdk/` is the only package that imports the MCP SDK (`github.com/modelcontextprotocol/go-sdk`); its `Register(server, facade, cfg)` mounts the catalog and the embedded `examples/` corpus onto a caller-supplied go-sdk server over stdio. `cmd/prism/cmd_mcp.go` dogfoods this single registration path (threading `versionString` via `mcp.Config`). `rpc/` exposes the same facade over Twirp HTTP behind `prism serve`.

Documentation lives in `docs/` (mdBook, published to <https://frankbardon.github.io/prism/>). The schema bundle in `schema/v1/` is the machine-readable contract loaded by editors (via `prism init`) and by `validate/` at runtime.

## Code Conventions

### Naming

- All identifiers, comments, docs are Prism-native. Module path: `github.com/frankbardon/prism`.
- `PRISM_*` is reserved for error codes and environment variables. Use `PRISM_<DOMAIN>_NNN` (`PRISM_SPEC_001`) for numbered codes and `PRISM_<DOMAIN>_<DESCRIPTOR>` (`PRISM_RENDER_FORMAT_UNAVAILABLE`, `PRISM_JOIN_MAX_ROWS`) for descriptor-style codes. Warnings use the `PRISM_WARN_*` prefix.
- Spec field keys are snake_case (`stroke_width`, `corner_radius`, `font_size`). Single-word Vega-Lite vocabulary (`mark`, `encoding`, `transform`, `layer`, `facet`) stays as-is. Channel names (`x`, `y`, `x2`, `y2`, `color`, `size`, `shape`, `opacity`, `text`, `tooltip`, `href`, `theta`, `radius`) stay verbatim from Vega-Lite.
- Aggregate aliases mirror Vega-Lite and are all computed client-side over the materialized `table.Table` (`compile/inmem/group_aggregate.go`; `compile/aggregates.go` is a name catalogue only — no backend op constant): `count`, `sum`, `mean`, `median`, `min`, `max`, `stdev`, `variance`, `q1`, `q3`, `ci0`, `ci1`. Prism adds `distinct`, `mode`, `frequency`, and the distribution-shape scalars `range`, `skewness`, `kurtosis`, `null_count` (`distinct`, `mode`, `frequency`, `null_count` are universal — any field type; the distribution-shape scalars are quantitative/temporal only). Cohort-analytics extensions are `wmean`, `ratio`, `lift`, `share`. `frequency` is the SCALAR modal count (occurrences of the most frequent value, the multiplicity companion to `mode`). A `zscore` aggregate is intentionally NOT offered — a per-group mean z-score is always 0.
- Mark names are bare nouns: `bar`, `line`, `area`, `point`, `rule`, `text`, `tick`, `rect`, `arc`, `pie`, `donut`, `histogram`, `heatmap`, `boxplot`, `violin`, `sankey`, `funnel`, `sparkline`, `sparkbar`, `winloss`, `sparkarea`, `bullet`, `image`, `path`.

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
- **Pinned coordinate precision.** The SVG renderer rounds coordinates via `render.precision.go` to 3 decimal places. Adding a new geometric primitive MUST route through the precision helper so cross-impl goldens stay stable.
- **Golden parity.** SVG goldens live under `render/svg/testdata/` and `cmd/prism/templates/` smoke fixtures. JS-side comparison fixtures live under `testdata/cross_impl/` — `scene.json` + `go.svg` are committed; `js.svg` + `diff.txt` regenerate per run (gitignored).

### Plan + Execute

`plan.Build(spec, registry) (*Plan, error)` constructs the DAG without executing. `plan.Execute(ctx, p, opts)` runs it. Topological order with bounded worker fan-out per `ExecOpts.Workers` (0 ⇒ `PRISM_QUERY_WORKERS` env ⇒ `runtime.NumCPU()`; 1 ⇒ serial). Partial-failure policy controlled by `ExecOpts.FailFast` (defaults true). Optimizer passes run between Build and Execute in this order: `DedupSources`, `FilterPushdown`, `ProjectionPruning`, `AggregateFusion`, `SampleInjection`. All transforms execute over the in-memory backend against the materialised `table.Table`. Add new passes via `plan/passes/register.go`.

### Composition

`encode/encode_composite.go` handles `layer`, `concat`, `hconcat`, `vconcat`, `facet`, `repeat`. Cross-layer scale resolution defaults to **shared** for matching channel + field pairs; opt-out via `resolve: "independent"` per scale. `facet`/`repeat` expand into per-cell child scenes whose absolute positions land via `encode/layout.go`.

### Multi-source

The `datasets` block in a spec declares named cohorts. Per-layer / per-mark `data` overrides bind to a dataset by name. Hash join is a transform (`{join: {left, right, on, kind}, as}`) with kinds `inner`, `left`, `outer`, `anti`. Cardinality is bounded by `PRISM_JOIN_MAX_ROWS` (default 5,000,000); overflow returns `PRISM_JOIN_001` with the offending product in `details`. Server-side registry: `resolve.DatasetRegistry` (loaded from `--datasets-config` JSON file + `PRISM_DATASETS` env, chained file → env). Browser-side: declared via `<prism-chart datasets="…">` attribute on the web component (see `static/`).

### Selections

`spec.Selection` (point + interval) compiles to `encode/scene.Selection` and is rendered as either client-resolved overlays (web component) or server-resolved derived datasets (Twirp / MCP). The two modes share the same selection grammar; mode is chosen by the renderer backend.

### Theming

Three built-in themes ship: `light` (default), `dark`, `print`. Each lives in `theme/<name>.go` and supplies a `theme.Tokens` struct (colors, fonts, sizes). The renderer materialises tokens as CSS variables in the SVG output via `theme/css.go` — downstream consumers can theme post-hoc by overriding variables. Custom themes load from `theme.json` via `theme/loader.go`; sparse spec-level overrides merge through `theme/override.go`. Adding a token requires updating every built-in theme and `theme/css.go`'s manifest emitter.

### Geographic marks

`geoshape` (choropleth polygons) and `geopoint` (lon/lat overlays) live alongside the rest of the encoders in `encode/marks/`. Spec block `projection.type` selects the lon/lat → pixel map (`mercator`, `equirectangular`, `naturalearth`, `albers_usa`, `orthographic`); per-projection params live in `encode/projection/`. Feature geometry comes from the `geodata/` package: a small manifest (~128 KB) is embedded in every build (host + WASM) for validate/plan; full tier bundles (`world-110m`, `world-50m`, `admin1-50m`) are NOT embedded — the host build loads them at runtime from a directory supplied via the `--geodata-dir` flag or the `PRISM_GEODATA` env var (`<dir>/<tier>.geo.json`), and rendering a geo mark with no directory configured hard-fails with `PRISM_GEODATA_DIR_UNSET` (a configured dir missing the requested tile fails with `PRISM_GEODATA_TIER_MISSING`). The WASM build fetches tiers from `${origin}/static/prism/geodata/` (override via `prism.geo.setBundleURL(url)` or `data-prism-geodata-url` attribute). `prism static-bundle` sources the tiers from `--geodata-dir` and emits the geodata files alongside the JS bundle so the WASM runtime works out-of-the-box. The committed tiers live in the repo at `geodata/*.geo.json` and are published for download at `https://frankbardon.github.io/prism/static/prism/geodata/<tier>.geo.json`. `make geodata` regenerates the committed artifacts from upstream Natural Earth — `make build` itself requires no network access. For the mdBook playground, `make docs-wasm-stage` copies the geodata files into `docs/src/static/prism/geodata/` (which is a symlink to `static/vendor/prism/geodata/`); the playground sets `prism.geo.setBundleURL("../static/prism/geodata/")` so the relative path works under any mdBook deployment.

## Build / Env

`make build` (default), `make build-wasm-tinygo`, `make test`, `make test-race`, `make fmt`, `make fmt-check`, `make vet`, `make lint`, `make cover`, `make clean`, `make proto`, `make docs`, `make docs-serve`, `make docs-clean`. A `.env` at repo root is auto-loaded by the Makefile.

`make build-wasm-tinygo` is the **sole** wasm build (`tinygo build -target=wasm -stack-size=$(TINYGO_STACK_SIZE) -o bin/prism.wasm ./cmd/prismwasm`, TinyGo 0.41.1+) — the standard-Go `GOOS=js GOARCH=wasm` target was retired, so `make build` needs no wasm toolchain. It emits ~7.2 MB raw / ~2.2 MB gzip. Its companion `wasm_exec.js` comes from `$(tinygo env TINYGOROOT)/targets/wasm_exec.js` and pairs only with the TinyGo binary (never mix it with a Go-toolchain loader). The filesystem seam is routed through `internal/vfs` (a host alias for `github.com/spf13/afero`, a native afero-free interface under the `wasm` build tag) so afero — and thus `net/http`, which TinyGo cannot compile for js/wasm — never enters the WASM import graph. `-stack-size` is raised from TinyGo's ~16 KB default because the JSON-Schema shape validator recurses deep enough to trap a small stack. `cmd/prismwasm/main.go` parks `main` on a package-level channel (not `select{}`, which TinyGo folds to a deadlock panic).

**Environment variables:**

- `PRISM_DATASETS` — semicolon-separated `name=ref` list registering named datasets for `data.name` lookup (a `{"data": {"name": "…"}}` reference resolves through this registry to a source-bound leaf). `ref` is an opaque resolver identifier. Layered behind `--datasets-config` JSON file (file wins). Defined in `resolve/registry_dataset.go` (`EnvDatasetVar`).
- `PRISM_GEODATA` — single directory path holding the map tier bundles (`<dir>/<tier>.geo.json` for `world-110m`, `world-50m`, `admin1-50m`) consumed at render time by `plot`, `scene`, `serve`, `mcp`, and `static-bundle` (the `--geodata-dir` flag is the per-command equivalent; flag wins). Host tiers are no longer embedded, so this (or the flag) must be set for any `geoshape`/`geopoint` mark — unset + a geo mark hard-fails with `PRISM_GEODATA_DIR_UNSET`; a configured dir missing the requested tier fails with `PRISM_GEODATA_TIER_MISSING`. This is a path, NOT numeric, so it does NOT live in `internal/limits/limits.go`; the flag is wired in `cmd/prism/geodata_dir.go` and consumed by the host loader in `geodata/`. The no-execute leaves (`validate`, `plan`) and the data-only `execute` leaf use only the embedded manifest and ignore it.
- `PRISM_TABLE_MAX_ROWS` — cap on any single materialised `table.Table`. Default 50,000,000. Defined in `internal/limits/limits.go`.
- `PRISM_JOIN_MAX_ROWS` — cap on left × right product for the hash-join node. Default 5,000,000. Overflow → `PRISM_JOIN_001`.
- `PRISM_RENDER_MAX_MARKS` — cap on the number of marks the renderer emits before auto-`Sample` injection by the `SampleInjection` optimizer pass. Default 100,000.
- `PRISM_QUERY_WORKERS` — bounded executor worker count for `plan.Execute`. 0 (or unset) ⇒ `runtime.NumCPU()`. 1 ⇒ serial. Positive integers cap the fan-out.
- `PRISM_TABLE_CACHE_SIZE` — LRU capacity for the plan-level table cache. Default 256 entries.
- `PRISM_CROSS_IMPL` — set to `1` to opt into the cross-implementation parity tests under `internal/devtools/`. The harness compares host-native SVG vs TinyGo-via-wasm SVG (TinyGo is the sole wasm build). Needs `node` on `PATH`.
- `PRISM_CROSS_IMPL_REGEN` — set to `1` to regenerate the WASM-side scene fixtures during a cross-impl run.
- `PRISM_CROSS_IMPL_TINYGO` — set to `1` to opt into the TinyGo↔host WASM float-parity harness (`internal/devtools/tinygo_parity_test.go`): builds `cmd/prismwasm` with TinyGo, renders the fixture corpus under Node, and diffs SVG byte-for-byte against the host-Go goldens; also drives `render.FormatFloat` over an edge-case corpus in host / TinyGo-wasm and asserts they agree. Guards the top TinyGo-migration risk (`strconv`/`FormatFloat` drift through `render/precision.go`). Needs both `node` and `tinygo` on `PATH`.
- `PRISM_WASM_TINYGO_MAX_BYTES` — gzipped size ceiling for the **TinyGo**-built `bin/prism.wasm` (`make build-wasm-tinygo`, the sole browser artifact), enforced by `internal/gates/wasm_tinygo_size_test.go`. Default 4,194,304 (4 MB); soft warning at 3 MB (measured 2,232,605 B gzipped). CI pins TinyGo 0.41.1 and runs this gate as a hard requirement; locally the gate builds a fresh TinyGo module into a temp dir and **skips cleanly when `tinygo` is not on `PATH`**. Defined in `internal/limits/limits.go`.
- `PRISM_WASM_TINYGO_RAW_MAX_BYTES` — uncompressed size ceiling for the TinyGo-built `bin/prism.wasm`, enforced by the same TinyGo gate. Default 12,582,912 (12 MB); soft warning at 10 MB (measured 7,239,767 B raw). Defined in `internal/limits/limits.go`.

Numeric env vars parse loudly: a non-empty value that fails to parse, or that resolves to non-positive, is rejected by the lookup helpers in `internal/limits/limits.go` (returns default + `ok=false`). Callers may surface a config error or silently fall back via the `Must*` helpers.

Hermetic testing: `afero.NewMemMapFs()` is the default for tests under `validate/`, `resolve/`, `plan/`, `compile/`. No disk I/O in unit tests outside the goldens path.

## Spec Format ($schema)

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data":    {"values": [{"Origin": "USA", "Horsepower": 130}, {"Origin": "Europe", "Horsepower": 90}]},
  "mark":    {"type": "bar"},
  "encoding": {
    "x": {"field": "Origin", "type": "nominal"},
    "y": {"aggregate": "mean", "field": "Horsepower", "type": "quantitative"}
  }
}
```

- `$schema` is the URN form `urn:prism:schema:v1:spec`. Schema bundle lives in `schema/v1/` (`//go:embed`'d into the binary). `prism init` writes the JSON Schema files into `.prism/schemas/` for editor autocomplete.
- `data` supplies rows as inline `values`, a runtime `ref` (resolved by a caller-supplied `DataResolver`), a `datasets`-block `name`, or a geodata `feature_collection`. The external `data.source` (`.pulse`) variant was removed in E4-S3 — Prism never reads `.pulse`; a `source` key is rejected at decode with `PRISM_SPEC_039`. Vega-Lite's `data.url` is **not** accepted either.
- `type` is required on every channel encoding (Prism is strict — Vega-Lite's inference is not implemented).
- Vega-Lite's `params` / signals and per-encoding `condition` blocks are not implemented in v1.

Bump the schema bundle version (`schema/v1/` → `schema/v2/`) only on backwards-incompatible spec shape changes. Additive fields stay on `v1`. Bump triggers an update of `cmd/prism/cmd_init.go` (templates) and every `$schema` reference in `docs/src/`.

## Non-Skippable Gates

These tests live in `internal/gates/` and per-package `*_test.go` files. CI is configured to fail the build on any of them:

- **Format / vet / staticcheck** — `make fmt-check && make lint`. CI runs both jobs (`test` + `lint`) on every PR.
- **Race detector** — `make test-race`. Spec validation, plan execution, and the table cache are concurrent paths; the race detector catches data-race regressions before they ship.
- **Golden parity** — `render/svg/goldens_test.go` and `validate/golden_test.go` compare against committed SVG and JSON envelopes; mismatches fail the build. Regenerate via the per-package `-update` flag, never hand-edit.
- **Cross-impl parity (opt-in)** — `PRISM_CROSS_IMPL=1` enables host-Go-vs-TinyGo-wasm scene IR / SVG comparison under `internal/devtools/`. Off by default in CI (requires `node`); run locally before changes to `encode/scene/` or `render/svg/`.
- **TinyGo wasm build + size gate** — CI installs pinned TinyGo 0.41.1, runs `make build-wasm-tinygo`, and hard-gates `internal/gates/wasm_tinygo_size_test.go` (the TinyGo module is the sole browser artifact — gzipped under `PRISM_WASM_TINYGO_MAX_BYTES`, raw under `PRISM_WASM_TINYGO_RAW_MAX_BYTES`). Locally the gate skips cleanly when `tinygo` is not on `PATH`.
- **Smoke tests** — `cmd/prism/*_smoke_test.go` covers every CLI leaf end-to-end against fixtures. New CLI leaves require a smoke test.
- **Gallery freshness** — `docs/src/gallery/` SVGs are regenerated by `render/svg/goldens_test.go` outputs; gallery changes require a matching test fixture update.
- **MCP import firewall** — `internal/gates/mcp_firewall_test.go` asserts the SDK-free `mcp/` core never transitively imports an MCP SDK (`github.com/modelcontextprotocol/go-sdk` or `github.com/mark3labs/mcp-go`); the go-sdk binding stays quarantined in `mcp/gosdk/`. The companion `mcp/catalog_test.go` asserts every tool's input/output schema still reflects from struct tags via jsonschema-go.
- **Pulse-free import firewall** — `internal/gates/pulse_firewall_test.go` shells out to `go list -deps` over the whole host module (regular + `-test` graphs) and the `cmd/prismwasm` entry under `GOOS=js/GOARCH=wasm`, and fails if any dependency path matches `github.com/frankbardon/pulse` or its former expression engine `github.com/expr-lang/expr`. This locks in the eviction: the core never re-acquires Pulse (or expr-lang) on any platform.

## What NOT to Do

- **Do not put business logic in `cmd/prism/`.** CLI is a thin adapter — parse flags, construct library objects, call methods, format output. Smoke tests gate this discipline.
- **Do not bypass `afero.Fs`** for file access — defeats hermetic testing and the in-memory `prism serve` path.
- **Do not hand-edit golden files.** Regenerate via the per-package `-update` flag, and review the diff before committing.
- **Do not introduce an expression language.** `filter`, `calculate`, and condition `test` are structured JSON built-ins (`spec/predicate.go`, `spec/calc_expr.go`) — decode rejects a raw string outright. There is no `datum.` DSL, no Pulse-expression string, and no JS eval; do not add one. Extend the built-in grammar (new operator / function) via the spec union + schema `$def` + validate rule instead. Anything the grammar cannot express is precomputed by the caller upstream.
- **Do not import `service/` or `processing/` from `descriptor/`.** (Pulse rule; mirrored here: do not import `plan/` or `render/` from `validate/`, `compile/`, or `resolve/`. Predict / validate are no-execute by structural ban.)
- **Do not import an MCP SDK from the `mcp/` core.** The `mcp/` package is the SDK-free importable core (typed handlers + `ToolDescriptor` catalog); the `github.com/modelcontextprotocol/go-sdk` dependency is isolated to `mcp/gosdk/`. A core consumer must be able to mount the catalog without acquiring an MCP SDK transitively. Enforced by `internal/gates/mcp_firewall_test.go` (`go list -deps` over the core's import graph, including the `-test` graph).
- **Do not `fmt.Sprintf` JSON or SVG.** Use `encoding/json` and the scene → SVG emitters in `render/svg/`. Hand-formatted output drifts from goldens within one release.
- **Do not skip `make fmt-check && make lint` before committing.** CI will reject the PR — fix locally first.
- **Do not defer doc updates to a follow-up PR.** The follow-up will not happen. Update `docs/src/`, `schema/v1/`, and this file in the same PR per the Update Demand.
- **Do not add a transform without registering a Plan node.** `spec.Transform*` decoders, the union dispatcher in `spec/transform_union.go`, and the `plan/nodes/<name>.go` file must land together. Tests in `plan/dag_test.go` and `plan/builder.go` will fail otherwise.
- **Do not add an env var without updating "Build / Env"** above. New ceilings also land their default constant and parser in `internal/limits/limits.go`.
