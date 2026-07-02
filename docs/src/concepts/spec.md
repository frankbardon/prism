# Spec

A Prism Spec is a JSON document describing one chart. It is the
contract between authors (humans / agents) and the Prism pipeline.

## Six-stage pipeline

```
Spec (JSON) → Parse → Validate → Plan → Compile → Encode → Render → Bytes
                                          │
                                          ├─→ Pulse engine (data ops)
                                          └─→ Renderer backend (SVG / Canvas)
```

## Minimum viable spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"source": "cohort.pulse"},
  "mark": "bar",
  "encoding": {
    "x": {"field": "brand_id", "type": "nominal"},
    "y": {"field": "score",    "type": "quantitative", "aggregate": "mean"}
  }
}
```

Five top-level keys are typically present:

| Key | Purpose |
|---|---|
| `$schema` | URN identifier (`urn:prism:schema:v1:spec`) for editor autocomplete + version pinning. |
| `data` | Where to read rows from — a `.pulse` source, an inline `values` array, a named alias, etc. |
| `transform` | Optional array of row-level operations (filter, calculate, aggregate, sort, ...). |
| `mark` | What to draw — `bar`, `line`, `point`, `pie`, `sankey`, ... |
| `encoding` | How to bind data fields to visual channels (x/y/color/size/...). |

## Full top-level field list

```
$schema       data            datasets        transform
mark          encoding        layer           concat
hconcat       vconcat         facet           repeat
spec          selection       resolve         theme
width         height          padding         background
title         subtitle        description     projection
animation
```

Exactly one of `mark | layer | concat | hconcat | vconcat | facet | repeat`
must be present. The validator enforces this with `PRISM_SPEC_*` codes.

## Animation

The optional `animation` block requests a client-side tween whenever the
spec swaps. Static SVG output is unaffected — the renderer
ignores the block entirely. Only the browser web component
(`<prism-chart>`) and the WASM runtime honour it.

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data":    {"name": "sales", "values": [...]},
  "mark":    "bar",
  "encoding": {
    "x": {"field": "region", "type": "nominal", "key": true},
    "y": {"aggregate": "mean", "field": "score", "type": "quantitative"}
  },
  "animation": {"duration_ms": 600, "easing": "cubic_in_out"}
}
```

Fields:

| Field | Default | Notes |
|---|---|---|
| `duration_ms` | `400` | Total tween length, capped at 5000. |
| `easing` | `cubic_in_out` | One of `linear`, `cubic_*`, `quad_*`, `sine_*`, `expo_*` (× `in`/`out`/`in_out`). |
| `stagger_ms` | `0` | Per-mark delay applied in document order. |
| `enter` | `fade` | `fade` or `none`. Marks that appear at scene-swap time. |
| `exit`  | `fade` | `fade` or `none`. Marks that disappear at scene-swap time. |

For the tween to match marks across scene swaps (object constancy),
declare a join key on one encoding channel via `"key": true`. Without a
key, validation fires `PRISM_SPEC_023`.

Animation respects the user's
[`prefers-reduced-motion`](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-reduced-motion)
setting: the animator snaps directly to the final state when the
preference is `reduce`.

When two scenes are structurally incompatible (different layer count,
different mark families, etc.) the animator falls back to an instant
replace and emits `PRISM_WARN_ANIM_FALLBACK` on the
`prism:warn` CustomEvent stream.

Spec rules that govern `animation`:

- `PRISM_SPEC_022` — unknown easing name.
- `PRISM_SPEC_023` — block declared but no channel has `key: true`.
- `PRISM_SPEC_024` — more than one channel carries `key: true`.

## Filter transform

`filter` keeps the rows for which a **structured predicate** evaluates
true. The predicate is a JSON object tree, never an expression string —
a raw string value is rejected at decode time. Each predicate node is
exactly one of a leaf test or a boolean combinator.

**Leaf comparisons** — `eq`, `ne`, `lt`, `lte`, `gt`, `gte` — compare a
field against a literal (`value`) or against another column
(`to_field`, a field-vs-field compare):

```json
"transform": [
  {"filter": {"op": "gt", "field": "Horsepower", "value": 100}},
  {"filter": {"op": "eq", "field": "Origin", "value": "USA"}},
  {"filter": {"op": "lt", "field": "sale_price", "to_field": "list_price"}}
]
```

**Set membership** — `one_of` / `not_one_of` — tests a field against a
non-empty candidate set:

```json
{"filter": {"op": "one_of", "field": "Origin", "values": ["USA", "Europe"]}}
```

**Inclusive range** — `between` — keeps rows where `lo <= field <= hi`:

```json
{"filter": {"op": "between", "field": "year", "lo": 2010, "hi": 2019}}
```

**Null checks** — `is_null` / `not_null` — take only a `field`:

