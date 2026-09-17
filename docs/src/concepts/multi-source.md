# Multi-source

Composing N materialized datasets into one chart is a first-class workflow.

## Datasets block

Each named dataset carries its rows inline via `values` (or defers them
to a runtime `ref` resolved by a `DataResolver` — see below). Prism does
not open `.pulse` files; the host materialises the rows and hands Prism a
Pulse-free spec.

```json
{
  "datasets": {
    "current": {"values": [{"brand_id": "a", "score": 0.62}, {"brand_id": "b", "score": 0.55}]},
    "prior":   {"values": [{"brand_id": "a", "score": 0.58}, {"brand_id": "b", "score": 0.57}]},
    "bench":   {"ref": "industry_benchmark"}
  },
  "transform": [
    {"data": "current", "groupby": ["brand_id"],
     "aggregate": [{"op": "mean", "field": "score", "as": "current_score"}],
     "as": "current_agg"},
    {"data": "prior", "groupby": ["brand_id"],
     "aggregate": [{"op": "mean", "field": "score", "as": "prior_score"}],
     "as": "prior_agg"},
    {"join": {"left": "current_agg", "right": "prior_agg", "on": "brand_id"},
     "as": "joined"}
  ],
  "layer": [...]
}
```

`transform.data` selects an input by alias. `transform.as` publishes
the transform's output under a new alias.

## Join

In-memory hash join. Kinds: `inner` (default), `left`, `outer`, `anti`.

```json
{
  "join": {
    "left":  "current_agg",
    "right": "prior_agg",
    "on":    ["brand_id", "region"],
    "kind":  "left"
  },
  "as": "joined"
}
```

Memory ceiling: `PRISM_JOIN_MAX_ROWS = 5_000_000` (env-overridable).
Exceeding it raises `PRISM_JOIN_003` with a fixup pointing at
pre-aggregation, push-to-Pulse, or env override.

### Null handling

`left` and `outer` joins surface unmatched cells as **null**, not as
the type's zero value. Downstream consumers see the absence of data
instead of a silent `0.0` / `""` / `false` that would look like a
genuine measurement:

| Op | Null policy |
|---|---|
| `count` | `count(*)` counts every row; `count(field)` skips nulls. |
| `sum`, `mean`, `min`, `max`, `median`, `q1`, `q3`, `stdev`, `variance`, `ci0`, `ci1` | Skip nulls. |
| `distinct`, `mode` | Skip nulls. |
| `wmean`, `ratio`, `lift`, `share` | Skip nulls. |
| `filter` predicates | Rows where any input is null evaluate to false (matches pandas / Vega-Lite). |
| `calculate` expressions | Any null input propagates to a null output. |
| Inline type inference | A null carries no type. Each column takes its kind from its **first non-null value**, wherever that value sits — a leading `null` no longer decides the column. |
| Inline column with no non-null value | Nothing to infer from, so it resolves to the categorical (string) fallback rather than erroring. Every row is flagged null, so the kind never holds a value. Declare `data.fields` if a specific kind matters. |
| Inline column order | The union of every row's keys, alphabetical. A field that first appears in a later row still becomes a column, in the slot it would have held had row 0 carried it; the rows above it are null. |
| Encoding: `x`, `y` (scale-bound) | The whole row is dropped before scale resolution. See below. |
| Encoding: `color`, `opacity`, `text`, `tooltip`, `detail`, `theta`, the sankey / geo bindings | The row is kept. A null there falls back to the channel's default (default fill, empty label, empty tooltip line) rather than removing a mark. |

An explicit `data.fields` declaration bypasses inference entirely and
wins over any observed value. Inference only applies when `data.fields`
is absent, and it still rejects a genuinely mixed column — a real string
arriving in a column inferred as numeric raises
`PRISM_RESOLVE_INLINE_TYPE_MISMATCH` with the offending row and field.

### Nulls at encode time

A **scale-bound** channel is one whose raw field values are handed to a
resolved scale — today exactly `x` and `y`, and only for the marks that
go through the standard cartesian scale resolution. Polar (`arc`,
`pie`, `donut`), self-scaling (`histogram`), specialty (`sankey`,
`funnel`, `path`, `tree`, `dendrogram`, `network`) and geographic
(`geoshape`, `geopoint`) marks build their own geometry and never hand
a raw field value to a scale, so they are not filtered.

