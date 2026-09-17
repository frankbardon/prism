# Changelog

## v0.17.0 — unreleased

Additive feature release: the offset (dodge) position channels, plus a
working `unpivot`. v0.16 spec and rendering semantics are preserved for
every spec that does not bind an offset — an unbound offset channel
resolves to nothing, injects no node, builds no scale, and every
committed golden that predates it is byte-identical. Two changes do
alter what an existing spec does; both are called out under
**Behaviour changes** below.

### Offset (dodge) position channels

- **`x_offset` / `y_offset`** — a second discrete binding that
  subdivides one category's band slot, so the rows sharing a category
  are drawn side by side instead of on top of one another. Vega-Lite's
  `xOffset` / `yOffset` under Prism's snake_case rule. Bind one beside a
  banded `x` (or `y`) and the mark draws grouped columns:

  ```json
  {"mark": "bar", "encoding": {
    "x": {"field": "metric", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "score", "type": "quantitative"},
    "x_offset": {"field": "series", "type": "nominal"}
  }}
  ```

- **`bar` only.** Every other mark **rejects** the channel
  (`PRISM_SPEC_063`) rather than ignoring it. That rejection is
  load-bearing, not defensive: every band-seated mark reaches the same
  slot geometry, so without it a `tick` or a `heatmap` with an offset
  would quietly draw dodged shapes no mark documents.
- **Sub-band order**, highest precedence first: `scale.domain`, then a
  `sort` naming the categories outright, then a `sort` direction (all
  four spellings), then the distinct values **ascending**. Ascending —
  rather than the first-seen order the x / y band scales use — is a
  considered divergence: an offset decides which sub-band a series
  occupies, and deriving that from row order would make the same data
  draw differently after an upstream re-sort. It is stable for a given
  set of values, **not across datasets**, so a chart whose category set
  varies between renders should pin `scale.domain` on the offset
  channel.
- **Offset scale padding defaults to inner 0 / outer 0** (the parent
  band keeps 0.1 / 0.05 / 0.5), so sub-bands touch and together fill the
  slot. That zero is what reduces a single distinct offset value to one
  sub-band spanning the whole slot — i.e. byte-identical to the channel
  being absent.
- **Horizontal dodging reads mirrored against Vega-Lite.** Prism's y
  band runs bottom-to-top, so with `y_offset` the first offset category
  takes the sub-band at the slot's **lower** pixel edge, the same
  direction the parent assigns its own categories.
- An offset scale reads `domain`, `padding`, `padding_inner`,
  `padding_outer`, `align`, `round` and `reverse`. Every other `scale`
  key on an offset channel now reports itself as
  `PRISM_WARN_SCALE_FIELD_INERT` instead of being discarded in silence.
- New gallery fixtures: `basic-marks/grouped_bar`,
  `basic-marks/grouped_bar_horizontal`,
  `transforms/unpivot_grouped_bar`.

### Composition

- **`resolve.scale.x_offset` / `y_offset`** — offsets are **shared by
  default** across `layer` children and `facet` cells, like x / y, so a
  series occupies the same sub-band in every panel it appears in. Opt
  out per channel with
  `{"resolve": {"scale": {"x_offset": "independent"}}}`.
- `concat` / `hconcat` / `vconcat` / `repeat` keep **independent**
  offsets. Those operators share no position scales at all today, so
  sharing an offset there would mean sharing x / y there first.
- Children that describe the shared offset differently fold
  **first-specified-wins per property** over `sort` and every property
  of the offset channel's `scale` block, and raise
  `PRISM_WARN_OFFSET_CONFIG_CONFLICT`. A conflict is never resolved
  silently. (`field` / `type` are taken from the first binding child and
  are not conflict-reported — the values are unioned regardless of which
  column they came from, the way a shared x scale unions layers binding
  different fields.)

### Transforms

- **`unpivot` executes** — wide → long, the Vega-Lite `fold` analogue,
  and the usual way to get several wide metric columns onto one
  categorical axis. It decoded, validated and built a plan node before,
  then failed at execute: the node carried no backend wiring, so its
  dispatch arm was unreachable. Now: row count is
  `input_rows × len(unpivot)`, checked against `PRISM_TABLE_MAX_ROWS`
  before anything is materialised; row order is row-major, so a source
  row's outputs stay adjacent; nulls survive as nulls rather than being
  dropped or coerced to zero; and a non-numeric source column, or an
  `as` name colliding with a carried column, is refused with
  `PRISM_COMPILE_002` naming the column.
- **`pivot` is the only transform that parses but cannot execute.** It
  is now rejected at validate (`PRISM_SPEC_067`, below) instead of
  failing mid-pipeline. Use `crosstab` for the same long → wide shape.

### Validation and diagnostics

New error codes:

| Code | Fires when |
|---|---|
| `PRISM_SPEC_063` | an offset channel is bound on a mark that cannot dodge — anything but `bar` |
| `PRISM_SPEC_064` | the offset binding is incoherent: the matching position channel is not banded, or both `x_offset` and `y_offset` are bound |
| `PRISM_SPEC_065` | an explicit `stack` is written beside a bound offset |
| `PRISM_SPEC_066` | an offset and a span channel (`x2` / `y2`) are bound on the same axis, which would leave the offset nothing to subdivide |
| `PRISM_SPEC_067` | a transform the spec grammar accepts but no backend can execute |

New warnings:

| Code | Fires when |
|---|---|
| `PRISM_WARN_OFFSET_COLLISION` | two or more rows repeat one (category, offset) pair, so their marks share a sub-band and only the last drawn stays visible |
| `PRISM_WARN_OFFSET_CONFIG_CONFLICT` | composition children disagree about a shared offset scale; first-specified wins |

