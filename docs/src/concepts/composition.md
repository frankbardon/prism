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

## Per-cell theme overrides

`facet` and `repeat` both accept an optional `cell_overrides` array —
a sparse theme override scoped to one cell of the resulting grid,
addressed by its **0-based `(row, column)` grid position**, not by
the data value that landed in that cell. Each entry's `theme` block
is the same sparse override shape used for a whole-chart `theme`
override (`spec.ThemeOverride` — see [Themes](themes.md)); it merges
over the chart's resolved theme for that one cell only.

```json
{
  "facet": {
    "column": {"field": "region"},
    "cell_overrides": [
      {"row": 0, "column": 1, "theme": {"mark": {"fill": "#e15759"}}}
    ]
  },
  "spec": {
    "$schema": "urn:prism:schema:v1:spec",
    "mark": "bar",
    "encoding": {...}
  }
}
```

Because addressing is positional, re-sorting or filtering the
faceted/repeated field shifts which value occupies a given cell —
the override always applies to whichever value currently lands in
that grid slot, not to a named value. For `repeat`, `row`/`column`
index into the `repeat.row`/`repeat.column` field lists (an axis
left empty collapses to a single implicit slot at index `0`,
mirroring the encoder's single-row/single-column scaffold); for
`facet`, an axis with no `row`/`column` channel likewise collapses
to a single implicit slot at index `0`.

This is a spec-level, model-only mechanism today: `spec.Facet` and
`spec.Repeat` carry `CellOverrides []spec.CellThemeOverride`, but the
composite encoders (`encode/encode_facet.go`,
`encode/encode_repeat.go`) do not yet apply them when rendering — a
declared `cell_overrides` block is accepted and validated but has no
visual effect until that encoder wiring lands.

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
