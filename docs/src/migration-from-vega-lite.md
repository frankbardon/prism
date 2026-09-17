# Migrating from Vega-Lite

Prism borrows Vega-Lite's vocabulary (mark, encoding, transform, layer,
facet) and channel model. The divergences are intentional — read this
guide to port specs in minutes.

## At a glance

| Vega-Lite | Prism | Why divergence |
|---|---|---|
| `data.url` | inline `data.values` / `datasets.*.values` (or a runtime `ref`) | Prism reads already-materialized rows; it never fetches a URL or reads a `.pulse` file. |
| `transform[].aggregate` | same shape | identical |
| `op: "mean"` | same | friendly aliases match Vega-Lite verbatim |
| `mark`, `encoding` | same vocabulary | same |
| `type: "quantitative"` | same | nominal/ordinal/quantitative/temporal |
| `scale.scheme` | same | same color schemes |
| `selection` | same shape | point + interval supported v1 |
| `params` / signals | **dropped** | no reactive runtime |
| `layer`, `concat`, `facet`, `repeat` | same | full composition v1 |
| `condition` encodings | same shape | selection + test predicate conditions supported |
| `strokeWidth` (camelCase) | `stroke_width` | snake_case throughout |
| `xOffset` / `yOffset` | `x_offset` / `y_offset` | snake_case; same grouped-bar primitive — see [Offset (dodge) channels](#offset-dodge-channels) |
| Vega expression language | structured `filter` / `calculate` built-ins | no expression language, no JS eval |

## snake_case (D019)

All field names in spec + scene IR are snake_case. Single-word
Vega-Lite vocabulary (`mark`, `encoding`, `transform`, `layer`,
`facet`) stays as-is.

| Vega-Lite | Prism |
|---|---|
| `strokeWidth` | `stroke_width` |
| `cornerRadius` | `corner_radius` |
| `fontSize` | `font_size` |
| `tickCount` | `tick_count` |
| `labelOverlap` | `label_overlap` |
| `xOffset` | `x_offset` |
| `yOffset` | `y_offset` |

## Offset (dodge) channels

Vega-Lite's `xOffset` / `yOffset` are `x_offset` / `y_offset` here —
the same grouped-bar primitive, renamed by the snake_case rule above.
The camelCase spelling is **rejected at decode** (`PRISM_SPEC_009`,
unknown field), never silently dropped, so a ported spec tells you
immediately rather than rendering an ungrouped chart.

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"metric": "Awareness",     "series": "Acme",             "score": 62},
    {"metric": "Awareness",     "series": "category average", "score": 48},
    {"metric": "Consideration", "series": "Acme",             "score": 41},
    {"metric": "Consideration", "series": "category average", "score": 44}
  ]},
  "mark": {"type": "bar"},
  "encoding": {
    "x":        {"field": "metric", "type": "nominal"},
    "y":        {"field": "score",  "type": "quantitative"},
    "x_offset": {"field": "series", "type": "nominal"},
    "color":    {"field": "series", "type": "nominal"}
  }
}
```

Two behavioural divergences to know before porting a dodged chart:

**Grouped *and* stacked bars are not supported.** An explicit `stack`
written beside a bound offset is rejected with `PRISM_SPEC_065` rather
than half-applied — the two spend the same geometry on the same
grouping. Dodging alone needs no `stack` key at all, because the
implicit bar/area stack yields to a bound offset. For a stack inside
each dodged group, draw the dodged chart and split the stacking field
out with `facet`.

**Horizontal groups are mirrored.** Prism's `y` band scale runs
bottom-to-top, and sub-bands run in the same direction the parent band
assigns its own categories, so with `y_offset` the **first** offset
category takes the **lower** sub-band of each slot. Vega-Lite's `y`
band runs top-to-bottom, so the identical spec placed side by side
looks flipped. Pin the order explicitly with
`"x_offset": {"scale": {"domain": [...]}}` (or `y_offset`) when the
comparison matters.

Sub-band ordering otherwise matches Vega-Lite: `scale.domain` first,
then a `sort` naming categories, then a `sort` direction, then the
distinct values ascending. See
[Encoding › Offset channels](concepts/encoding.md#offset-channels-x_offset--y_offset).

## Structured transforms (D005)

Prism has **no expression language**. Vega-Lite's inline expression
strings for `filter` predicates and `calculate` computed columns are
replaced by **structured built-ins** — JSON object trees. A raw string
where a predicate or expression is expected is rejected at decode
time.

| Vega-Lite | Prism |
|---|---|
| `"filter": "datum.score > 50"` | `"filter": {"op": "gt", "field": "score", "value": 50}` |
| `"filter": "datum.region === 'NA'"` | `"filter": {"op": "eq", "field": "region", "value": "NA"}` |
| `"filter": "datum.a > 0 && datum.b != null"` | `"filter": {"and": [{"op": "gt", "field": "a", "value": 0}, {"op": "not_null", "field": "b"}]}` |
| `"calculate": "datum.x * 2", "as": "y"` | `"calculate": {"op": "mul", "operands": [{"field": "x"}, {"literal": 2}]}, "as": "y"` |
| `"calculate": "datum.x == null ? 0 : datum.x", "as": "y"` | `"calculate": {"fn": "coalesce", "args": [{"field": "x"}, {"literal": 0}]}, "as": "y"` |

No `datum.` prefix, no operators, no JS function calls. See
[Spec › Filter transform](concepts/spec.md#filter-transform) and
[Spec › Calculate transform](concepts/spec.md#calculate-transform) for
the full grammar (operators, functions, `case`, and null / division
semantics).

## Aggregate aliases (D003)

Vega-Lite parity:

```
count sum mean median min max stdev variance q1 q3 ci0 ci1
```

Prism adds: `distinct mode`.

Cohort-analytics extensions (Prism-only): `wmean ratio lift share`.

## Dropped features (v1)

- `params` / signals — no reactive runtime.
- Inline Vega expressions everywhere — use the structured `filter` /
  `calculate` built-ins, or pre-compute richer logic before the data
  reaches Prism.
- Vega-Lite tooltip template strings — Prism tooltips are
  pre-formatted `TooltipLine` lists.

## Added features

- `datasets` block + per-layer `data` overrides — first-class
  multi-source.
- Hash join transform (`{join: {left, right, on, kind}, as}`) —
  in-Prism, no Pulse change.
- Cohort-analytics aggregates (`wmean`, `lift`, `share`, `ratio`).
- `sankey`, `funnel`, `sparkline` marks — first-class, not
  third-party plugins.
- Server-side + browser-side dataset registries.
- MCP tool surface for agent integration.

## Worked porting example

**Vega-Lite:**

```json
{
  "$schema": "https://vega.github.io/schema/vega-lite/v5.json",
  "data": {"url": "data/cars.json"},
  "transform": [{"filter": "datum.Horsepower > 100"}],
  "mark": {"type": "bar", "cornerRadius": 4},
  "encoding": {
    "x": {"field": "Origin", "type": "nominal"},
    "y": {"aggregate": "mean", "field": "Horsepower", "type": "quantitative"},
    "color": {"field": "Origin"}
  }
}
```

**Prism:**

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"Origin": "USA",    "Horsepower": 130},
    {"Origin": "Europe", "Horsepower": 105},
    {"Origin": "Japan",  "Horsepower": 95}
  ]},
  "transform": [{"filter": {"op": "gt", "field": "Horsepower", "value": 100}}],
  "mark": {"type": "bar", "corner_radius": 4},
  "encoding": {
    "x": {"field": "Origin", "type": "nominal"},
    "y": {"aggregate": "mean", "field": "Horsepower", "type": "quantitative"},
    "color": {"field": "Origin", "type": "nominal"}
  }
}
```

Diffs:
- `$schema`: URN form.
- `data.url` → inline `data.values` (the caller materializes the rows; Prism reads no URL or `.pulse` file).
- `filter`: expression string → structured `{op, field, value}` predicate.
- `cornerRadius` → `corner_radius`.
- `color` channel: explicit `type` (Vega-Lite infers; Prism is strict).

## Editor setup

`prism init` writes `.prism/editor/` with configs for VSCode,
JetBrains, Neovim, Vim — autocomplete + inline validation on
`*.prism.json` files from the embedded JSON Schema bundle.
