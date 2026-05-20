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
| `heatmap` | `rect` + 2D bin + sequential color scale. |
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
| `image` | Sprites / data-URL images at position. |
| `path` | Raw SVG path data — escape hatch. |

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
