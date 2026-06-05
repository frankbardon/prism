# Changelog

## v0.3.0 — 2026-06-05

Additive feature release. v0.2 spec and rendering semantics preserved
where backwards-compatible; built-in theme defaults refreshed (see
**Theme system v2** below) so SVG goldens drift by design.

### Theme system v2

- **Nested `theme.Theme` blocks** — `Mark` (global default), `Marks`
  (per-mark-type), `Axis`, `Legend`, `Title`, `View`, `Range` (per
  scale-role scheme defaults), `Schemes` (custom named-scheme
  registry), `Style` (Vega-Lite-style named-style registry), `States`
  (selected / deselected / hover overlays). Legacy flat fields kept
  for back-compat with v0.2 theme JSON.
- **49-scheme catalogue** — full d3-scale-chromatic taxonomy
  (`tableau10`, `category10`, `observable10`, `viridis`, `magma`,
  `plasma`, `inferno`, `cividis`, `rdbu`, `spectral`, ...) plus four
  accessibility additions (`okabe_ito`, `tol_bright`, `tol_vibrant`,
  `tol_muted`). Reference any scheme by name in `scale.scheme` or
  `theme.range.*.scheme`.
- **Two new built-in themes** — `high_contrast` (bold black/white,
  no grid lines, projector / low-vision use) and `colorblind`
  (Okabe-Ito categorical + Cividis sequential). `light` / `dark` /
  `print` refreshed with full nested-block coverage.
- **CSS variable surface ~4× wider** — every nested-block token
  emits a `--prism-*` variable (e.g. `--prism-mark-bar-fill`,
  `--prism-axis-tick-size`, `--prism-view-bg`). Post-hoc theming can
  override any token without re-rendering.
- **`spec.theme` retypes** — `ThemeOverride` mirrors the nested
  `theme.Theme` shape 1:1 (typed `Mark`, `Marks`, `Axis`, `Legend`,
  `Range`, etc.). The stub `spec.Config` is removed (was never
  wired). `schema/v1/theme.schema.json` types every block.
- **Two validate rules** — `PRISM_SPEC_030` (unknown scheme name in
  `scale.scheme` or `theme.range.*`), `PRISM_SPEC_031` (unknown mark
  type in `theme.marks`).

### Crosstab transform

- **`{transform: [{crosstab: {...}}]}`** delegates row × column ×
  cell-aggregation to Pulse's new `Request.Crosstab` section
  (Pulse v0.13). Margins (`rows`, `columns`, `grand`) + normalisation
  (`none`, `row`, `column`, `total`) supported. Returns long-form
  rows ready for the heatmap encoder.
- **Position constraint** — crosstab must be the **first** transform
  on the chain. Pulse has no in-memory cohort constructor, so it
  cannot follow a Prism filter / aggregate / join. The plan node
  opens the `.pulse` cohort directly (mirroring `PulseChainNode`).
- **Three validate codes** — `PRISM_SPEC_032` (shape: rows/cols/cell
  required), `PRISM_SPEC_033` (position rule), `PRISM_SPEC_034`
  (normalize enum). Plus runtime `PRISM_PLAN_CROSSTAB_REQUIRES_SOURCE`
  / `PRISM_PLAN_CROSSTAB_PROCESS`.

### Mark fixes

- **Bar mark consumes color channel** — previously every bar
  rendered with the default `theme.Marks.bar.fill` regardless of the
  categorical color binding, so bars + legend swatches disagreed.
  Now mirrors the point / funnel pattern (per-row
  `lookupCategoryColor`).
- **Heatmap normalises rect bounds** — y-axis band step is negative
  (range goes `plot.bottom → plot.top`), so cells were rendering
  with negative `height` and SVG was skipping them. Heatmap now
  flips origin + sign when `w/h < 0`; cells are visible.

### Dependencies

- `github.com/frankbardon/pulse` 0.10.2 → 0.13.1
- `golang.org/x/sys` 0.44.0 → 0.45.0 (transitive)

## v0.2.0 — 2026-05-27

Additive feature release. All v0.1 spec and rendering semantics preserved.

### New marks + transforms

