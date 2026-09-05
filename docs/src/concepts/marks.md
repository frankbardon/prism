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
| `table` | Interactive, paginated data table. Columns replace x/y — see [Table](#table) below. |
| `custom` | Escape hatch for a caller-registered renderer function. No position channels — see [Custom](#custom) below. |

### Spark adornments

The `sparkline`, `sparkbar`, and `sparkarea` marks accept three opt-in
mark-def fields that emphasize specific values on the bare spark. All
three default **off** — a spark with none set renders byte-identically
to one without the fields. They are independent and compose freely; set
any combination on the same mark.

| Mark-def field | Type | Effect |
|---|---|---|
| `point_last` | boolean | Draws an emphasis dot on the final (most recent) value. |
| `point_extent` | boolean | Draws highlight dots on the minimum and maximum values. |
| `reference_band` | `{from, to}` | Shades a faint horizontal normal-range band, spanning the full spark width between the two value-axis bounds, behind the series. |

Dots inherit the spark's line color; the band is a faint fill of the
same color. `from` / `to` are data-space values on the spark's value
axis and may be given in either order. The `winloss` mark is **not** in
scope for adornments — its bars encode direction, not a continuous
series.

```json
{
  "mark": {
    "type": "sparkline",
    "point_last": true,
    "point_extent": true,
    "reference_band": {"from": 15, "to": 22}
  },
  "encoding": {
    "x": {"field": "t", "type": "quantitative"},
    "y": {"field": "v", "type": "quantitative"}
  }
}
```

### Tree / dendrogram / network

Hierarchical and relational marks share a small layout package
(`encode/marks/layout`) and decompose to existing primitives (path,
point, rect, text) so the SVG renderer handles them without
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

### Image and path

`image` and `path` are single-geometry escape hatches: each spec emits
exactly one mark from a mark-def field rather than one mark per data
row. They take no positional data series of their own — `encoding` may
be left empty (`{}`).

**`image`** places a raster sprite at a position. Key fields:

- `url` (string, required) — the image source, read from `mark_def.url`.
  Offline-first: only `data:` URLs (e.g. base64-encoded PNG) and
  relative paths are accepted; remote `http(s)` fetch is rejected at
  validate time by `PRISM_SPEC_016`. The string passes through verbatim
  to the rendered `<image href>`.
- `size` (number) — side length in pixels. Images are square; defaults
  to `64`.
- Position — when both `x` and `y` channels are bound, the image anchors
  at the scaled value of row 0; with no position channels it lands at
  the plot region's top-left quarter (a sensible single-decoration
  default).

```json
{
  "mark": {"type": "image", "url": "data:image/png;base64,iVBOR...", "size": 64},
  "encoding": {}
}
```

**`path`** draws a raw SVG path — the escape hatch for primitives Prism
does not model natively. Key field:

- `path` (string, required) — the SVG `d` string, read from
  `mark_def.path` and passed through untouched to the rendered
  `<path d=...>` (the renderer handles attribute escaping). An empty `d`
  is rejected by `PRISM_SPEC_017`.

Standard style props (`fill`, `stroke`, `stroke_width`, `opacity`) apply.
For a data-driven polyline, prefer `line` with `x`/`y` encodings.

```json
{
  "mark": {"type": "path", "path": "M 100 100 L 200 100 L 150 200 Z", "fill": "#3b82f6"},
  "encoding": {}
}
```

Validate rules: `PRISM_SPEC_016` (image URL allowed), `PRISM_SPEC_017`
(non-empty path `d`).

### Table

`table` is an interactive, paginated data table (E1). It has no
position channels — `encoding.columns[]` is the entire visual
contract, and each entry is a standard channel binding (`field`,
`type`, `aggregate`, `title`, `format`, …) plus an optional `mark`
naming a sub-mark that renders that column's cells (e.g. `sparkline`
for an inline trend column) instead of formatted text.

Mark-def options:

- `page_size` — rows rendered per page. Defaults to `25` when unset
  (`spec.TablePageSizeDefault`).

Column fields (`encoding.columns[]`, one object per column):

- `field`, `type`, `aggregate`, `scale`, `title`, `format`, `bin`,
  `sort`, `value`, `condition` — same shape and meaning as any other
  channel encoding.
- `mark` — optional sub-mark rendering this column's cells (e.g.
  `"sparkline"`). Omit to render the column as formatted text.

```json
{
  "mark": {"type": "table", "page_size": 50},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative", "aggregate": "sum", "format": "$,.0f"},
      {"field": "trend", "type": "quantitative", "mark": "sparkline"}
    ]
  }
}
```

