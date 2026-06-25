# Marks

A mark is the visual primitive that data rows become — bars, lines,
arcs, etc. Specify via top-level `mark` (shorthand string) or
`mark: {type: "...", ...properties}`.

## Catalog

### Basic marks (Vega-Lite parity)

| Mark | When to use |
|---|---|
| `bar` | Compare categories. The default. |
| `line` | Continuous trends; ordered x-axis. |
| `area` | Filled trends. Supports negative values + stacks. |
| `point` | Scatter, dot plots. |
| `circle`, `square` | Convenience aliases for `point` with shape preset. |
| `tick` | Strip plots, ranking dot plots. |
| `rect` | Heatmap cells, custom rectangular layouts. |
| `rule` | Reference lines, benchmarks, ranges. |
| `text` | Inline labels, annotations. |
| `arc` | Primitive for `pie` / `donut` / sankey links. |

### Composite marks

| Mark | Internally expands to |
|---|---|
| `histogram` | `bar` + auto-bin transform. |
| `heatmap` | `rect` + 2D bin + sequential color scale. Binds an optional field-driven `opacity` channel for per-cell shading — pair it with a crosstab `zscore_vs_margin` overlay column to fade insignificant cells (significance shading). Opacity maps the field linearly over `[min, max]` to `[0.15, 1.0]`. |
| `boxplot` | `rect` (IQR) + `rule` (whiskers) + `point` (outliers). |
| `violin` | `area` symmetric around centerline (Epanechnikov KDE). |
| `pie` | `arc` with theta computed from share. |
| `donut` | `arc` with `inner_radius_ratio > 0`. |

### Specialty marks

| Mark | When to use |
|---|---|
| `sankey` | Flow diagrams (source/target/value table). |
| `funnel` | Conversion funnels — stacked trapezoids. |
| `sparkline` | Inline micro-line charts, no axes. |
| `sparkbar` | Inline micro-column charts, no axes — bar-family sibling of `sparkline`. |
| `winloss` | Equal-height up/down micro-bars by the sign of `y` (>0 up, <0 down, ==0 flat). Magnitude is ignored — only direction encodes. |
| `sparkarea` | Inline filled micro-area charts, no axes — area-family sibling of `sparkline`; fill reaches the y=0 baseline. |
| `bullet` | Compact KPI gauge — a measure bar over qualitative bands, with an optional comparative bar and target tick. Keeps its measure axis. |
| `image` | Sprites / data-URL images at position. |
| `path` | Raw SVG path data — escape hatch. |
| `geoshape` | Country / admin-1 polygons (choropleth). See [Geographic Marks](geo.md). |
| `geopoint` | Lon/lat → point overlay. See [Geographic Marks](geo.md). |

### Tree / dendrogram / network

Hierarchical and relational marks share a small layout package
(`encode/marks/layout`) and decompose to existing primitives (path,
point, rect, text) so the SVG and PDF renderers handle them without
new geometry types.

| Mark | When to use |
|---|---|
| `tree` | Rooted hierarchy (org charts, decision trees). Reingold-Tilford tidy layout. |
| `dendrogram` | Clustering tree — tree variant with `link_shape: step` + `node_shape: none` defaults. |
| `network` | Undirected / directed node-link diagram. Force-directed layout (deterministic seed). |

Channel bindings:

- `source` — parent / from-node id field (required for tree/dendrogram/network).
- `target` — child / to-node id field (required).
- `value` — optional edge weight (network) / node size (tree).
- `text` — optional per-node label.
- `color`, `fill`, `stroke`, `opacity`, `size` — standard mark props.

Mark-def options:

- `orient` — `vertical` (default), `horizontal`, `radial`.
- `link_shape` — `step` (default), `curve`, `straight`.
- `node_shape` — `circle` (default), `rect`, `none`.
- `node_size` — base radius / side length (default 6).
- `layout` (network) — `force` (default), `random`.
- `iterations`, `link_distance`, `charge`, `seed` (network).

Validate rules: `PRISM_SPEC_028` (missing source/target),
`PRISM_SPEC_029` (multi-root tree). Encode-time:
`PRISM_ENCODE_TREE_CYCLE`, `PRISM_ENCODE_NETWORK_NONFINITE`,
`PRISM_WARN_NETWORK_CYCLE`.

### Bullet

The `bullet` mark is a compact KPI gauge (after Stephen Few's bullet
graph). It draws, back-to-front:

1. qualitative **band** rects — graded background ranges (dark → light),
2. the **measure** bar — the encoded data value (thick),
3. an optional **comparative** bar — a secondary value, thinner overlay,
4. an optional **target** tick — the value to beat.

Unlike the spark family, `bullet` keeps its measure axis, and the
measure-axis domain is widened to span the bands / target / comparative
so none of them clip past the data range.

Channel bindings:

- Horizontal (default): `x` is the quantitative measure, `y` is the
  nominal metric label.
- Vertical (`orientation: "vertical"`): `y` is the quantitative measure,
  `x` is the nominal metric label.

The headline measure reads from row 0 of the measure field (a bullet is
a single KPI readout).

Mark-def options:

- `bands` — ordered list of cumulative qualitative range bounds measured
  from zero, **strictly ascending** (e.g. `[150, 225, 300]`). Validated
  by `PRISM_SPEC_036`.
- `target` — the reference value to beat. A literal number, or a string
  naming a data field resolved from row 0.
- `comparative` — a secondary measure (e.g. prior period). Like `target`,
  a literal number or a data-field name.
- `orientation` — `horizontal` (default) or `vertical`.

```json
{
  "mark": {
    "type": "bullet",
    "bands": [150, 225, 300],
    "comparative": 240,
    "target": 260
  },
  "encoding": {
    "x": {"field": "actual", "type": "quantitative"},
    "y": {"field": "metric", "type": "nominal"}
  }
}
```

Validate rule: `PRISM_SPEC_036` (bands strictly ascending).

## Channel allowlists

Not every channel is valid for every mark — `theta` only makes sense
on `arc`, `source`/`target` only on `sankey`, etc. The validator
catches mismatches with `PRISM_SPEC_003`.

## Worked examples

Every mark above has a fixture in the [gallery](../gallery/index.md).
Start with:

- [bar_basic](../gallery/basic-marks/bar_basic.prism.json)
- [line_basic](../gallery/basic-marks/line_basic.prism.json)
- [histogram](../gallery/composite-marks/histogram.prism.json)
- [pie](../gallery/composite-marks/pie.prism.json)
- [sankey_user_flow](../gallery/specialty-marks/sankey_user_flow.prism.json)
