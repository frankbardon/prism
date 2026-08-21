# Encoding

The `encoding` object binds data fields to visual channels.

## Channels

| Family | Channels |
|---|---|
| Position | `x`, `y`, `x2`, `y2`, `theta`, `theta2`, `radius`, `radius2` |
| Color & opacity | `color`, `fill`, `stroke`, `opacity` |
| Size & shape | `size`, `shape` |
| Text & order | `text`, `tooltip`, `order`, `detail` |
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
| `legend` | Legend config (`title`, `orient`, `direction`, ...). |
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

## Axes & legends

Both are auto-generated from the encoded channels and can be
overridden per channel. Bundled support: 4 orientations
(bottom/left/top/right), major + minor ticks, grid toggle, label
rotation, overlap handling, gradient + symbol legends.

The defaults below are what Prism picks when the spec says nothing.
Anything stated on `encoding.<channel>.axis` wins over all of them.

### Grid: one axis carries it

One axis carries the reference lines, and it is the **measure** axis —
the one whose values a reader interpolates. Categorical positions are
read off the label directly, so a grid line through them adds a stroke
and no information.

| Chart | Grid |
|---|---|
| Vertical bars, lines, scatter | horizontal, off the y axis |
| Horizontal bars (categorical y) | vertical, off the x axis |
| Both axes categorical (heatmap) | none |

A scatter with two continuous axes takes the y grid only, not a full
mesh. Whichever axis carries the grid drops both its domain line and
its tick marks: a grid line already lands on those pixels, and a domain
line under a grid line is two strokes of different weight on one edge.

Set `"axis": {"grid": true}` on a channel to force a grid onto it; that
also restores its own domain line and ticks.

### Zero baseline

Whether an axis includes zero depends on what the mark encodes:

- **Bars, areas, rects, histograms, box plots, violins, bullets,
  funnels** measure *magnitude* — the length of the mark is the value —
  so their axis always includes zero. A bar chart truncated at 40 says
  something false about the ratio between 48 and 62.
- **Lines, points, rules, ticks** mark *position*. Forcing zero on them
  is not honesty but the opposite: it compresses the variation the
  chart exists to show into a strip at the top. Conversion rates of
  3.2% to 3.6% become a flat line.

Either way the axis is labelled, so a reader can always see where it
starts. Override per channel with `"scale": {"zero": true}` or
`{"zero": false}`.

Where zero falls strictly *inside* a domain, it is drawn as an
emphasised baseline (`.prism-zero-line`, `--prism-axis-zero-*`) rather
than as one more grid line.

### Tick count and domain rounding

The tick count comes from the plot's pixel extent — roughly one label
per 150px vertically and 190px horizontally — not from a fixed number,
so a facet cell and a full-width chart are both legible. Continuous
domains round outward to the tick step, which puts the topmost grid
line on the plot's own edge instead of leaving an unexplained strip
above it.

### Tick labels

With no `axis.format`, labels are chosen from the tick **step** and the
domain **magnitude** together:

| Domain | Renders as |
|---|---|
| `0 … 80`, step 20 | `0`, `20`, `40`, `60`, `80` |
| `0 … 3000`, step 1000 | `0`, `1,000`, `2,000`, `3,000` |
| `0 … 2000000`, step 500000 | `0`, `0.5M`, `1M`, `1.5M`, `2M` |
| `2.8 … 3.8`, step 0.2 | `2.8`, `3`, `3.2`, `3.4`, `3.6`, `3.8` |

Past 10,000 labels compact to SI (`24k`, `1.5M`, `3.2B`); below it they
keep their exact value with thousands separators. Precision follows the
step, so an axis stepping by 10 never prints `40.0`. Zero is always
`0` — never `-0`, never `0M`.

An explicit `axis.format` is a d3-format specifier and overrides all of
this.

### Label overflow

When x labels do not fit their slot the escalation is, in order:

1. keep them level;
2. rotate to −45°, which still says the whole word;
3. truncate with an ellipsis, carrying the full text as a `<title>`
   child so the value is recoverable on hover and by a screen reader;
4. hide every other label — last resort, and only on numeric axes,
   where the omitted values are interpolable. Category labels are never
   dropped this way.

Rotation before truncation because a rotated label has lost only
convenience; a truncated one has lost information.

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

## Further reading

- [Spec field reference](../reference/spec.md) — every channel
  property exhaustively.
- [Themes](themes.md) — how scale color schemes resolve.