```json
{"filter": {"op": "not_null", "field": "quota_mean"}}
```

**Boolean combinators** — `and` / `or` / `not` — nest predicates to any
depth. A combinator node carries only its branch, never leaf operands:

```json
{"filter": {"and": [
  {"op": "gt", "field": "Horsepower", "value": 100},
  {"or": [
    {"op": "eq", "field": "Origin", "value": "USA"},
    {"not": {"op": "is_null", "field": "Cylinders"}}
  ]}
]}}
```

Operator reference:

| Operator | Operands | Meaning |
|---|---|---|
| `eq` `ne` `lt` `lte` `gt` `gte` | `field` + exactly one of `value` / `to_field` | Equality / ordered comparison against a literal or another column. |
| `one_of` `not_one_of` | `field` + `values` (non-empty) | Set membership. |
| `between` | `field` + `lo` + `hi` | Inclusive range (`lo <= x <= hi`). |
| `is_null` `not_null` | `field` only | Null-state test. |
| `and` `or` | non-empty list of predicates | Boolean conjunction / disjunction. |
| `not` | one predicate | Boolean negation. |

The grammar is intentionally minimal — no substring, regex, or date
arithmetic. Anything richer is precomputed by the caller before the
data reaches Prism.

## Calculate transform

`calculate` appends one derived column, named by `as`, from a
**structured expression tree** (again, never an expression string). A
node is exactly one of:

- a field reference — `{"field": "Horsepower"}`
- a literal — `{"literal": 5}` (number, string, or bool; a null literal is rejected)
- an arithmetic op — `{"op": "add"|"sub"|"mul"|"div"|"mod", "operands": [...]}`
- a pure function — `{"fn": "abs"|"round"|"floor"|"ceil"|"neg"|"coalesce"|"min"|"max", "args": [...]}`
- a string concat — `{"concat": [...]}`
- a conditional — `{"case": [{"when": <predicate>, "then": <expr>}], "else": <expr>}`

`add` and `mul` take **two or more** operands; `sub`, `div`, `mod` take
**exactly two**. `abs`/`round`/`floor`/`ceil`/`neg` take **one**
argument; `coalesce`/`min`/`max` take **two or more**. `case` requires
at least one `when → then` branch and a mandatory `else` fallback (`if`
is accepted as a decode-time alias for `case`).

Arithmetic — `Horsepower / Weight`:

```json
{"calculate": {"op": "div", "operands": [{"field": "Horsepower"}, {"field": "Weight"}]}, "as": "power_ratio"}
```

Default a null with `coalesce`:

```json
{"calculate": {"fn": "coalesce", "args": [{"field": "quota"}, {"literal": 0}]}, "as": "quota_padded"}
```

Build a label with `concat`:

```json
{"calculate": {"concat": [{"field": "Origin"}, {"literal": " — "}, {"field": "Name"}]}, "as": "label"}
```

Bucket with `case`; each `when` arm reuses the **filter predicate
grammar** verbatim:

```json
{"calculate": {
  "case": [
    {"when": {"op": "gte", "field": "score", "value": 0.9}, "then": {"literal": "A"}},
    {"when": {"op": "gte", "field": "score", "value": 0.8}, "then": {"literal": "B"}}
  ],
  "else": {"literal": "C"}
}, "as": "grade"}
```

The output column type is inferred: a numeric expression yields a float
column, a string expression a categorical column.

The grammar is intentionally minimal — no `log`/`sqrt`/`pow`/trig, no
substring, no date arithmetic. Precompute anything richer upstream.

### Null and division semantics

Both `filter` and `calculate` use **two-valued** logic — there is no
SQL-style three-valued "unknown".

- **Filter leaves.** A null operand makes a leaf comparison,
  `one_of` / `not_one_of`, or `between` evaluate **false** (the row is
  excluded unless an enclosing `or` / `not` rescues it). Test for null
  explicitly with `is_null` / `not_null`; `and` / `or` / `not` then
  operate on plain booleans.
- **Calculate null propagation.** Arithmetic
  (`add`/`sub`/`mul`/`div`/`mod`) and the single-argument numeric
  functions (`abs`/`round`/`floor`/`ceil`/`neg`) propagate nulls: any
  null operand yields a null result. `min` / `max` skip null arguments
  and return null only when every argument is null. `coalesce` returns
  its first non-null argument. `concat` treats a null operand as the
  empty string and always yields a (possibly empty) string. `case`
  returns the `then` of the first branch whose `when` holds, else the
  `else`.
- **Division by zero.** A runtime zero divisor (`div` / `mod`) yields
  **null silently** — no error, no warning. A *literal*-zero divisor
  (e.g. `{"op": "div", "operands": [{"field": "x"}, {"literal": 0}]}`)
  is a spec mistake and is **rejected at validate time** as
  `PRISM_SPEC_038`.

