# Migrating from Vega-Lite

Prism borrows Vega-Lite's vocabulary (mark, encoding, transform, layer,
facet) and channel model. The divergences are intentional — read this
guide to port specs in minutes.

## At a glance

| Vega-Lite | Prism | Why divergence |
|---|---|---|
| `data.url` | `data.source` | Pulse refs aren't URLs (could be cohort ID, GCS path, archive#shard). |
| `transform[].aggregate` | same shape | identical |
| `op: "mean"` | same | friendly aliases match Vega-Lite verbatim |
| `mark`, `encoding` | same vocabulary | same |
| `type: "quantitative"` | same | nominal/ordinal/quantitative/temporal |
| `scale.scheme` | same | same color schemes |
| `selection` | same shape | point + interval supported v1 |
| `params` / signals | **dropped** | no reactive runtime |
| `layer`, `concat`, `facet`, `repeat` | same | full composition v1 |
| `condition` encodings | **dropped v1** | post-v1 feature |
| `strokeWidth` (camelCase) | `stroke_width` | snake_case throughout |
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

Pulse adds: `distinct mode`.

Cohort-analytics extensions (Prism-only): `wmean ratio lift share`.

## Dropped features (v1)

- `params` / signals — no reactive runtime.
- `condition` encodings — post-v1.
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
  "data": {"source": "cars.pulse"},
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
- `data.url` → `data.source`; `.json` → `.pulse`.
- `filter`: expression string → structured `{op, field, value}` predicate.
- `cornerRadius` → `corner_radius`.
- `color` channel: explicit `type` (Vega-Lite infers; Prism is strict).

## Editor setup

`prism init` writes `.prism/editor/` with configs for VSCode,
JetBrains, Neovim, Vim — autocomplete + inline validation on
`*.prism.json` files from the embedded JSON Schema bundle.
