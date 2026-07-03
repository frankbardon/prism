# Multi-source

Composing N Pulse queries into one chart is a first-class workflow.

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

The encoder collects null rows it drops and emits
`PRISM_WARN_NULL_DROPPED` carrying the count + offending channels.
An aggregate group whose every input is null returns null and
surfaces `PRISM_WARN_NULL_AGG_ALL`.

## Server-side dataset registry

Wire shared aliases via a JSON config file:

```json
{
  "datasets": {
    "current": "cohorts/brand_q1.pulse",
    "prior":   "cohorts/brand_q4.pulse"
  }
}
```

```
prism plot --datasets-config datasets.json spec.json > chart.svg
prism serve --datasets-config datasets.json --addr :8080
```

Specs that reference `{"data": {"name": "current"}}` resolve through
the registry. Server-side cache deduplicates fetches across requests.

## Browser-side dataset registry

```html
<prism-dataset name="current" src="cohorts/brand_q1.pulse"></prism-dataset>
<prism-dataset name="prior"   src="cohorts/brand_q4.pulse"></prism-dataset>

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
