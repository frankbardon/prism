# Composition

Prism supports five composition primitives, all v1:

| Op | What | Multi-source? |
|---|---|---|
| `layer` | Stack marks on shared axes | per-layer `data` allowed |
| `concat` / `hconcat` / `vconcat` | Side-by-side panels | per-panel `data` allowed |
| `facet` | Grid by data values (one cell per partition) | usually single source |
| `repeat` | Grid by field list (one cell per field) | usually single source |

## What a parent passes down

A composition parent inherits exactly three things to its children:

| Key | Inherited? | Rule |
|---|---|---|
| `datasets` | yes | Merged; an entry the child redeclares wins. |
| `data` | yes | Only when the child declares no `data` of its own. |
| `$schema` | yes | Only when the child omits it. |
| `encoding` | **no** | — |
| `mark`, `transform`, `title`, `theme`, … | **no** | — |

Every layer and every panel is a self-contained chart. That is a
divergence from Vega-Lite, which inherits a parent `encoding` into
layer children.

Because nothing reads it, an `encoding` block written beside a
composition operator is **rejected**, not ignored:

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "encoding": {"x": {"field": "month", "type": "ordinal"}},
  "layer": [ ... ]
}
```

```
PRISM_SPEC_054: An "encoding" block on the spec root sits beside
"layer", where nothing reads it.
```

The rule (`validate/rules/composite_parent_encoding.go`) covers every
operator — `layer`, `concat`, `hconcat`, `vconcat`, `facet`, `repeat` —
and walks the whole tree, so a `layer` nested inside a `concat` panel is
caught at `concat[1].layer`. Two shapes stay legal, because their
encoding really is read:

- the `spec` child of a `facet` or `repeat` parent — that child *is* the
  chart being drawn, so its own `encoding` is where the chart lives;
- a flat spec with `mark` + `encoding` and no composition operator,
  including the `encoding.row` / `encoding.column` facet shorthand.

To share channel config across layers, repeat the block on each child
and leave `resolve` at its default (`shared`) — the encoder folds the
children's `axis` blocks and reports a disagreement rather than dropping
one. See [the shared-axis rule](#the-shared-axis-rule-first-specified-wins-per-property)
below.

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
      {"row": 0, "column": 1, "theme": {"marks": {"bar": {"fill": "#e15759"}}}}
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

`encode/encode_facet.go` and `encode/encode_repeat.go` apply each
cell's matching `CellThemeOverride.Theme` on top of the chart's
resolved base theme via `theme.ApplyOverride` — the same merge
machinery a whole-chart `theme` override uses — when materializing
that cell's child scene; cells with no matching entry render with
the base theme unchanged. Note the override targets the same
per-mark-type slot (`marks.<type>`) a built-in theme uses for that
mark: a built-in theme (e.g. `light`) typically sets an explicit
`marks.bar.fill`, which wins over the generic top-level `mark.fill`
fallback, so a per-cell fill override on a bar chart should target
`marks.bar.fill` (as above) rather than `mark.fill`. This is
orthogonal to `resolve.scale` below — a per-cell theme override never
changes whether scales/axes are shared or independent across cells.

## Scale resolution

`resolve.scale.{x,y,color,size}` controls cross-cell scale sharing:

| Value | Behavior |
|---|---|
| `shared` (default for x/y) | Union of domains across cells/layers, single axis. |
| `independent` (default for color) | Per-cell domains, per-cell axes. |

Mixing incompatible types on a shared scale (quantitative + nominal)
raises `PRISM_PLAN_005`.

## Axis config under shared vs independent scales

A channel's `axis` block (`grid`, `title`, `label_angle`,
`label_overlap`, `format`, …) is honoured under **both** resolve modes —
which one is in force only changes *whose* block is read.

| Resolve mode | Where the axis config comes from |
|---|---|
| `independent` | Each child renders its own axis from its own `axis` block. Nothing is merged. |
| `shared` (the default for `x`/`y`) | One axis is drawn for all children, so the children's `axis` blocks are folded into one. |

### The shared-axis rule: first specified wins, per property

A shared axis is drawn once but may be described by N children that
disagree. Prism resolves this **property by property, in child
declaration order**:

- A property is taken from the **first** child that specifies it. A
  child that omits the property does not participate — so if only
  layer 1 sets `grid`, layer 1's value is used even though layer 0 came
  first.
- A later child specifying the **same** property with the **same** value
  is a no-op.
- A later child specifying the same property with a **different** value
  is ignored, and the encoder emits
  `PRISM_WARN_AXIS_CONFIG_CONFLICT` naming the channel, the property,
  the winning child, and the ignored value. A conflict is never
  silently resolved.

This matches Vega-Lite, which also resolves a shared axis from the
first child that specifies it. Prism's refinement is that the unit of
resolution is the individual property rather than the whole `axis`
block, so a child that sets only `grid` does not wipe out another
child's `title`.

```json
{
  "layer": [
    {"encoding": {"x": {"field": "day", "type": "nominal", "axis": {"grid": false}}}},
    {"encoding": {"x": {"field": "day", "type": "nominal", "axis": {"grid": true, "title": "Day"}}}}
  ]
}
```

The shared x axis draws **no grid** (layer 0 specified `grid` first)
with the title **"Day"** (only layer 1 specified `title`), and a
`PRISM_WARN_AXIS_CONFIG_CONFLICT` reports that layer 1's `grid: true`
was ignored.

To silence a conflict, either set the property identically on every
child that specifies it, set it on only one child, or opt the channel
out of sharing with `"resolve": {"scale": {"x": "independent"}}` —
noting that independent scales also stop the children's positions from
aligning.

Facets take the same path. A facet has a single child `spec`, so its
`axis` block simply flows onto the shared axis and no conflict is
possible.

### Axis titles

When no child sets `axis.title`, a shared axis falls back to the field
name bound on the first child that declares the channel. An explicit
`"title": false` suppresses the title and is **not** overwritten by that
fallback — including on a histogram's synthetic bin-count axis, whose
`"count"` label is only a fallback.

## Worked examples

- [layer_actual_vs_benchmark](../gallery/composition/layer_actual_vs_benchmark.prism.json) — bar + rule overlay.
- [vconcat_metrics](../gallery/composition/vconcat_metrics.prism.json) — 3-row stack.
- [facet_by_region](../gallery/composition/facet_by_region.prism.json) — 3×3 grid.
- [facet_nested](../gallery/composition/facet_nested.prism.json) — recursion proof.
- [facet_cell_theme_override](../gallery/composition/facet_cell_theme_override.prism.json) — 1×3 region facet with two cells recolored via `cell_overrides`.
- [repeat_metrics](../gallery/composition/repeat_metrics.prism.json) — 1×4 over 4 metrics.
- [dashboard](../gallery/composition/dashboard.prism.json) — 4-cell vconcat showcasing mixed marks.