- **Geographic marks** — `geoshape` (choropleth polygons) and
  `geopoint` (lon/lat overlays) with five projections (mercator,
  equirectangular, naturalearth, albers_usa, orthographic). Embedded
  Natural Earth catalog (~100 KB manifest) for host builds; WASM
  fetches tier bundles from `${origin}/static/prism/geodata/`
  (override via `prism.geo.setBundleURL`).
- **Tree / hierarchy marks** — tidy + radial layouts (see
  `concepts/marks.md`).

### Planning + execution

- **PulseChainFusion optimizer pass** — collapses source-rooted
  linear chains (`Filter` / `Calculate` / `GroupAggregate` / `Sort`)
  into one `pulse.ProcessChain` call so Pulse pushes filters down at
  the cohort reader and Prism never materialises the source table.
  Falls back to per-node execution with `PRISM_PLAN_CHAIN_NOT_MERGEABLE`
  on chain-gate trip.

### Animation

- **`spec.animation` block** — duration, delay, easing (linear,
  ease, ease-in, ease-out, ease-in-out, cubic-bezier), stagger, and
  per-channel overrides. Scene IR surface in `encode/scene/animation.go`.
- **Web component tween engine** — `static/vendor/prism/prism-animator.mjs`
  drives numeric + color attribute interpolation on the live SVG.
- **Structural-mismatch fallback** — `PRISM_WARN_ANIM_FALLBACK`
  emitted when before/after scenes can't be tweened element-wise;
  renderer cross-fades instead.
- **Gallery + playground demo** under `docs/src/gallery/animation/`.

### Spec polish (tier 1)

- **Conditional channel encodings** — `condition: {test, value}`
  on channels (`PRISM_SPEC_025/026/027`); compiled to
  `ConditionalAttr` in the scene IR.
- **Per-column null handling** — `table.Column.IsNull` / `NullCount`
  consulted by every aggregate, scale, mark, and transform;
  `PRISM_WARN_NULL_*` warnings on drop / skip.
- **PDF polish** — improved font metrics, paginate ergonomics.
- **Versioned docs** — `docs/src/` carries an explicit version
  marker for downstream pinning.

### Post-v1 upgrades

- **Structured selection events** (`selection/`) — uniform `Event`
  struct (`scene_id`, `selection_id`, `kind`, `marks`, `data_rows`,
  `data_extent`, `pixel_extent`, `spec_path`) across Go, WASM, and
  Twirp. Stable `instance_key` derived from `(layer_id, row_id)`.
  `prism:select` CustomEvent.detail now conforms; legacy `id` /
  `state` keys retained.
- **Compile-only mode** (`prism.Compile` / `prism.compile`) — Go +
  WASM API returning a `CompiledPlan` (marks, scales, data, layout,
  diagnostics + canonical Scene) without rasterising. Typically
  10–50× faster than `execute` + `render`.
- **Runtime data references** (`spec.Data.Ref` + `resolve.DataResolver`)
   — new `{data: {ref: "<name>"}}` spec variant resolved by a
  caller-supplied resolver. Browser hook: `prism.setDataResolver(fn)`.
  Same spec renders in server / browser / test without modification.
  Unresolved refs surface as `PRISM_RESOLVE_REF_UNRESOLVED`.
- **Spec patches (RFC 6902)** — `prism.ApplyPatch` / `prism.DiffSpecs`
   + stateful `prism.Scene` wrapper. Atomic; failing op index in
  `PRISM_SPEC_PATCH_001`. WASM exports `prism.applyPatch` /
  `prism.diffSpecs`.

### Docs

- Cookbook recipes for the four post-v1 upgrades.
- Concept doc expansions: `concepts/encoding.md` (conditions),
  `concepts/multi-source.md` (nulls + runtime refs), `concepts/geo.md`,
  `concepts/browser.md` (animation + data resolver).

## v0.1.0 — 2026-05-20

First public release. Seventeen phases of work delivering:

### Pipeline

- **Spec types + JSON Schema validator** with 21 semantic rules and
  fixup-templated error envelopes.
- **Resolver + Table + Source node** reading `.pulse` files via
  `afero.Fs` (local + archive-shard refs; GCS deferred behind
  `PRISM_RESOLVE_GCS_UNAVAILABLE`).