Before any scale is resolved, the encoder drops every row carrying a
null in a scale-bound channel and emits `PRISM_WARN_NULL_DROPPED`
carrying the dropped-row count and the offending channel names. The
surviving rows render normally:

```
WARN PRISM_WARN_NULL_DROPPED: 1 rows skipped: encoding channels y carried null values.
```

Because the filter runs on the table rather than inside each mark
encoder, everything downstream sees one consistent row set — scale
domains, colour categories, the `color` / `detail` row partitioner,
tooltips, datum back-references, category styles and conditions all
stay aligned. In a `layer` composite the policy runs per layer, and
the warning's `layer` field names the layer that shed rows.

A row that is null **only** in a non-scale-bound channel is kept — a
missing tooltip line is not a reason to delete a mark.

Dropping *some* rows is a warning; dropping *all* of them is an error.
If a bound field is null in every row there is nothing left to draw, so
the encoder fails with `PRISM_ENCODE_NULL_ALL_ROWS` rather than
emitting a silently empty chart.

Prism drops null rows rather than breaking a line or area into
separate segments around the gap (Vega-Lite's behaviour). A three-point
series with a null in the middle therefore renders as one straight
segment from the first point to the third.

### Choosing what happens — `mark.invalid`

`mark.invalid` picks between the two behaviours. It defaults to
`"filter"`, which is what every version before v0.16 did.

| Value | Effect |
|---|---|
| `"filter"` | Drop the row. Its category leaves the scale domain with it, so the axis never mentions it and a line closes over the hole. The default. |
| `"break"` | Keep the row. Its category holds its slot on the axis, no mark is drawn for it, and a line or area splits into separate segments either side of the gap. |

```json
{"mark": {"type": "line", "invalid": "break"}}
```

Four quarters with `FY26 Q1` unmeasured:

```
"filter"                      "break"
axis:  Q3   Q4   Q2           axis:  Q3   Q4   Q1   Q2
line:  *----*----*            line:  *----*         *
       (asserts continuity)                gap at Q1
```

A measurement stranded between two gaps is drawn as a **dot**. A
one-point path renders nothing, so emitting one would make `"break"`
hide a row the author explicitly asked to keep — worse than the
`"filter"` it was chosen over, which at least drew that row as part of
the line.

`"break"` is implemented by `line`, `area`, `point`, `bar`, `rule` and
`text`. On any other mark it is **rejected** with `PRISM_SPEC_062`
rather than quietly falling back to `"filter"`. Marks that bring their
own geometry — the polar, histogram, specialty and geographic families
— never hand a raw field value to a scale, so the null policy does not
reach them and neither mode means anything there.

Both modes emit `PRISM_WARN_NULL_DROPPED`; the message says which
behaviour applied.

**Under the default, the dropped row leaves the axis too, not only the
path.** The filter
runs before any scale resolves, so a discrete domain is built from the
surviving rows and the missing category never existed as far as the
scale is concerned. Four quarters with `FY2026 Q1` unmeasured render
three evenly spaced points labelled `FY2025 Q3`, `FY2025 Q4`,
`FY2026 Q2` — there is no blank slot, no wider gap, nothing in the
drawing from which a reader could recover that a period is missing.

That is the default, and it is a sharper one for `line` and `area`
than for the rest — which is what `"break"` above exists to answer. A bar or a point that is simply absent omits
a fact; a continuous path drawn across the hole *asserts* that the
series runs uninterrupted, which is a stronger and different claim.

**`PRISM_WARN_NULL_DROPPED` is the only signal, so a consumer has to
read it.** `prism plot` and `prism scene` print it to stderr, but a
library caller rendering `CompiledPlan.Scene` directly will not see it
unless it inspects `SceneDoc.Warnings` (surfaced as
`CompiledPlan.Diagnostics`). The warning names the channels, the
fields and the row count, which is enough to annotate the chart or
refuse to draw it — but nothing in the rendered output will do that for
you. The whole-chart case stays loud (`PRISM_ENCODE_NULL_ALL_ROWS` is
an error); it is the *partial* case that goes quiet, and it is the
harder one to notice precisely because the chart still looks right.

An aggregate group whose every input is null returns null and
surfaces `PRISM_WARN_NULL_AGG_ALL`.

## Server-side dataset registry

Wire shared aliases via a JSON config file:

```json
{
  "datasets": {
    "current": "brand_q1",
    "prior":   "brand_q4"
  }
}
```

```
prism plot --datasets-config datasets.json spec.json > chart.svg
prism serve --datasets-config datasets.json --addr :8080
```

Specs that reference `{"data": {"name": "current"}}` resolve through
the registry to an opaque ref, which a caller-supplied `DataResolver`
turns into materialized rows (Prism reads no file itself). Server-side
cache deduplicates resolution across requests.

## Browser-side dataset registry

```html
<prism-dataset name="current" src="cohorts/brand_q1.rows.json"></prism-dataset>
<prism-dataset name="prior"   src="cohorts/brand_q4.rows.json"></prism-dataset>

<prism-chart spec="overview.prism.json"></prism-chart>
<prism-chart spec="detail.prism.json"></prism-chart>
```

`<prism-dataset>` populates a page-level registry. Charts referencing
the same dataset share fetches (3 charts × 2 datasets = 2 fetches,
not 6).

## Runtime data references (`data: {ref}`)

A runtime ref is an opaque identifier resolved by a caller-supplied
`DataResolver` at compile time. The spec describes *what to draw*;
the resolver supplies *the data to draw it with*. Lets the same
spec render in multiple environments (server, browser, test)
without modification:

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"ref": "current_window"},
  "mark": "line",
  "encoding": { "x": {"field": "ts", "type": "temporal"},
                "y": {"field": "rate", "type": "quantitative"} }
}
```

Resolver wiring per environment:

**Browser.** Register a synchronous callback via
`prism.setDataResolver`:

```js
const data = await fetch("/api/window.json").then(r => r.json());
prism.setDataResolver((ref) => ref === "current_window" ? { values: data } : null);
const svg = prism.execute(specJSON);
```

The callback must be synchronous — return the dataset object
directly (no Promise). Pre-resolve any asynchronous fetches before
registering the callback.

**Go-native.** Pass `build.Options.DataResolver`:

```go
resolver := resolve.MapDataResolver{
    "current_window": {Values: rows},
}
dag, tip, _ := build.Build(s, build.Options{
    DataResolver: resolver,
    /* ... */
})
```

`resolve.DataResolver` is the interface:

```go
type DataResolver interface {
    ResolveData(ctx context.Context, ref string) (*Dataset, error)
}
```

`resolve.MapDataResolver` is a map-backed in-memory implementation
useful for tests and small fixture data; chain multiple resolvers
via `resolve.ChainDataResolvers`. An unresolved ref surfaces as
`PRISM_RESOLVE_REF_UNRESOLVED` at build time.

| Variant | Discriminator key | Use when |
|---|---|---|
| `data: {values: […]}` | `values` | Inline literal rows |
| `data: {ref: "…"}` | `ref` | Caller-resolved opaque identifier (`DataResolver`) |
| `data: {name: "…"}` | `name` | Datasets-block alias |
| `data: {feature_collection: {…}}` | `feature_collection` | Geodata basemap |

> The `data: {source: "…"}` variant (an external Pulse path) was removed
> in v0.x: Prism no longer reads `.pulse`. A spec that still carries a
> `source` key is rejected at decode with `PRISM_SPEC_039` — inline the
> rows via `values` or defer them to a `DataResolver` via `ref`.

## Partial failure

One Source failing doesn't kill the whole render. Dependents skip;
sibling paths continue; the Scene carries a
`PRISM_WARN_LAYER_SKIPPED` warning for the missing layer. Flip to
fail-fast via `ExecOpts.AbortOnError` (CI image diffs).

## Optimizer passes

Five passes run to fixpoint after build:

1. `DedupSources` — two reads of the same source collapse to one.
2. `FilterPushdown` — filters on joined output push to the side that
   owns the referenced columns.
3. `ProjectionPruning` — only request columns layered/encoded
   downstream.
4. `AggregateFusion` — sibling group-aggregates on the same input
   merge into one call.
5. `SampleInjection` — input rows > `PRISM_RENDER_MAX_MARKS` (100k
   default) → auto-sample with `PRISM_WARN_DOWNSAMPLE`.

## Worked examples

- [actual_vs_benchmark](../gallery/multi-source/actual_vs_benchmark.prism.json) — two Pulse sources, hash join, overlay.
- [multi_source_join](../gallery/multi-source/multi_source_join.prism.json) — N-way join.
- [layer_actual_vs_benchmark](../gallery/composition/layer_actual_vs_benchmark.prism.json) — two-layer composition.