Validation codes:

- `PRISM_SPEC_037` — filter predicate not well-formed (unknown field,
  type-mismatched comparison, `between` with `lo > hi`, empty `values`
  set).
- `PRISM_SPEC_038` — calculate expression not well-formed (unknown
  operand field, literal-zero divisor, `as` missing or shadowing a
  source column).

## Crosstab transform

The `crosstab` transform builds a contingency table in Prism's in-memory
engine: it composes the cell aggregation across the row × column grouper
grid, recomputes the margin axes, applies the configured normalisation,
and returns long-form rows ready for a heatmap encoder.

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"source": "sales.pulse"},
  "transform": [{
    "crosstab": {
      "rows":    [{"field": "region"}],
      "columns": [{"field": "quarter"}],
      "cell":    {"aggregate": "mean", "field": "revenue", "as": "mean_revenue"},
      "margins": {"rows": true, "columns": true},
      "normalize": "none"
    }
  }],
  "mark": "heatmap",
  "encoding": {
    "x":     {"field": "quarter", "type": "nominal"},
    "y":     {"field": "region",  "type": "nominal"},
    "color": {"field": "mean_revenue", "type": "quantitative"}
  }
}
```

Body:

| Field | Required | Notes |
|---|---|---|
| `rows`      | yes | Row-axis groupers. One or more `{field: "..."}` (category, default) or `{field: "...", type: "date", period: "..."}` (date bucketing). |
| `columns`   | yes | Column-axis groupers. Same shape. |
| `cell`      | yes | `{aggregate, field, as}` — aggregate alias (sum, mean, count, ...). |
| `margins`   |     | `{rows, columns, grand}` — emit total rows with `_margin` sentinel. |
| `normalize` |     | `none` (default), `row`, `column`, `total`. |
| `shape`     |     | `long` (default) returns one row per cell; `matrix` is reserved. |
| `overlays`  |     | Post-result overlay layers; each adds one F64 column aligned to the base cell. See below. |

### Crosstab overlays

`overlays` attaches post-result overlay layers to the cell grid.
Each overlay adds one F64 column — index-aligned to the base cell — so
it can drive a `color` or `opacity` channel. v1 supports the
cell-scoped kinds that align one-to-one with heatmap cells:

| `kind` | Column value | Notes |
|---|---|---|
| `share_of_row` | cell / row-margin | cells along a row sum to 1.0 |
| `share_of_col` | cell / column-margin | cells down a column sum to 1.0 |
| `index_vs_margin` | cell / margin × 100 | requires `axis` (`row` or `column`); 100 = on-margin |
| `zscore_vs_margin` | (cell − margin) / sd | requires `axis`; a significance proxy (\|z\| > 1.96 ≈ p < .05) — bind to `opacity` for significance shading |

```json
"crosstab": {
  "rows":    [{"field": "region"}],
  "columns": [{"field": "quarter"}],
  "cell":    {"aggregate": "sum", "field": "revenue", "as": "revenue"},
  "overlays": [{"kind": "share_of_row", "as": "row_share"}]
}
```

When any overlay is present the node emits body cells only (overlays
decorate body cells), so user `margins` flags are ignored for the visual
output. Group/series-scoped kinds (`index_vs_total`, `share_of_total`)
land in a follow-up.

Constraints:

- Crosstab must be the **first** transform on the chain. v1 crosstab
  consumes the source table directly, so chaining it after another Prism
  transform is not supported (`PRISM_SPEC_033`).
- Grouper `type` is `category` (default) or `date`. A date grouper
  buckets a temporal field by `period` — one of `year`, `quarter`,
  `month` (default), `week`, `day`, `day_of_week` — emitting string
  bucket-key labels (`"2024"`, `"2024-Q1"`, `"2024-03"`, ...). Range /
  rounded / quantile groupers land in a follow-up.
- Margin rows carry a `_margin` column the encoder leaves on the table
  — filter them out at the chart level by upstream `filter`-after
  composition or by avoiding the `margins` flag for the visual
  rendering use case.

Cells are evenly numbered through `PRISM_SPEC_032` (shape rule),
`PRISM_SPEC_033` (position rule), `PRISM_SPEC_034` (normalize enum).
Run `prism errors lookup <code>` for details + fixups.

## Regression transform

The `regression` transform fits an OLS regression over the cohort
(Pulse `REG_OLS`) and emits the two endpoints of the fitted trend line —
`(min(x), ŷ)` and `(max(x), ŷ)`. Because every OLS fitted point is
collinear, two endpoints draw the full line; layer a `line` mark over
`(predictor, fitted)` on top of a `point` scatter of `(predictor,
target)` for the classic regression overlay.

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"source": "sales.pulse"},
  "layer": [
    {"mark": "point", "encoding": {
      "x": {"field": "spend", "type": "quantitative"},
      "y": {"field": "revenue", "type": "quantitative"}
    }},
    {
      "transform": [{"regression": {"target": "revenue", "predictors": ["spend"], "as": "fit"}}],
      "mark": "line",
      "encoding": {
        "x": {"field": "spend", "type": "quantitative"},
        "y": {"field": "fit", "type": "quantitative"}
      }
    }
  ]
}
```

