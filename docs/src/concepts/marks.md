# Marks

A mark is the visual primitive that data rows become — bars, lines,
arcs, etc. Specify via top-level `mark` (shorthand string) or
`mark: {type: "...", ...properties}`.

## Catalog

### Basic marks (Vega-Lite parity)

| Mark | When to use |
|---|---|
| `bar` | Compare categories. The default. Stacks by segment — see [Encoding › Stacking](encoding.md#stacking). |
| `line` | Continuous trends; ordered x-axis. |
| `area` | Filled trends. Supports negative values, an explicit `y2` lower edge, stacking — see [Encoding › Stacking](encoding.md#stacking) — and [`orient`](#orientation-markorient). |
| `point` | Scatter, dot plots. |
| `circle`, `square` | Convenience aliases for `point` with shape preset. |
| `tick` | Strip plots, ranking dot plots. Honours [`orient`](#orientation-markorient). |
| `rect` | Heatmap cells, custom rectangular layouts. |
| `rule` | Reference lines, benchmarks, ranges. |
| `text` | Inline labels, annotations. Content comes from the `text` channel — see [Text](#text). |
| `arc` | Primitive for `pie` / `donut` / sankey links. |

### Composite marks

| Mark | Internally expands to |
|---|---|
| `histogram` | `bar` + auto-bin transform. |
| `heatmap` | `rect` + 2D bin + sequential color scale. Binds an optional field-driven `opacity` channel for per-cell shading — pair it with a crosstab `zscore_vs_margin` overlay column to fade insignificant cells (significance shading). Opacity maps the field linearly over `[min, max]` to `[0.15, 1.0]`. |
| `boxplot` | `rect` (IQR) + `rule` (whiskers) + `point` (outliers). Honours [`orient`](#orientation-markorient). |
| `violin` | `area` symmetric around centerline (Epanechnikov KDE). Honours [`orient`](#orientation-markorient). |
| `pie` | `arc` with theta computed from share. |
| `donut` | `arc` with `inner_radius_ratio > 0`. |

### Specialty marks

| Mark | When to use |
|---|---|
| `sankey` | Flow diagrams (source/target/value table). |
| `funnel` | Conversion funnels — stacked trapezoids. |
| `sparkline` | Inline micro-line charts, no axes. |
| `sparkbar` | Inline micro-column charts, no axes — bar-family sibling of `sparkline`. |
| `winloss` | Equal-length micro-bars by the sign of the measured value (>0 one way, <0 the other, ==0 flat). Magnitude is ignored — only direction encodes. Honours [`orient`](#orientation-markorient). |
| `sparkarea` | Inline filled micro-area charts, no axes — area-family sibling of `sparkline`; fill reaches the y=0 baseline. |
| `bullet` | Compact KPI gauge — a measure bar over qualitative bands, with an optional comparative bar and target tick. Keeps its measure axis. |
| `progress` | Multi-row metric bars — one value bar per row on a full-scale track. The multi-row sibling of `bullet`; row labels come from the category axis. |
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
| `reference_band` | `{from, to}` | Shades a faint normal-range band between the two value-axis bounds, spanning the spark's full extent across the category axis, behind the series. |

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

### Text

The `text` mark draws one label per row at the position its `x` / `y`
channels resolve to. Label **content** comes from the `text` encoding
channel:

| `encoding.text` | Label content |
|---|---|
| `{"field": "label", "type": "nominal"}` | That column's value for the row. |
| `{"field": "score", "type": "quantitative", "format": ".1f"}` | The column's value through the [d3-format](encoding.md#text-channel) subset — `80.04` renders as `80.0`. |
| `{"value": "n/a"}` | The literal, repeated on every row (formatted too, when `format` is set). |
| *omitted* | Fallback: the `y` field's value verbatim (or the `x` field's when `y` is unbound). |

`text.aggregate` is honoured exactly like a position channel's: it
injects the same synthetic group-aggregate node, and a non-aggregated
`text` field joins the group-by alongside the other channels. So
`{"text": {"aggregate": "mean", "field": "score", "type": "quantitative"}}`
labels each group with its mean.

At least one position channel must be bound. A label-only mark (say
`x` bound, `y` omitted) is valid — the unbound axis centres the label
in the plot region rather than erroring.

```json
{
  "mark": {"type": "text", "font_size": 12, "baseline": "bottom"},
  "encoding": {
    "x": {"field": "brand_id", "type": "nominal"},
    "y": {"field": "score", "type": "quantitative"},
    "text": {"field": "score", "type": "quantitative", "format": ".0%"}
  }
}
```

Mark-def fields `align` (`left` / `right`), `baseline` (`top` /
`bottom`), `angle`, and `font_size` position and orient the label.

Arbitrary free-floating annotations ("no data" callouts not tied to a
row) are **not** reachable from a spec today — every text mark is
row-driven.

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

- `orient` — `vertical` (default) or `horizontal`; picks the direction
  the layout grows. See [Orientation](#orientation-markorient). `radial`
  is rejected (`PRISM_SPEC_046`).
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
- `orientation` — `horizontal` (default) or `vertical`. Note the field
  name: `bullet` keeps its own `orientation` rather than the shared
  `orient`, because it is not a category/measure swap — a bullet is a
  single KPI readout whose bands, comparative bar and target tick all
  rotate together, and its default is `horizontal` where `orient`'s is
  `vertical`. Folding it into `orient` would silently flip every
  existing bullet. See [Orientation](#orientation-markorient).

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

### Progress

The `progress` mark draws **one metric row per data row**: a value bar
sitting on a full-scale track, where the visible remainder of the track
reads as "distance still to go". It is the layout behind a metric-row
panel — four labelled rows, each a bar against a 0–100 ceiling.

It is the multi-row sibling of [`bullet`](#bullet). `bullet` collapses
its measure to row 0 — it is a single KPI readout — so a four-metric
panel needs a `facet` wrapper, and facet labels its rows
`"<field> = <value>"` with no format control. `progress` reads every
row, and the row labels are simply the category axis's tick labels.

Two things make it more than a bar with a background rect:

- **The measure domain is mark-owned.** `total` names the value the
  track runs to, and the measure scale is extended to reach it before
  the scale is built. The track therefore ends at the plot edge instead
  of running past it — the clipping `bullet` still suffers when a band
  bound sits above the data range.
- **The track is a separate scene mark.** Each row emits a
  `progress-track-N` rect *and* a `progress-N` value rect, in that
  order, rather than one rect with a painted backdrop. Both carry the
  row's `data-prism-datum-row` back-reference, so a hover on the filled
  part of a row behaves like a hover on its remainder.

Channel bindings:

- Horizontal (the default reading): `x` is the quantitative value, `y`
  is the nominal metric label.
- Vertical: `x` is the nominal label, `y` is the quantitative value.

Orientation comes from the shared [`mark.orient`](#orientation-markorient)
vocabulary — `progress` does **not** carry a per-mark orientation field
the way `bullet` does. You rarely write it: a nominal `y` against a
quantitative `x` already infers horizontal.

Mark-def options:

- `total` — the measure ceiling the track runs to. A literal number
  applies to every row; a string names a data field read **per row**, so
  each metric can carry its own maximum (a per-rep quota, say). Omit it
  and the track spans the data-derived domain instead. A literal must be
  positive (`PRISM_SPEC_061`).
- `thickness` — the fraction of the category band a row occupies,
  centred in it. Defaults to `0.5`; must be greater than 0 and at most 1.
- `corner_radius` — the standard mark-def field, applied to both the
  track and the value bar so they round together.

A value above its row's `total` overflows the track rather than being
clipped — over-attainment stays visible.

```json
{
  "mark": {"type": "progress", "total": 100, "corner_radius": 3, "thickness": 0.45},
  "encoding": {
    "x": {"field": "score", "type": "quantitative"},
    "y": {"field": "metric", "type": "nominal"}
  }
}
```

The track's colour comes from the active theme's grid colour, so it
tracks light / dark / print without per-chart configuration. Set the
value bar's colour with `mark.fill`, a `color` channel, or the theme's
`marks.progress` block.

Right-hand value and delta labels ("92.4", "+14.3 vs category") are not
part of the mark — layer a `text` mark over it.

Validate rule: `PRISM_SPEC_061` (both position channels bound;
`thickness` in (0, 1]; a literal `total` positive).

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

## Style properties

Beyond `type`, a `mark_def` object carries the visual properties every
mark is drawn with. They are spec-level constants — one value for the
whole mark — as distinct from an encoding channel, which varies per
row. Where a mark-def property and a theme `mark` token name the same
thing, the **mark def wins**; see
[Themes: mark style precedence](themes.md#mark-style-precedence).

### Paint

| Property | Applies to | Meaning |
|---|---|---|
| `fill` | filled marks | Fill color, `#RRGGBB` / `#RRGGBBAA`. |
| `stroke` | all | Stroke color. |
| `stroke_width` | all | Stroke width in pixels. |
| `stroke_dash` | line-family | Dash pattern, `[on, off, …]` pixels. |
| `opacity` | all | Overall element opacity, `[0, 1]`. |
| `fill_opacity` | filled marks | Fill-paint alpha, `[0, 1]`. |
| `stroke_opacity` | stroked marks | Stroke-paint alpha, `[0, 1]`. |
| `corner_radius` | `bar` / `rect` | Corner rounding in pixels. |

`opacity`, `fill_opacity` and `stroke_opacity` are **independent and
multiplicative** — none overrides another. This matches Vega-Lite,
whose canvas renderer computes the fill alpha as
`opacity × (fillOpacity ?? 1)` and the stroke alpha as
`opacity × (strokeOpacity ?? 1)`. Prism emits all three as separate
SVG attributes (`opacity`, `fill-opacity`, `stroke-opacity`), which
SVG composites the same way, so:

```json
{"mark": {"type": "bar", "opacity": 0.5, "fill_opacity": 0.5}}
```

paints a fill at an effective alpha of `0.25`, not `0.5`. Set
`fill_opacity` alone when you want a translucent fill under a solid
stroke; set `opacity` when you want the whole mark — fill, stroke and
all — to fade together. An explicit `0` is honoured (a fully
transparent paint), unlike an omitted property, which inherits.

### Clipping (`clip`)

`clip` is the one mark-def property that is not a paint at all: it
forces the **plot-region clip** on (`true`) or off (`false`).

```json
{"mark": {"type": "line", "clip": true}}
```

Omit it and the encoder decides: the clip is armed only when a position
channel pins an explicit `scale.domain`, which is the one way a mark can
land outside the plot rect. Because the clip bounds a plot rect rather
than a single mark, a `layer` resolves it once for the whole stack — one
`clip: true` arms it for every layer, and a `clip: false` otherwise
disarms it for all of them. See
[Encoding: rows outside the domain](encoding.md#rows-outside-the-domain--overflow-and-clip).

### Typography

Applies to `text` marks and to any mark that draws a text component.

| Property | Meaning |
|---|---|
| `font` | Font family, e.g. `"Inter, system-ui, sans-serif"`. |
| `font_size` | Glyph size in pixels. |
| `font_weight` | `"normal"` \| `"bold"` \| `"lighter"` \| `"bolder"`, or a number (100–900). |
| `font_style` | `"normal"` \| `"italic"` \| `"oblique"`. |
| `align` | Horizontal anchor — `"left"` \| `"center"` \| `"right"`. |
| `baseline` | Vertical anchor — `"top"` \| `"middle"` \| `"bottom"` \| `"alphabetic"`. |
| `angle` | Rotation in degrees about the anchor point. |
| `dx`, `dy` | Pixel offset from the anchor point. |

`font_weight` normalises to a number in the Scene IR, since SVG's
`font-weight` attribute is numeric. The CSS keywords map to their
computed values against the default inherited weight of 400:
`normal` → 400, `bold` → 700, `bolder` → 700, `lighter` → 100.

`dx` / `dy` are applied **after** `angle`, in the rotated frame — so a
rotated label nudged with `dy: -4` moves 4px along its own baseline
normal, not straight up the page. This is Vega's rule
(`translate(x,y) rotate(a) translate(dx,dy)`) and is what makes
`dx`/`dy` useful for lifting a label clear of the geometry it
annotates:

```json
{
  "mark": {"type": "text", "dy": -6, "font_weight": "bold", "font_style": "italic"},
  "encoding": {
    "x": {"field": "quarter", "type": "nominal"},
    "y": {"field": "revenue", "type": "quantitative"},
    "text": {"field": "revenue", "type": "quantitative"}
## Orientation (`mark.orient`)

A bar does not really have an "x axis" and a "y axis" — it has a
**category** axis (the discrete band the bar sits in, which sets its
thickness) and a **measure** axis (the continuous value, along which it
grows from the data-zero baseline). Which physical axis plays which
role is the mark's orientation.

| `orient` | Category axis | Measure axis | Bars grow |
|---|---|---|---|
| `vertical` | `x` | `y` | up/down from a baseline at `y = 0` |
| `horizontal` | `y` | `x` | right/left from a baseline at `x = 0` |

The same split applies to every cartesian family, not just `bar` —
a boxplot's band and whisker caps sit on the category axis while its
quantiles walk the measure axis, an area's series runs along the
category axis and fills to a baseline on the measure axis, and a tick
draws a short segment along the measure axis at its category's centre.

### Inference

**You usually do not write `orient` at all.** It is inferred from
whichever axis carries the discrete (band) scale, the same way
Vega-Lite infers it:

| `x` scale | `y` scale | Inferred |
|---|---|---|
| band | continuous | `vertical` |
| continuous | band | `horizontal` |
| band | band | `vertical` (ambiguous; the default wins) |
| continuous | continuous | error — neither axis can host the category |

The last row holds for the marks that *need* a band to sit in (`bar`,
`rect`, `boxplot`, `violin`, `winloss`). `area` and `tick` position
rows along an axis that is usually continuous or temporal, so two
continuous axes are perfectly legal there and fall back to the mark's
historic direction — `vertical` for `area`, `horizontal` for `tick`.
An explicit `orient` still wins on those marks, and is the only way to
draw a horizontal area.

So a nominal `y` against a quantitative `x` already draws a horizontal
bar chart:

```json
{
  "mark": "bar",
  "encoding": {
    "y": {"field": "channel", "type": "nominal"},
    "x": {"field": "delta",   "type": "quantitative"}
  }
}
```

An explicit `orient` **overrides** the inference. On a mark that needs
a band it cannot invent one, though: `"orient": "horizontal"` against a
continuous `y` fails with `PRISM_ENCODE_001` naming the axis that needs
the band, rather than drawing something else and hoping you notice.

Everything else about the mark is orientation-agnostic: the baseline,
`corner_radius`, `color` grouping and the `x2`/`y2`
[span channels](encoding.md#span-channels) all behave the same in
either direction. A negative value crosses the baseline the same way
too — leftward instead of downward.

Categories run in the same direction as every other Prism `y` scale:
the first category sits at the **bottom** of a horizontal bar chart,
not the top. Pin an explicit order with
`{"scale": {"domain": [...]}}` when you want a different one.

### Which marks read it

| Mark | Meaning of `orient` | Default | Needs a band on the category axis |
|---|---|---|---|
| `bar`, `rect` | Swaps the category and measure axes, as above. | inferred, else `vertical` | yes |
| `progress` | Swaps the category and measure axes, as above. | inferred, else `horizontal` | yes |
| `boxplot`, `violin` | Swaps the axes: the band holds the box / density fan, the measure axis the quantiles or samples. | inferred, else `vertical` | yes |
| `winloss` | Swaps the axes: the streak runs across the band and the equal-length bars grow either side of the zero baseline. | inferred, else `vertical` | yes |
| `sparkbar` | As `bar` — `sparkbar` is a thin wrapper over the bar encoder. | inferred, else `vertical` | yes |
| `area`, `sparkarea` | Moves the series axis and the fill baseline. A horizontal area runs bottom-to-top and fills to `x = 0`. | `vertical` | no |
| `tick` | Moves the short segment to the other axis, centred in its category slot. | inferred, else `horizontal` | no |
| `tree`, `dendrogram`, `network` | The direction the layout grows — not a category/measure swap. | `vertical` | n/a |
| `bullet` | Uses its own `orientation` field instead (see [Bullet](#bullet)). | `horizontal` | n/a |
| `heatmap` | Not implemented — a heatmap is banded on **both** axes, so there is no category/measure split to swap. | — | — |
| everything else | Not implemented — `orient` is **rejected**, never ignored. | — | — |

Two shapes are worth calling out:

- `y2` supplies an area's explicit lower edge only while the area is
  vertical. On a horizontal area the fill measures along `x`, so a
  bound `y2` would be a second position on the series axis — that
  combination is rejected rather than quietly dropped.
- A horizontal `boxplot` or `violin` reads its *category* from `y` and
  its values from `x`, so swap the two channel bindings (or set
  `orient` explicitly) rather than only relabelling the axes.

`radial` is named by the vocabulary but implemented by no mark, so it
is rejected too. For a radial reading reach for a polar mark (`arc` /
`pie` / `donut`). Both rejections are `PRISM_SPEC_046` at validate
time; the rule is `mark orient supported` in
[`validate/RULES.md`](https://github.com/frankbardon/prism/blob/main/validate/RULES.md).

## Interpolation (`line` and `area` curves)

`mark.interpolate` selects how consecutive points are joined. It
applies to the `line` and `area` families (`sparkline` and `sparkarea`
inherit it, since they are thin wrappers over the same encoders).
Default is `linear`.

| `interpolate` | Shape |
|---|---|
| `linear` | Straight segments between points. The default. |
| `monotone` | Monotone cubic spline (Fritsch–Carlson). Smooth, and provably never overshoots the data — the safe smoothing choice. |
| `step` | Right-angle steps with the riser midway between each pair of x values. |
| `step-before` | Right-angle steps with the riser at the *earlier* x — the value changes before it is reached. |
| `step-after` | Right-angle steps with the riser at the *later* x — the value holds until the next point. |
| `cardinal` | Cardinal spline through every point. Smoother than `monotone`, but it may overshoot. |

`mark.tension` (0–1) parameterises `cardinal` only; every other method
ignores it. `0` is the default and the loosest curve; `1` collapses the
spline back to straight segments. Values outside the range are clamped.

```json
{
  "mark": {"type": "line", "interpolate": "monotone", "stroke_width": 2},
  "encoding": {
    "x": {"field": "day",  "type": "temporal"},
    "y": {"field": "load", "type": "quantitative"}
  }
}
```

### Arc geometry

| Property | Meaning |
|---|---|
| `inner_radius` | Donut hole radius in pixels (absolute; wins over the ratio). |
| `inner_radius_ratio` | Donut hole as a fraction of the outer radius, `[0, 1]`. |
| `outer_radius` | Outer radius in pixels. |
| `pad_angle` | Angular gap between neighbouring sectors, in **radians**. |

`pad_angle` is the gap *between* two sectors, not the inset applied to
one: each sector gives up half the pad at each of its two ends, so two
adjacent sectors end up `pad_angle` radians apart. A sector narrower
than `pad_angle` collapses to nothing rather than drawing backwards.

Prism applies a single constant angular inset at every radius.
d3-shape (and therefore Vega-Lite) varies the inset with radius so the
*linear* gap stays constant from the inner to the outer edge; the two
agree closely for a thin annulus and diverge for a full pie with a
large pad. `pad_angle: 0.02` (about 1.15°) is a good starting point:

```json
{"mark": {"type": "donut", "pad_angle": 0.02, "inner_radius_ratio": 0.6}}
```
An `area` applies its curve to **both** boundaries — the upper edge and
the reversed lower/baseline edge — so a band keeps parallel outlines
rather than a curved top over a straight bottom. The short connector
between the two edges is always a straight segment.

That is what makes `area` the streamgraph mark: a centred stack
(`"stack": "center"`, see
[Encoding › Centred stacks](encoding.md#centred-stacks-the-streamgraph))
hands each series a floating pair of edges, and a smooth
`interpolate` carries both of them, so the ribbons read as one
flowing stream. `bar` cannot take the centred offset — it is
baseline-anchored geometry, and `PRISM_SPEC_053` says so rather than
drawing detached columns.

Geometry semantics match d3-shape's `curveLinear`, `curveMonotoneX`,
`curveStep`/`curveStepBefore`/`curveStepAfter` and
`curveCardinal.tension(t)`, so a Prism curve and the equivalent
Vega-Lite / d3 curve trace the same path.

**Out of scope.** The `basis` and `bundle` families and d3's `-open` /
`-closed` variants are not implemented and are rejected by
`schema/v1/mark.schema.json` at validation time rather than silently
falling back.

**Rendering.** `linear` lines emit `<polyline points="…">`; every other
interpolation emits `<path d="…">` with the same `prism-mark-line`
class, identity and style attributes. The split is intentional —
`<polyline>` expresses a linear line exactly, and keeping it pins the
byte shape of every committed linear golden and cross-impl fixture.
Area marks were already `<path>` and stay so for every curve. All
control points route through `render/precision.go`'s 3-decimal
quantisation, so host Go, TinyGo-via-WASM and the browser bundle emit
identical path data.

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
the `data-prism-datum-row` attribute every `<tr>` carries). A host page
that serves/mounts server- or CLI-produced `html`-backend output
(`prism plot --format html`) imports `prism-table.mjs` directly and
calls `installTableHandlers(root)` itself, independent of the
`<prism-chart>`/WASM pipeline.

`<prism-table>` (E4-S2, registered in `prism-element.mjs` alongside
`<prism-chart>`) is the live-in-browser counterpart: it renders a
`spec`/`src` through `prism.renderHTML` (the WASM HTML backend bridge
— see [Browser: Render backends](browser.md#render-backends-svg-vs-html))
and calls `installTableHandlers` on the mounted result automatically,
so a `table` mark is now live-renderable in the browser exactly like
any other mark, just through the HTML backend instead of the SVG one.

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