- **Plan + DAG + sequential/parallel executor** with bounded worker
  pool, partial-failure policy, LRU table cache, and 5 optimizer
  passes (DedupSources, FilterPushdown, ProjectionPruning,
  AggregateFusion, SampleInjection).
- **Pulse compiler** mapping 18 friendly aggregate aliases to Pulse
  AGG_* constants (6 cohort-analytics aliases — wmean, ratio, lift,
  share, ci0, ci1 — implemented client-side until Pulse upstreams).
- **Hash join** (inner/left/outer/anti) + union, with cardinality
  ceiling `PRISM_JOIN_MAX_ROWS`.

### Encoding + rendering

- **Scene IR** (Go-only, stable JSON for JS port) covering 9 geom
  types: Rect, Line, Area, Point, Rule, Arc, Text, Path, Image.
- **8 scale types** — linear, log, pow, sqrt, time, band, point, ordinal.
- **Axis polish** — 4 orientations, major + minor ticks, grid toggle,
  label rotation + overlap handling, d3-format subset.
- **Legends** — symbol + gradient swatches, 8 positions.
- **Theme system** — 3 built-in themes (light, dark, print); CSS
  variable manifest emitted into SVG output; sparse overrides at
  spec level; custom `theme.json` loader.
- **SVG renderer** (Go) — pinned 3-decimal precision, viewBox +
  responsive sizing, layered group structure.
- **PDF renderer** (`signintech/gopdf`) — vector throughout, embedded
  Inter + JetBrains Mono fonts, `--paginate` for multi-page grids.

### Marks

20 marks across three families:

- **Basic** — bar, line, area, point, rule, text, tick, rect, arc.
- **Composite** — histogram (auto-bin), heatmap (2D bin), boxplot
  (IQR + Tukey whiskers), violin (Epanechnikov KDE + Silverman
  bandwidth), pie (share expansion), donut.
- **Specialty** — sankey (depth-first layout), funnel (stacked
  trapezoids), sparkline (axes-stripped), image (data: URLs only),
  path (raw SVG `d` passthrough).

### Composition

- `layer`, `concat`, `hconcat`, `vconcat` with cross-layer scale
  resolution (shared/independent) and `PRISM_PLAN_005` for
  incompatible types.
- `facet` (data-value partitioning, shared upstream + encode-time
  split, recursive nested) and `repeat` (field-name substitution
  via `{repeat: "row|column"}`).

### Browser + JS port

- Vendored D3 modules (8 modules, pinned versions with sha256
  manifest).
- `prism.mjs` SceneDoc → SVG renderer (cross-impl byte-parity with
  Go renderer on 5 curated fixtures).
- `<prism-chart>` + `<prism-dataset>` + `<prism-coordinator>` web
  components.
- `prism-resolver.mjs` page-level dataset registry (fetch dedupe).
- `prism-selection.mjs` selection state plumbing.
- Cross-impl test harness via Node + happy-dom (gated by
  `PRISM_CROSS_IMPL=1`).

### Selections

- Point + interval selections with point/brush hit-testing.
- Client reactive mode (DOM class toggle, no network) +
  server reactive mode (re-plan via `/prism/scene`).
- `<prism-coordinator>` cross-chart selection broadcast.
- URL hash state round-trip with localStorage fallback.

### Service surface

- **Twirp** service at `/twirp/prism.v1.Prism/` with 5 RPCs (Plot,
  Validate, Scene, Plan, ListDatasets). Error interceptor maps
  PRISM_* + Pulse codes to Twirp status.
- **MCP** stdio server with 4 agent tools (prism_plot, prism_validate,
  prism_describe, prism_examples_search).
- **OTel** bridge via opt-in env (`PRISM_OTEL_ENABLED=1`) — no hard
  SDK dep.

### CLI

`prism validate | plan | execute | plot | scene | serve | mcp |
inspect | examples | schema | init | errors lookup | static-bundle |
version` — all with `--help`.

### Bootstrap + docs

- `prism init` writes `.prism/{schemas,examples,editor,README.md}`.
- 59-fixture gallery across 8 categories.
- Concept docs, cookbook recipes, Vega-Lite migration guide.
- Plain Markdown — no build pipeline.

### Decisions

90+ locked design decisions in `.planning/DECISIONS.md`. See
`.planning/STATE.md` for full phase log.
