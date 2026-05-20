# Composition

Prism supports five composition primitives, all v1:

| Op | What | Multi-source? |
|---|---|---|
| `layer` | Stack marks on shared axes | per-layer `data` allowed |
| `concat` / `hconcat` / `vconcat` | Side-by-side panels | per-panel `data` allowed |
| `facet` | Grid by data values (one cell per partition) | usually single source |
| `repeat` | Grid by field list (one cell per field) | usually single source |

## Layer

```json
{
  "layer": [
    {"$schema": "urn:prism:schema:v1:spec", "mark": "bar", "encoding": {...}},
    {"$schema": "urn:prism:schema:v1:spec", "mark": "rule", "encoding": {...}}
  ]
}
```

Layer order = render order = z-index (last is on top).

## Concat / hconcat / vconcat

```json
{
  "vconcat": [
    {"$schema": "...", "mark": "line", "encoding": {...}},
    {"$schema": "...", "mark": "histogram", "encoding": {...}}
  ]
}
```

`hconcat` lays out left-to-right. `vconcat` top-to-bottom. `concat`
is a flat array; today it behaves like `hconcat` (the `columns` wrap
parameter is post-v1).

## Facet

```json
{
  "facet": {"column": {"field": "region"}},
  "spec": {
    "$schema": "urn:prism:schema:v1:spec",
    "mark": "bar",
    "encoding": {...}
  }
}
```

Partitions data by `region`, renders one cell per partition. Inner
`spec` is fully recursive — facet within facet within facet works.

## Repeat

```json
{
  "repeat": {"row": ["score", "share", "lift", "growth"]},
  "spec": {
    "$schema": "urn:prism:schema:v1:spec",
    "mark": "line",
    "encoding": {
      "x": {"field": "week"},
      "y": {"field": {"repeat": "row"}}
    }
  }
}
```

Each cell substitutes `{repeat: "row"}` with the field name for that
cell. Pure substitution — no template expressions.

## Scale resolution

`resolve.scale.{x,y,color,size}` controls cross-cell scale sharing:

| Value | Behavior |
|---|---|
| `shared` (default for x/y) | Union of domains across cells/layers, single axis. |
| `independent` (default for color) | Per-cell domains, per-cell axes. |

Mixing incompatible types on a shared scale (quantitative + nominal)
raises `PRISM_PLAN_005`.

## Worked examples

- [layer_actual_vs_benchmark](../gallery/composition/layer_actual_vs_benchmark.prism.json) — bar + rule overlay.
- [vconcat_metrics](../gallery/composition/vconcat_metrics.prism.json) — 3-row stack.
- [facet_by_region](../gallery/composition/facet_by_region.prism.json) — 3×3 grid.
- [facet_nested](../gallery/composition/facet_nested.prism.json) — recursion proof.
- [repeat_metrics](../gallery/composition/repeat_metrics.prism.json) — 1×4 over 4 metrics.
- [dashboard](../gallery/composition/dashboard.prism.json) — 4-cell vconcat showcasing mixed marks.