Validate rule: `PRISM_SPEC_040` (`encoding.columns[]` required and
non-empty). See [Renderer compatibility](#renderer-compatibility)
below for the `svg` vs `html` backend split, and the [gallery `table/`
entries](../gallery/index.md#table) for full worked examples
(including a paginated plain-column table and a `sparkline`
sub-mark column).

### Custom

`custom` (E2) is the escape hatch for a visualization none of the
built-in marks express: a consuming application registers its own
render function under a name (`prism.RegisterCustomMark(name,
renderer)`), and a spec references that name instead of describing
geometry. Like `table`, it has no position channels — the mark-def
`renderer` field is the entire visual contract, and `encoding` may be
left empty (`{}`).

Mark-def field:

- `renderer` (string, required) — the name a `CustomRenderer` was
  registered under. Always a plain string key, never executable code
  — the spec JSON never carries the implementation itself (this
  preserves Prism's no-expression-language invariant). Resolved
  against the active registry at render time, not decode time: an
  unregistered name is a render-time error
  (`PRISM_RENDER_CUSTOM_MARK_NOT_FOUND`), not a validate-time one.

A registered renderer implements at least one of two Go interfaces
(`prism.SVGCustomRenderer` / `prism.HTMLCustomRenderer` — thin
re-exports of `github.com/frankbardon/prism/custommark`, the package
that actually owns the registry), or is registered as a synchronous JS
callback in the browser via `prism.registerCustomMark(name, fn)`. Both
paths, the full SVG/HTML dual-method fallback matrix, and — most
importantly — **the security contract (the renderer author owns
escaping row data and owns all script execution, not Prism)** are
covered in the [Custom marks cookbook
entry](../cookbook/custom-marks.md).

```json
{
  "mark": {"type": "custom", "renderer": "badge"},
  "encoding": {}
}
```

Errors: `PRISM_RENDER_CUSTOM_MARK_NOT_FOUND` (unregistered `renderer`
name at render time, naming every currently-registered name in its
details). See [Renderer compatibility](#renderer-compatibility) below
— unlike `table`, `custom` renders through **both** backends, since a
renderer can implement `RenderSVG`, `RenderHTML`, or both.

## Channel allowlists

Not every channel is valid for every mark — `theta` only makes sense
on `arc`, `source`/`target` only on `sankey`, etc. The validator
catches mismatches with `PRISM_SPEC_003`.

## Renderer compatibility

Every mark listed above renders through both Go backends
(`render/svg` and `render/html` — see [Themes: Rendering
backends](themes.md#rendering-backends)); `render/html` reuses
`render/svg`'s own emitters internally, so there is nothing
mark-specific to opt into.

The one exception is the `table` mark: it renders as DOM/CSS markup —
sortable/paginated rows, row selection — with no SVG geometry
equivalent. Requesting a top-level `table` mark via the `svg` backend
fails with `PRISM_RENDER_MARK_UNSUPPORTED` naming the mark and
backend, rather than silently emitting an empty `<svg>`; render it via
the `html` backend instead. This restriction applies only to a
`table` mark used directly — embedding a geometry-bearing mark (e.g.
a `sparkline` column) inside a table's cells is unaffected and
renders normally via either backend (the `html` backend re-invokes
`render/svg`'s own emitters for that one cell's inline `<svg>`).

The `html` backend's `<table>` markup is inert until wired up with
`static/vendor/prism/prism-table.mjs` (E1-S5): `installTableHandlers(root)`
attaches header-click sort (by each column's underlying field value —
read from a `data-prism-sort-value` attribute stamped on every `<td>`,
not the cell's rendered display, so a `sparkline` column sorts by its
numeric series rather than by its `<svg>` markup), client-side
pagination (slices the already-rendered rows using `page_size`; no
extra network/WASM round trip), and row-click selection (dispatches
the same structured `prism:select` event other marks emit, keyed off
the `data-prism-datum-row` attribute every `<tr>` carries). This path
is independent of the `<prism-chart>`/WASM pipeline — a table mark
never renders through `prism.wasm`'s SVG-only bridge — so a host page
that serves/mounts `html`-backend output imports `prism-table.mjs`
directly rather than through `prism-element.mjs`.

## Worked examples

Every mark above has a fixture in the [gallery](../gallery/index.md),
with one exception: `custom` has no gallery fixture, since rendering
one requires a registered `CustomRenderer` implementation (Go code),
not just a JSON spec — see the [Custom marks
cookbook](../cookbook/custom-marks.md) for worked, runnable examples
instead. Start the gallery tour with:

- [bar_basic](../gallery/basic-marks/bar_basic.prism.json)
- [line_basic](../gallery/basic-marks/line_basic.prism.json)
- [histogram](../gallery/composite-marks/histogram.prism.json)
- [pie](../gallery/composite-marks/pie.prism.json)
- [sankey_user_flow](../gallery/specialty-marks/sankey_user_flow.prism.json)
- [table_revenue_trend](../gallery/table/table_revenue_trend.prism.json) (`html` backend; renders a `sparkline` sub-mark column)