`PRISM_WARN_SCALE_FIELD_INERT` gained coverage of an offset channel's
`scale` block (see above). `PRISM_COMPILE_001`'s catalogue text was
rewritten: it described a phase rollout that finished long ago and
pointed at a planning file that no longer exists.

Warnings ride on `SceneDoc.Warnings` / `CompiledPlan.Diagnostics`.
`prism plot` and `prism scene` print them to stderr and `prism scene`
also carries them in the document's `warnings` array — but a library
embedder that renders `CompiledPlan.Scene` without reading
`Diagnostics` sees none of them. Read the field.

### Behaviour changes

Two changes alter what an already-valid spec does.

1. **Binding an offset suppresses implicit stacking.** A bar spec with
   an aggregated measure and a bound grouping channel stacked before;
   adding `x_offset` / `y_offset` now dodges instead. Stacking and
   dodging spend the same geometry on the same grouping, so one of them
   has to yield, and the explicit request wins. The suppression lives in
   `spec.ResolveStack` — the single decision point the planner and the
   encoder share — so the two stages cannot disagree. It yields to the
   **inferred** stack only: an explicit `stack` written beside an offset
   is rejected as `PRISM_SPEC_065` rather than silently dropped.
2. **`PRISM_SPEC_067` moves an existing execute-time failure to validate
   time.** It creates no new failure. Every spec it rejects already
   failed, further down the pipeline, as `PRISM_COMPILE_001` naming an
   internal node kind the author never wrote. **`pivot` is the only
   transform affected** — `join` and `union` execute (their `Execute`
   bodies live on the plan nodes rather than in the in-memory backend,
   which is why they are absent from its dispatch table), and `unpivot`
   executes as of this release.

### Go API

- **`spec.OffsetChannel`** — the `x_offset` / `y_offset` channel type
  (`field`, `type`, `sort`, `scale`), bound on `spec.Encoding.XOffset` /
  `.YOffset`.
- **`spec.ResolveOffset(*spec.Encoding) *spec.OffsetBinding`** — the
  single decision point for whether an offset is bound and on which
  axis, called by both the plan builder and the encoder. An offset
  carrying no `field` binds nothing; an offset bound on both axes is
  refused outright rather than half-honoured.
- **`encode.OffsetDomain`** — a resolved sub-band category order plus
  band options. It is not a `Scale`: an offset scale's range is the
  parent band width of the cell being drawn, which is only knowable per
  child.
- **`encode.EncodeOpts.OverrideOffset`** — hands a composition child the
  shared offset domain, mirroring `OverrideXScale` / `OverrideYScale`.
- **`spec.ResolveChannelMap`** gains `x_offset` / `y_offset`.

### Docs

- `concepts/encoding.md` — Offset channels: the global (not
  per-category) sub-band domain, padding, ordering and its
  cross-dataset caveat, duplicate sub-bands.
- `concepts/marks.md` — Grouped bars (dodging).
- `concepts/composition.md` — Offset (dodge) scales: what shares, what
  does not, and how conflicts fold.
- `concepts/spec.md` — the `unpivot` transform, a wide → long → grouped
  bar worked example, and `pivot` as the one transform that parses but
  does not execute.
- `migration-from-vega-lite.md` — the `xOffset` → `x_offset` rename, the
  mirrored horizontal sub-band direction, and stack-vs-dodge.

## v0.4.0 — 2026-06-18

Additive feature release surfacing Pulse v0.22 capabilities as Prism
visualization features (PR #27, seven slices). v0.3 spec and rendering
semantics preserved; all new spec vocabulary is opt-in.

### Aggregate aliases

- **Distribution-shape scalars** — `range`, `skewness`, `kurtosis`,
  `null_count` aliasing Pulse `AGG_RANGE/SKEWNESS/KURTOSIS/NULL_COUNT`.
  skewness/kurtosis use the population (Fisher-Pearson, excess) forms;
  `null_count` is universal (any field type). The in-memory backend
  computes them client-side with parity against `pulse.Process`.
- **`frequency`** — the scalar companion to `mode`: the modal count
  (how many times the most frequent value occurs). Universal compat.
  Rides the existing F64 pipeline; the per-value cardinality map stays
  on Pulse's meta surface, so no new column kind is introduced.

### Transforms

- **`regression`** — source-rooted OLS leaf (`REG_OLS`). Synthesises the
  two fitted endpoints (intercept + slope·x) so a trend line composes as
  a `line` layer over a scatter. v1 single-predictor. New
  `PRISM_SPEC_035` (must be the first transform) +
  `PRISM_PLAN_REGRESSION_REQUIRES_SOURCE` / `_PROCESS` codes.
- **`timeunit`** — client-side calendar truncation (year / quarter /
  month / week-ISO / day → date period start) via epoch arithmetic.
  Composes anywhere, no Pulse leaf.

### Crosstab

- **Date groupers** — `crosstab` group `type: "date"` + `period`
  (year / quarter / month / week / day / day_of_week) emitting Pulse
  `GROUP_DATE` calendar buckets.
- **Overlays → color** — per-cell `share_of_row`, `share_of_col`,
  `index_vs_margin`, `zscore_vs_margin` overlays joined coordinate-
  aligned into long rows, bindable to the `color` channel.

### Marks

- **Heatmap opacity channel** — field-driven per-cell opacity
  (linear over [min,max] → [0.15, 1.0]); pairs with the
  `zscore_vs_margin` overlay for significance shading.

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