Body:

| Field | Required | Notes |
|---|---|---|
| `target`     | yes | Dependent variable (y). |
| `predictors` | yes | Independent variable (x). Exactly one in v1 — the only shape that maps to a 2-D line. |
| `as`         |     | Fitted-value output column name (default `fitted`). |

Constraints:

- Regression must be the **first** transform on the chain — the OLS
  prepass fits the source cohort directly, like crosstab
  (`PRISM_SPEC_035`, `PRISM_PLAN_REGRESSION_REQUIRES_SOURCE`).
- v1 fits unpenalized OLS with a single predictor. Multiple predictors,
  GLM/Bayesian families, and the per-row residual / leverage attributes
  land in a follow-up.

## TimeUnit transform

The `timeunit` transform truncates a temporal field to a calendar
period and appends the truncated date as a new column — the Vega-Lite
`timeUnit` analogue. The output is a date (the period start), so the
derived column stays temporal for axis / scale resolution and sorts
chronologically. It runs client-side (pure epoch arithmetic), so —
unlike `crosstab` / `regression` — it composes anywhere in a chain.

```json
{
  "transform": [{"timeunit": "month", "field": "order_date", "as": "order_month"}],
  "mark": "line",
  "encoding": {
    "x": {"field": "order_month", "type": "temporal"},
    "y": {"aggregate": "sum", "field": "revenue", "type": "quantitative"}
  }
}
```

| Field | Required | Notes |
|---|---|---|
| `timeunit` | yes | Period: `year`, `quarter`, `month`, `week` (ISO / Monday start), `day`. Truncates to the period start. |
| `field`    | yes | Temporal field to truncate. |
| `as`       | yes | Output date column name. |

`day_of_week` and other component-extraction units (which return an
ordinal, not a date) land in a follow-up.

## Strict by default

- Unknown fields error (typos like `xfield` vs `x.field` caught at parse).
- Semantic violations error (agg op on incompatible field type, etc.).
- 24+ `PRISM_SPEC_*` rules cover field-existence, channel-for-mark,
  selection refs, structured filter / calculate predicates, scale type compatibility,
  animation easing / key constraints, and more. Run
  `prism errors lookup <code>` for details on any.

## Validate a spec

```
prism validate my-chart.prism.json
prism validate --json my-chart.prism.json
```

## Spec patches (RFC 6902)

Iterative edits to a rendered chart don't need a full spec re-send.
A caller can transmit an [RFC 6902 JSON Patch][rfc6902] and the
library applies it atomically, re-decodes, and re-compiles:

```json
[
  { "op": "replace", "path": "/mark", "value": "area" },
  { "op": "add",     "path": "/encoding/color",
                     "value": { "field": "category", "type": "nominal" } },
  { "op": "test",    "path": "/data/name", "value": "current_window" },
  { "op": "remove",  "path": "/title" }
]
```

Same protocol in Go and in WASM:

```go
next, err := prism.ApplyPatch(s, patch)
// or, statefully:
scn, _ := prism.NewScene(ctx, s, prism.CompileOptions{})
err := scn.Apply(patch)
```

```js
const newSpecJSON = prism.applyPatch(specJSON, JSON.stringify(patch));
const patchJSON   = prism.diffSpecs(beforeJSON, afterJSON);
```

**Atomic application.** Either every operation in the patch
succeeds and the new spec replaces the old, or no state changes.
A failing op surfaces as `PRISM_SPEC_PATCH_001` with the offending
op index in the envelope's `Details.OpIndex`.

**Test operations.** Include a `test` op to fail-fast on
optimistic-concurrency violations — the patch aborts if the
current spec value at `path` differs from the expected value.

**Diff helper.** `prism.DiffSpecs(before, after)` (Go) and
`prism.diffSpecs(beforeJSON, afterJSON)` (WASM) produce a patch
that transforms one spec into the other. Useful for callers that
think in full specs and only want to transmit the delta.

[rfc6902]: https://www.rfc-editor.org/rfc/rfc6902

## Further reading

- [Marks](marks.md), [Encoding](encoding.md), [Composition](composition.md).
- [Spec field reference](../reference/spec.md) — every field with type + description.
- [Gallery](../gallery/index.md) — 59 worked examples.
