# Prism Spec Validation — Rules + Layering

## Validation layering

Prism validates a spec in two stages, in this order:

1. **Shape validation** (`validate/shape.go`) — JSON Schema check using the
   embedded v1 bundle. Catches: unknown fields, missing required fields,
   wrong types, oneOf misses, enum violations, pattern mismatches.
2. **Semantic validation** (`validate/semantic.go`) — Go-side rules that
   need awareness of the dataset schema shim, the structured
   predicate/calculate grammar, or cross-field invariants. Returns
   `PRISM_SPEC_*` errors with fixup metadata.

A spec that fails shape never reaches the semantic stage; many semantic
rules assume well-formed structure.

## Why each rule lives where it does

| Rule | Layer | Why |
|---|---|---|
| Unknown fields | Shape | JSON Schema `additionalProperties: false`. |
| Missing required fields | Shape | JSON Schema `required`. |
| Enum membership (mark types, agg ops, scale types) | Shape | JSON Schema `enum`. |
| Numeric ranges (opacity 0..1, dimension ≥ 0) | Shape | JSON Schema `minimum`/`maximum`. |
| Pattern (snake_case dataset names) | Shape | JSON Schema `pattern`. |
| `oneOf` between composition keys | Shape | JSON Schema top-level `oneOf`. |
| `PRISM_SPEC_001` field exists | Semantic | Needs dataset schema-shim lookup. A `join` transform widens the checked field set with the right-hand (`with`) dataset's own fields (looked up the same way, by name) for `inner`/`left`/`outer` joins — `anti` keeps only the left schema. |
| `PRISM_SPEC_002` agg / field type compat | Semantic | Needs dataset schema-shim lookup. |
| `PRISM_SPEC_003` channel valid for mark | Semantic | Cross-field; per-mark allowlist table. |
| `PRISM_SPEC_004` selection ref resolves | Semantic | Cross-field check inside the spec. |
| `PRISM_SPEC_005` dataset ref resolves | Semantic | Cross-field check inside the spec. |
| `PRISM_SPEC_006` predicate / calculate parses | Semantic | Validates the structured predicate/calculate grammar. |
| `PRISM_SPEC_007` scale type compat with field type | Semantic | Needs field type lookup. |
| `PRISM_SPEC_008` pie/donut requires theta + color | Semantic | Cross-field rule per mark type. |
| `PRISM_SPEC_009` `$schema` references known schema | Semantic | Requires the bundle URN registry. |
| `PRISM_SPEC_019` selection encoding channel is bound | Semantic | Cross-field check: walks selection.encodings and matches against bound channels. |
| `PRISM_SPEC_020` interval selection encodings are position channels | Semantic | Spec-internal allowlist (x/y/x2/y2/theta). |
| `PRISM_SPEC_021` geo projection / geo-mark channel bindings | Semantic | Cross-field rule for geoshape + geopoint; ensures `projection.type` is known and the matching encoding channels are bound. |
| `PRISM_SPEC_022` animation easing name is known | Semantic | Enum check against `spec.AnimationEasings`. Lives at the semantic layer so the error message can suggest the full easing list. |
| `PRISM_SPEC_023` animation declares a join key | Semantic | Cross-field rule: when `animation` is set, at least one descendant encoding channel must carry `key: true`. |
| `PRISM_SPEC_024` animation join key is unique | Semantic | Cross-field rule: at most one encoding channel per spec node may carry `key: true`. Composite keys deferred. |
| `PRISM_SPEC_025` condition selection reference | Semantic | Every `selection` name in a channel `condition` clause must resolve to a declared selection in the same spec scope. |
| `PRISM_SPEC_026` condition test parses | Semantic | Every `test` predicate in a channel `condition` clause must parse via the structured predicate grammar. Mirrors `PRISM_SPEC_006`. |
| `PRISM_SPEC_027` condition value-or-binding | Semantic | A condition entry must carry exactly one of `value` or `field`; a `selection`-form entry with neither inherits the channel's own field binding. |
| `PRISM_SPEC_035` regression shape | Semantic | A `regression` transform must declare `target` + at least one predictor. It accepts derived input (may follow another transform, like crosstab), so there is no chain-position constraint. |
| `PRISM_SPEC_036` bullet bands strictly ascending | Semantic | A `bullet` mark's `bands` are cumulative range bounds from zero, so each bound must be strictly greater than its predecessor. Fires per out-of-order pair. |
| `PRISM_SPEC_040` table requires columns | Semantic | A `table` mark has no x/y — its `encoding.columns[]` is the entire visual contract, so it must be present and non-empty. |
| `PRISM_SPEC_041` scale domain shape | Semantic | An explicit `scale.domain` must match its scale family: two ascending numeric bounds on a continuous scale, two date-string / epoch-ms bounds on a time scale, a non-empty list of string categories on band/point/ordinal. Walks composition children. The family comes from `scale.type`, falling back to the channel's measure type; an unknown family no-ops and the encoder raises the same code at resolve time. |
| `PRISM_SPEC_042` span channel supported | Semantic | A bound `x2` / `y2` must be geometry the mark can draw: `bar`, `rect` and `rule` range on either axis, `area` takes `y2` as its lower edge, every other mark is rejected instead of silently dropping the binding. Also fires when a span channel has no base channel (`x2` without `x`) or names no field. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_043` span channel type match | Semantic | A span channel is measured on its base channel's scale, so `x2.type` must equal `x.type` and `y2.type` must equal `y.type`. Skipped when either type is absent. |
| `PRISM_SPEC_044` axis orient channel | Semantic | `axis.orient` must name a side the channel's axis can occupy: `bottom` / `top` on `x` (and `x2`), `left` / `right` on `y` (and `y2`). Orient moves both the axis and the padding its side reserves, so a cross-axis value is an author error rather than an inert typo. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_046` mark orient supported | Semantic | A declared `mark.orient` must be a value some mark draws. The cartesian families — `bar`, `rect`, `area`, `tick`, `boxplot`, `violin`, `winloss` and the `sparkbar` / `sparkarea` wrappers — read `vertical` / `horizontal` as which axis carries the category vs the measure; `tree` / `dendrogram` / `network` read the same pair as the direction their layout grows; every other mark type rejects the property instead of ignoring it. `bullet` is excluded on purpose (it keeps its own `orientation` field, a whole-mark rotation with the opposite default), and so is `heatmap` (banded on both axes at once, so there is no category/measure split to swap). `radial` is vocabulary-only — no mark implements it — so it is rejected everywhere. Whether an explicitly requested orientation is *drawable* against the spec's actual scales is an encode-time question (`PRISM_ENCODE_001`), not this rule's. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_061` progress mark structure | Semantic | A `progress` mark draws one metric row per data row — a value bar on a full-scale track — so both position channels must be bound, one discrete (the row labels) and one quantitative (the value). `thickness` is a fraction of the category band and must be in (0, 1]; a literal `total` (the measure ceiling the track runs to) must be positive. Structural only: whether an explicitly requested orientation is drawable is an encode-time question (`PRISM_ENCODE_001`), and whether `orient` is vocabulary a mark draws is `PRISM_SPEC_046`. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_053` stack center mark | Semantic | `stack: "center"` — the streamgraph offset, which floats each stack's baseline so the band is symmetric about zero — is drawable only by `area`, which carries both of its own edges. On a baseline-anchored mark (`bar`) centring detaches every column from the axis it is measured against and the tick labels stop naming values, so it is rejected rather than drawn. `zero` and `normalize` are unaffected on bars. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_045` scale range position | Semantic | `scale.range` is honoured on color channels only. Declaring it on a position channel (`x`, `y`, `x2`, `y2`, `theta`, `radius`) is rejected: a position scale's range is the plot rect `encode/layout.go` computes, and the axis, the gridlines and every mark measure against that same rect — a spec-supplied range would move the marks without moving the chrome. Bound the axis with `scale.domain` / `zero` / `nice`, or size the rect with `width` / `height` / the band paddings. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_049` scale band geometry | Semantic | The band-geometry knobs must sit inside the ranges their geometry is defined over: `padding`, `padding_inner` and `padding_outer` are fractions of the step and live in [0,1), and `align` is a position in the leftover slack and lives in [0,1]. The encoder pins an out-of-range value rather than drawing an inside-out band, so the rule is what keeps the author from getting silently different geometry. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_055` order channel shape | Semantic | Every `encoding.order` entry must name a `field`, and its `sort` direction must be one Prism recognises — `ascending` / `descending`, or the `asc` / `desc` aliases. Both halves close a silent no-op: a fieldless entry has nothing to compare, and an unrecognised direction would otherwise sort ascending and look intentional. Walks layer / concat / facet / repeat children. |
| `PRISM_SPEC_054` composite parent encoding | Semantic | An `encoding` block may not sit beside a composition operator (`layer`, `concat`, `hconcat`, `vconcat`, `facet`, `repeat`). A parent passes only `data`, `datasets` and `$schema` down to its children, and the composite encoder reads `child.Spec.Encoding` and nothing else, so a parent-level block changes no rendered byte. Vega-Lite does inherit a parent encoding into layer children, which is exactly what makes the block look like the way to share axis config — rejecting it stops the spec from lying without foreclosing real inheritance later. The `spec` child of a `facet` / `repeat` parent is untouched (that child's encoding is the chart that is drawn), as is any flat spec, including the `encoding.row` / `encoding.column` facet shorthand. Walks the whole tree, so a composite nested inside another composite is reached; fires once per offending node. |
| `PRISM_SPEC_056` order channel aggregate | Semantic | An `encoding.order` entry may not declare its own `aggregate`. Ordering by a per-series total needs a second aggregation at a different granularity than the chart's own, joined back onto the rows; Prism rejects the key rather than accepting and ignoring it. Precompute the total with a `window` / `join` transform, name a field another channel already aggregates, or use a `sort` transform instead. Walks layer / concat / facet / repeat children. |

## $ref resolution strategy

Each schema file declares `$id: urn:prism:schema:v1:<name>`. Cross-file
`$ref`s are authored as relative paths (`data.schema.json#/$defs/data`)
per the format rules in `.planning/design/03-spec-format.md`.

Because `santhosh-tekuri/jsonschema/v6` resolves relative refs against the
document's `$id`, relative refs cannot resolve naturally when `$id` is a
URN — there is no notion of "next to" within the `urn:` namespace.

Choice: at compile time, the validator walks each loaded schema document
and rewrites every relative `$ref` (`name.schema.json[#…]`) to its URN
form (`urn:prism:schema:v1:name[#…]`). Intra-file refs (`#/$defs/x`) and
refs already in URN form are left untouched. The on-disk source keeps the
human-friendly relative-ref form; only the in-memory compile graph uses
URNs.

This keeps:

- The on-disk schemas portable across editors that resolve relative refs.
- The runtime resolver consistent: every cross-file reference becomes a
  URN before the JSON Schema engine sees it.

Alternative considered: register each file under a synthetic
`https://prism.local/v1/<name>.schema.json` URL and resolve relative refs
that way. Rejected because it would diverge from the design-doc rule
that `$id` is the canonical URN.

## Adding a new semantic rule

1. Add a file `validate/rules/<short_name>.go` implementing `SemanticRule`.
2. Register it in `validate/rules/init.go` via `validate.RegisterDefault(...)` (loaded by `init()`) — do not edit existing rule files.
3. Add a `PRISM_SPEC_xxx` entry in `errors/codes.go` with `Message`,
   `Fixups`, and any `SeeAlso` links.
4. Add a positive + negative fixture under `testdata/specs/` and
   `testdata/specs/invalid/`.
5. Update the table above.
