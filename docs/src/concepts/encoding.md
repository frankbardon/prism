# Encoding

The `encoding` object binds data fields to visual channels.

## Channels

| Family | Channels |
|---|---|
| Position | `x`, `y`, `x2`, `y2`, `theta`, `theta2`, `radius`, `radius2` |
| Color & opacity | `color`, `fill`, `stroke`, `opacity` |
| Size & shape | `size`, `shape` |
| Text & order | `text` (see [Text channel](#text-channel)), `tooltip`, `order` |
| Grouping | `detail` — see [Detail channel](#detail-channel) |
| Facet | `row`, `column` |
| Sankey | `source`, `target`, `value` |

## Channel shape

```json
"x": {
  "field": "score",
  "type": "quantitative",
  "aggregate": "mean",
  "scale": {"type": "log"},
  "axis": {"title": "Average score", "format": ".2f"},
  "sort": "-y"
}
```

| Key | Purpose |
|---|---|
| `field` | Column from the source (or transform output). |
| `type` | One of `nominal`, `ordinal`, `quantitative`, `temporal`. |
| `aggregate` | Friendly alias: `mean`, `sum`, `count`, `null_count`, `median`, `q1`, `q3`, `min`, `max`, `range`, `stdev`, `variance`, `skewness`, `kurtosis`, `ci0`, `ci1`, `distinct`, `mode`, `frequency`, plus `wmean`, `ratio`, `lift`, `share`. `count`, `distinct`, `mode`, `frequency`, and `null_count` work on any field type; numeric aggregates require a quantitative or temporal field. `frequency` is the scalar companion to `mode` — it returns the modal count (how many times the most frequent value occurs), whereas `mode` returns the value itself. |
| `scale` | Scale spec (`type`, `domain`, `range`, `scheme`, `padding`, ...). |
| `axis` | Axis config (`title`, `format`, `grid`, `tick_count`, `label_angle`, ...). |
| `legend` | Legend config (`orient`, `padding`, `offset`, `title`, `direction`, ...) — see [Legend placement](#legend-placement). |
| `format` | d3-format string for label formatting. |
| `sort` | `"ascending"` / `"descending"` / `"-y"` / `[explicit, order, ...]`. |
| `key` | `true` to mark this channel as the animation join key — see [Spec › Animation](spec.md#animation). At most one channel per encoding may set this; only valid on position channels (`x`, `y`, `x2`, `y2`, `theta`, `radius`) and mark channels (`color`, `fill`, `stroke`, `opacity`, `size`, `shape`, sankey `source`/`target`/`value`, geo `longitude`/`latitude`/`feature`). |

## Conditions

A channel can carry a `condition` clause that switches its visual
value based on a declared [selection](selections.md) or a structured
predicate `test`. The channel's own `value` / `field` supplies the
fallback ("otherwise") branch.

```json
"color": {
  "condition": [
    {"selection": "brush", "value": "#22c55e"},
    {"test": {"op": "lt", "field": "score", "value": 0}, "value": "#ef4444"}
  ],
  "value": "#94a3b8"
}
```

Rules:

- `selection` references a name declared in the spec's `selection`
  block (validate rule `PRISM_SPEC_025`).
- `test` is a **structured predicate** — the same grammar `filter`
  uses (`{op, field, value}` leaves and `and` / `or` / `not`
  combinators), not an expression string. It is evaluated row-by-row
  at encode time (`PRISM_SPEC_026`). See
  [Spec › Filter transform](spec.md#filter-transform) for the full
  operator set.
- Each entry needs exactly one of `value` or `field`. A
  `selection`-form entry without `value` inherits the channel's own
  field binding (`PRISM_SPEC_027`).
- Entries evaluate top-down; the first match wins.

Where the work happens:

- **`test`-driven entries** are evaluated server-side at encode time
  and baked directly into the mark's resolved style. SVG output
  reflects them with no client involvement.
- **`selection`-driven entries** land in the scene-IR as a
  `Mark.Conditions[]` slice. The browser-side `prism-selection`
  module flips the matching SVG attribute when the named selection
  becomes active, and reverts to the resolved "otherwise" branch
  when it clears.

See the [conditions gallery](../gallery/conditions) and the
[highlight-on-brush recipe](../cookbook/highlight-on-brush.md).

## Scales

Eight types: `linear` (default for quantitative), `log`, `pow`, `sqrt`,
`time` (default for temporal), `band` (default for nominal bar x),
`point` (default for nominal point x), `ordinal` (default for color
over nominal).

See the [scales gallery](../gallery/scales) for one fixture per type.

### Bounding the domain — `domain`, `zero`, `nice`

Three keys shape the resolved domain. They compose in a fixed
precedence: **`domain` wins outright**, and when it is absent the data
extent is widened by `zero` and then rounded by `nice`.

| Key | Type | Default | Effect |
|---|---|---|---|
| `domain` | array | data extent | Pins the domain exactly. `zero` and `nice` are not applied on top of it. |
| `zero` | boolean | `true` on `linear` / `pow` / `sqrt`; ignored on `log` and `time` | Widens a positive-only extent down to 0 (or a negative-only extent up to 0). |
| `nice` | boolean | `true` on `linear` / `pow` / `sqrt` / `time`; `false` on `log` | Rounds the bounds outward so the axis starts and ends on a labelled tick. |

```json
"y": {
  "field": "score", "type": "quantitative",
  "scale": {"zero": false, "domain": [90, 130]}
}
```

**`domain` shape by family.** A continuous scale (`linear`, `log`,
`pow`, `sqrt`) takes exactly two ascending numeric bounds. A `time`
scale takes two bounds, each an ISO-8601 date string or an
epoch-millisecond number. A discrete scale (`band`, `point`,
`ordinal`) takes the ordered category list:

```json
"x": {
  "field": "size", "type": "nominal",
  "scale": {"domain": ["small", "medium", "large"]}
}
```

A discrete `domain` pins the **leading** order; any data category the
list omits is appended after it in first-seen order, so a partial
domain reorders without dropping rows. This is Prism's one intentional
divergence from Vega-Lite, which treats a discrete domain as the
complete category set.

> **Retired workaround.** Explicit category order used to be reachable
> only by arranging the layers so the desired category appeared in the
> first layer's data — order fell out of the domain-union order. Set
> `scale.domain` instead; layer ordering no longer affects category
> order.

A malformed `domain` — wrong arity, non-numeric bounds on a continuous
scale, reversed or zero-width bounds, a non-string category — is
rejected at validate time with `PRISM_SPEC_041`, and the encoder
raises the same code for callers that skip validate.

**`nice` is boolean-only.** Vega-Lite's numeric (tick-count) and
time-interval forms of `nice` are rejected by the schema. Control tick
density with `axis.tick_count` instead.

**`log` has no zero-forcing, by design.** A log domain cannot contain
zero, so `zero` is ignored there — including an explicit `zero: true`.

> **Retired workaround.** Because zero-forcing used to be unconditional
> on `linear`, `scale.type: "log"` was the only way to get an axis that
> did not start at zero. That is no longer necessary: use
> `{"zero": false}` on the linear scale, or pin `domain` outright, and
> reach for `log` only when the data genuinely wants a logarithmic
> mapping.

`nice` defaults on, so adding no scale block at all moves axis bounds
compared with pre-E2-S1 Prism: an extent of 3..97 now resolves to
0..100 rather than 0..97. Set `{"nice": false}` to keep the raw extent.

## Axes & legends

Both are auto-generated based on the encoded channels but can be
overridden per channel. Bundled support: 4 orientations
(bottom/left/top/right), major + minor ticks, grid toggle, label
rotation, overlap handling, gradient + symbol legends.

### Legend placement

A legend is built from the `color` channel, and the `legend` block on
that channel places it:

```json
"color": {"field": "region", "type": "nominal", "legend": {"orient": "left"}}
```

`orient` accepts nine values, and they fall into two groups that
behave differently:

| `orient` | Group | Behaviour |
|---|---|---|
| `left`, `right`, `top`, `bottom` | **Side** | **Reserves margin.** The plot rect shrinks by the legend's width (left/right) or height (top/bottom) plus the offset gap, and the legend parks in that reserved band — outside the axis chrome, so a left legend never lands on the y axis. |
| `top-left`, `top-right`, `bottom-left`, `bottom-right` | **Corner** | **Overlays the plot.** The plot rect is untouched and the legend is drawn over its corner. This is the default (`top-right`) and the historical behaviour. |
| `none` | — | Suppresses the legend entirely. No legend is built and no margin is reserved. |

Two knobs adjust the placement:

| Key | Effect |
|---|---|
| `padding` | Interior padding. Grows the legend frame by that many pixels on every side and insets the swatches, labels and title by the same amount. A side placement widens its reserved band to match, so padding never eats into the plot. Default `0`. |
| `offset` | The gap the legend keeps from the plot. On a side placement it is the space between the legend and the plot's chrome, and it widens the reserved band (defaults: `10` for left/right, `4` for top/bottom). On a corner placement it is the inward inset from the plot corner (default `0`). |

Reservations are measured, not guessed: a symbol legend is 104 px
wide, and a top/bottom band is sized from the entry count. A channel
with fewer than two categories builds no legend, so it reserves
nothing either.

In a `layer` composite each layer resolves its own legend. Legends
sharing an anchor stack rather than overlap, and a shared side's
reservation is the sum (top/bottom) or the widest (left/right) of what
the stacked legends claim.

## Text channel

The `text` channel supplies the label content for a `text` mark (and
the line text for [tooltips](#tooltip-channel), which share the same
slimmer channel shape: `field`, `type`, `aggregate`, `format`,
`title`, `value`).

```json
"text": {"field": "score", "type": "quantitative", "format": ".1f"}
```

| Key | Effect on a `text` mark |
|---|---|
| `field` | The column whose value becomes the label. |
| `value` | A literal label, repeated on every row. Used when no `field` is set. |
| `format` | d3-format specifier applied to the label — `.1f` renders `80.04` as `80.0`, `.0%` renders `0.42` as `42%`. Invalid specifiers are rejected at validate time with `PRISM_SPEC_011`. |
| `aggregate` | Honoured exactly like a position channel's: it injects the same synthetic group-aggregate node, and a non-aggregated `text` field joins the group-by. |

With **no** `text` channel bound, a `text` mark falls back to
rendering the `y` field's value verbatim (or `x`'s when `y` is
unbound). See [Marks › Text](marks.md#text) for the mark-side
positioning rules.

## Tooltip channel

```json
"tooltip": [
  {"field": "brand_id"},
  {"field": "score", "format": ".2f"}
]
```

Materialized in the Scene IR as pre-formatted `TooltipLine` lists.
SVG emits `<title>` per mark; the JS port renders rich HTML tooltips
in P12+.

## Detail channel

`detail` is a **pure grouping channel**. It splits a mark into one
series per distinct value of the bound field — exactly as `color`
does — but it consumes no palette slot, builds no legend, and leaves
mark styling untouched. Use it when the data has more series than the
chart should distinguish visually: ten sensors that all belong to one
cohort should draw as ten separate lines in one colour, not ten
colours plus a ten-entry legend.

```json
"detail": {"field": "sensor", "type": "nominal"}
```

An array binds several fields; the grouping key is the tuple, so a
new series starts at each distinct combination:

```json
"detail": [
  {"field": "site", "type": "nominal"},
  {"field": "unit", "type": "nominal"}
]
```

Semantics:

| Aspect | Behaviour |
|---|---|
| Marks affected | `line` and `area` — the path-forming marks, whose geometry spans multiple rows. Every other mark already emits one mark per row, so a partition changes nothing; `detail` is accepted there and is simply inert. |
| Composition with `color` | Both bound produces one series per distinct (colour, detail…) pair. Every series sharing a colour keeps that colour — `detail` never advances the palette. |
| Legend | Never. The legend is built from `color` alone, so a `color` + `detail` chart still shows one entry per colour. |
| Emission order | Colour first-appearance order outer (so it continues to match legend order), full-tuple first-appearance order inner. Series sharing a colour are emitted contiguously. |
| Point ordering | As with `color`, points within a series are sorted by resolved x pixel ascending, so each path traces left-to-right regardless of upstream row order. |
| Aggregates | A `detail` field joins the implicit group-by of the synthetic aggregate an aggregated channel injects, so `detail` + `y: {"aggregate": …}` aggregates per series. |

Key coercion differs slightly between the two grouping channels:
`color` categories are string-valued (a non-string cell has no
palette entry), whereas a `detail` key is the value's string form —
a numeric series id groups per distinct number rather than collapsing
into one bucket.

## Further reading

- [Spec field reference](../reference/spec.md) — every channel
  property exhaustively.
- [Themes](themes.md) — how scale color schemes resolve.
