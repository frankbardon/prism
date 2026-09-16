# Encoding

The `encoding` object binds data fields to visual channels.

## Channels

| Family | Channels |
|---|---|
| Position | `x`, `y`, `x2`, `y2` (see [Span channels](#span-channels)), `theta`, `radius` |
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
| `axis` | Axis config (`orient`, `title`, `format`, `grid`, `tick_count`, `label_angle`, ...) — see [Axis placement](#axis-placement) — or `null` to [hide the axis](#hiding-an-axis-or-legend). |
| `legend` | Legend config (`title`, `orient`, `direction`, ...), or `null` to [hide the legend](#hiding-an-axis-or-legend). |
| `format` | d3-format string for label formatting. |
| `sort` | `"ascending"` / `"descending"` / `"-y"` / `[explicit, order, ...]`. |
| `stack` | Position channels only. `"zero"` / `"normalize"` / `true` to stack, `null` / `false` to opt out — see [Stacking](#stacking). |
| `key` | `true` to mark this channel as the animation join key — see [Spec › Animation](spec.md#animation). At most one channel per encoding may set this; only valid on position channels (`x`, `y`, `x2`, `y2`, `theta`, `radius`) and mark channels (`color`, `fill`, `stroke`, `opacity`, `size`, `shape`, sankey `source`/`target`/`value`, geo `longitude`/`latitude`/`feature`). |

## Span channels

`x2` and `y2` turn a position into an interval. They take the same
channel shape as `x` / `y`, with one rule that shapes everything else:
**a span channel never resolves a scale of its own.** It is measured on
the scale its base channel resolved, so both ends of a span land on one
axis and in one set of units. Two consequences follow.

- The base channel's domain is widened with the span column's values
  before the scale is built, so an interval reaching past the base
  column's own range is never clipped.
- `x2.type` must equal `x.type` (and `y2.type` must equal `y.type`).
  A mismatch is `PRISM_SPEC_043`, because declaring two types does not
  produce two scales — it produces one scale silently reading the
  second column under the first column's rules.

A span channel also needs its base channel: `x2` without `x` is an
error, as is a span channel that names no field.

### Per-mark semantics

| Mark | `x2` | `y2` |
|---|---|---|
| `bar` | Spans `x`→`x2`. The other axis keeps its band slot, so a ranged bar still needs a categorical axis on the side that is not ranged — bind both `x2` and `y2` only when you want a free-floating rect. | Spans `y`→`y2`, replacing the baseline anchor. |
| `rect` | Spans `x`→`x2`. An unranged axis uses its band width, or the historic 1-px cell when it is continuous. | Spans `y`→`y2`, same rules. |
| `rule` | Draws an interval segment from `(x, y)` to `(x2, y2)` instead of spanning the plot. Both `x` and `y` must be bound; an unbound span channel holds its endpoint at the base pixel, so `x`/`x2` + `y` is a horizontal whisker, `y`/`y2` + `x` a vertical one, and all four a diagonal. Endpoints keep their authored order, so a descending interval stays descending. | As `x2`. |
| `area` | Not supported — an area's `x` sequence is the path it traces, not an extent. | Replaces the implicit zero baseline with an explicit lower edge read per row. Grouping, x-sorting and curve interpolation are unchanged, so a `color`- or `detail`-split band keeps parallel boundaries. |
| every other mark | Rejected (`PRISM_SPEC_042`). | Rejected (`PRISM_SPEC_042`). |

Rejecting rather than ignoring is a design choice: a span channel a mark
cannot draw used to disappear silently, which made a ranged bar look
like an ordinary baseline bar with no diagnostic.

A Gantt row — categorical `y`, ranged `x`:

```json
{
  "mark": {"type": "bar"},
  "encoding": {
    "y":  {"field": "task",  "type": "nominal"},
    "x":  {"field": "start", "type": "quantitative"},
    "x2": {"field": "end",   "type": "quantitative"}
  }
}
```

A confidence band — `area` with an explicit lower edge:

```json
{
  "mark": {"type": "area"},
  "encoding": {
    "x":  {"field": "day",   "type": "temporal"},
    "y":  {"field": "upper", "type": "quantitative"},
    "y2": {"field": "lower", "type": "quantitative"}
  }
}
```

Vega-Lite's polar span channels `theta2` and `radius2` are **not**
implemented. They are not in the schema, so a spec carrying either is
rejected at decode.

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
extent is widened by `zero` and then rounded by `nice`. How that
resolved domain is then laid onto pixels is a separate set of keys —
see [Mapping the domain onto pixels](#mapping-the-domain-onto-pixels--clamp-reverse-round).

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

### Rows outside the domain — overflow and clip

Pinning `domain: [90, 130]` on a column that holds a 150 puts that row
outside the plot rect. Prism's answer is **overflow with a clip**: the
row is encoded at its true position, and the plot region clips whatever
reaches past its edge.

Three things it is not, by design:

- **Not dropped.** The row still produces a mark, so a line stays
  continuous through it and an aggregate still counts it.
- **Not clamped.** Pulling the value back to the domain bound is
  `scale.clamp`'s job. Making it the default would make that key
  meaningless and would silently misreport the value's magnitude.
- **Not drawn over the chrome.** Without a clip, a line reaching past
  the top of the plot would cross the title, and a bar reaching past
  the left would cross the y-axis labels.

The clip is expressed in the Scene IR, not invented by a renderer: the
encoder registers the plot rect as a `defs.clips` entry and points
`scene.clip_ref` at it, and every renderer applies it to the **mark
container alone**. Axes, gridlines, legends and the title are siblings
of that container and stay unclipped — including an axis raised above
the marks with [`axis.zindex`](#axis-components-and-geometry), which is
emitted outside the container for exactly this reason. Gridlines need no
clip of their own: they are generated from ticks inside the domain, so
they can never reach past the plot edge.

**When the clip is armed.** By default, only when a position channel
(`x`, `y`, `x2`, `y2`) pins an explicit `scale.domain`. That is the one
way a mark can land outside the plot rect — with a data-derived domain
the bounds come from the very rows being drawn, so every mark is inside
by construction, and arming a clip there would only risk shaving a
stroke or a glyph that legitimately overhangs the edge by a pixel.

**Forcing it either way.** `mark_def.clip` overrides the default:

```json
"mark": {"type": "line", "clip": true}
```

`true` always clips, `false` never does. The clip bounds one plot rect
rather than one mark, so in a `layer` a single `clip: true` arms it for
every layer and a `clip: false` otherwise disarms it for all of them.
`concat` / `facet` / `repeat` cells each own a plot rect and decide
independently.

### Choosing the colors — `range`, `scheme`, `interpolate`

`scale.range` is an inline list of colors: the alternative to naming a
`scheme`. It is the **top** of the palette cascade, so it beats both
`scheme` and whatever the active theme's
[range slots](./themes.md#color-schemes) supply:

```json
"color": {
  "field": "origin", "type": "nominal",
  "scale": {"range": ["#4c78a8", "#f58518", "#54a24b"]}
}
```

Colors are `#rrggbb` or `#rrggbbaa`. An entry Prism cannot parse is
dropped and the rest still win; a range whose every entry is
unparseable falls through to the next tier rather than blanking the
chart. On a discrete color channel the list is indexed positionally —
the i-th category takes the (i mod n)-th color — which is what the
symbol legend's swatches show. On a quantitative color channel the
list is the ramp's control points, interpolated between.

**`range` is honoured on color channels only.** On a position channel
(`x`, `y`, `x2`, `y2`, `theta`, `radius`) it is rejected at validate
with `PRISM_SPEC_045`, not silently ignored. A position scale's range
is the plot rect Prism computes, and the axis, the gridlines and every
mark all measure against that same rect — a spec-supplied range would
move the marks without moving the chrome, producing a chart whose axes
disagree with the data they label. Bound the axis with `domain` /
`zero` / `nice`, and size the rect with `width` / `height` or the band
paddings. Vega-Lite's non-color ranges (`size`, `opacity`, a named
range reference string) are not implemented; those forms read as "no
explicit range" and fall through to the rest of the cascade.

`scale.interpolate` picks the colorspace a **continuous** ramp is
traversed in:

| Value | Effect |
|---|---|
| `rgb` | Default. Blends the sRGB components directly. |
| `hsl` | Blends hue along the shorter arc, keeping saturation up where sRGB would pass through grey. |
| `lab` | Blends in CIELAB, which is roughly perceptually uniform, so the ramp's steps read as evenly spaced. |

Vega-Lite's `hcl` is **not** supported — the schema rejects it.

```json
"color": {
  "field": "density", "type": "quantitative",
  "scale": {"scheme": "viridis", "interpolate": "lab"}
}
```

The interpolation happens once, at encode time: Prism resamples the
ramp into evenly spaced sRGB stops that already trace the requested
space's curve. Everything downstream — the SVG renderer, the Scene
IR's gradient stops, the browser web component — only ever blends
neighbouring stops linearly, so no renderer carries colorspace math of
its own and every backend agrees. `interpolate` has no effect on a
discrete palette, which is indexed rather than traversed.

### Mapping the domain onto pixels — `clamp`, `reverse`, `round`

`domain` / `zero` / `nice` decide *what* the scale covers. Five further
keys decide *how* that domain lands on pixels.

| Key | Type | Default | Applies to | Effect |
|---|---|---|---|---|
| `clamp` | boolean | `false` | continuous | Pins an out-of-domain value to the nearest domain edge instead of letting it map past the range. |
| `reverse` | boolean | `false` | all | Runs the scale the other way round, relative to the channel's default direction. |
| `round` | boolean | `false` | all | Quantises layout to whole pixels. |
| `padding_inner` | number `[0,1)` | `0.1` | band | Gap between adjacent bands, as a fraction of the step. |
| `padding_outer` | number `[0,1)` | `0.05` band, `0.5` point | band / point | Gap before the first and after the last band. |
| `align` | number `[0,1]` | `0.5` | band / point | Where the slack left over after layout sits. |

An out-of-range padding or `align` is rejected at validate time with
`PRISM_SPEC_049`; a caller that skips validate gets the value pinned
into its legal range rather than an inside-out band.

**`clamp`.** By default a value outside the resolved domain keeps
mapping past the range and is clipped by the plot rect — the useful
behaviour when a pinned `domain` crops outliers on purpose. Turn
`clamp` on to pile them against the edge instead:

```json
"y": {
  "field": "score", "type": "quantitative",
  "scale": {"domain": [0, 100], "clamp": true}
}
```

`clamp` is a continuous-family knob. A band / point / ordinal scale has
no out-of-domain image to pin — an unknown category is an error, not an
overflow. On a `log` scale `clamp` pins positive out-of-domain values,
but a zero or negative value still has no image at all and raises
`PRISM_SPEC_010`.

**`reverse` is axis-relative, not screen-relative.** Prism's y scales
already run bottom-to-top: the domain minimum is handed the *bottom*
pixel, because SVG's origin is the top-left corner. `reverse` composes
with that existing inversion rather than replacing it. Concretely:

- on `x`, `reverse: true` puts the domain minimum on the **right**;
- on `y`, `reverse: true` puts the domain minimum at the **top**, so
  the axis reads downward.

Axes, ticks and gridlines all place through the same scale, so they
follow automatically — a reversed axis relabels itself rather than
needing a separate `axis` override.

Continuous families implement `reverse` by flipping the pixel range.
Discrete families instead hand the computed slots to the categories
back to front — d3-scale's own band behaviour — which keeps band widths
and the signed step untouched, so a reversed band scale on `y` still
draws horizontal bars correctly. The resolved domain order is not
changed either way: `scale.domain` still lists categories in the order
the axis reads them before reversal.

**`round` is layout, not serialisation.** On a band or point scale it
floors the step and rounds the leading offset and the band width, which
is what makes band edges land on device pixels and stops adjacent bars
sharing a half-pixel seam. On a continuous scale it rounds the resolved
pixel. It is independent of the renderer's pinned 3-decimal coordinate
quantisation (see [Scene IR](./spec.md)) — both can apply, and rounding
the layout first is what actually produces whole numbers in the output.

**Band geometry.** A band scale divides its range into one step per
category and draws the band inside that step:

```text
step   = span / (n - padding_inner + 2 * padding_outer)
offset = (span - step * (n - padding_inner)) * align
width  = step * (1 - padding_inner)
left_i = range_start + offset + step * i
```

`padding` is the shorthand: on a band scale it sets `padding_inner` and
`padding_outer` together, on a point scale it sets the (only) outer
padding. An explicit `padding_inner` / `padding_outer` outranks it.

```json
"x": {
  "field": "origin", "type": "nominal",
  "scale": {"padding_inner": 0.4, "padding_outer": 0.2}
}
```

The defaults — `0.1` inner, `0.05` outer, `0.5` align — are Vega-Lite's,
and they reproduce Prism's pre-split band layout exactly (which spent
half an inner gap at each end of the range), so adopting the split moved
no existing chart. A point scale has no inner padding — every band
collapses to a point — so `padding_inner` never reaches it; its outer
padding defaults to `0.5`, which is what centres the first and last
point half a step inside the range.

`align` only moves where the leftover slack sits; it never changes the
band width. `0` packs the bands against the range start, `1` against the
end, `0.5` centres them.

## Axes & legends

Both are auto-generated based on the encoded channels but can be
overridden per channel. Bundled support: 4 orientations
(bottom/left/top/right — see [Axis placement](#axis-placement)),
major + minor ticks, grid toggle, label rotation, overlap handling,
gradient + symbol legends.

### Axis placement

`axis.orient` on a position channel picks the side of the plot the
axis occupies:

```json
"encoding": {
  "x": {"field": "month", "type": "nominal", "axis": {"orient": "top"}},
  "y": {"field": "sales", "type": "quantitative", "axis": {"orient": "right"}}
}
```

| Channel | `orient` | Result |
|---|---|---|
| `x` | `bottom` | Default. Axis below the plot. |
| `x` | `top` | Axis above the plot. |
| `y` | `left` | Default. Axis to the left of the plot. |
| `y` | `right` | Axis to the right of the plot. |

An x axis runs horizontally and a y axis vertically, so only two of
the four sides are meaningful per channel. The other two —
`{"x": {"axis": {"orient": "left"}}}` — are rejected at validation
with `PRISM_SPEC_044` rather than silently ignored.

**The padding follows the axis.** The side an axis moves to reserves
the room for its tick marks, labels and title; the side it left
releases it and the plot rect expands into the freed space. A chart
with `x` on top and `y` on the right therefore has the same plot size
as the default one, mirrored. Grid lines, tick marks and the domain
line all move with the axis.

Orient composes with the rest of the axis vocabulary. `"axis": null`
wins over it: a hidden axis reserves nothing on any side, so the
orient is moot (see [Hiding an axis or legend](#hiding-an-axis-or-legend)).

Under `layer`, the layers share one pair of axes, so `orient` folds
across them first-specified-wins like every other axis property — the
first layer that sets it decides the side, and a later layer asking
for a different one raises `PRISM_WARN_AXIS_CONFIG_CONFLICT` and is
ignored. Under `facet` and `repeat`, every cell renders the same child
spec, so the child's `orient` places the shared axis for the whole
grid: a top-oriented x axis anchors to the top row instead of the
bottom, and a right-oriented y axis to the last column instead of the
first.

### Axis components and geometry

An axis is three drawable components plus a title, and the `axis`
block on a position channel controls each one separately:

```json
"x": {
  "field": "day", "type": "nominal",
  "axis": {
    "labels": true, "ticks": true, "domain": true,
    "tick_size": 5, "label_padding": 4, "label_limit": 0, "zindex": 0
  }
}
```

| Key | Default | Effect |
|---|---|---|
| `labels` | `true` | Draws the tick labels. `false` suppresses **only** the labels — the tick marks, the domain line, the grid and the title all stay. |
| `ticks` | `true` | Draws the tick marks. `false` suppresses **only** the marks; the labels sit at the same tick positions and are unaffected, as are the grid lines. |
| `domain` | `true` | Draws the axis line along the plot edge. `false` suppresses **only** that line. |
| `tick_size` | `5` | Major tick length in pixels. Minor ticks stay proportionally shorter (0.6×). |
| `label_padding` | `4` | The gap between the axis line and its tick labels. Added to a fixed per-side text allowance, so `0` puts the labels as close as the baseline permits. |
| `label_limit` | `0` (no limit) | Maximum label width in pixels. A wider label is truncated with an ellipsis (`Engineering` → `Engi…`). A limit too small to hold even the ellipsis drops the label. |
| `zindex` | `0` | `0` draws the axis and its grid lines **behind** the marks; any positive value draws them **in front**. |

The three visibility switches compose independently — set any
combination and each component obeys only its own key. They are also
finer-grained than [`"axis": null`](#hiding-an-axis-or-legend), which
suppresses the whole block: with `labels`, `ticks` and `domain` all
`false` the axis is still emitted and still draws its grid lines and
its title.

**Suppression releases padding.** The margin an axis reserves on its
side of the plot is the sum of a tick-mark share and a label share, so
`"ticks": false` or `"labels": false` hands that share back and the
plot rect expands into it. The domain line rides on the plot edge and
reserves nothing, so hiding it moves nothing. Reservations are fixed
pixel metrics rather than measured text: a `tick_size` or
`label_padding` far larger than the default draws into the outer
margin instead of growing the reservation.

**`zindex` and clipping.** An above-marks axis is emitted as a sibling
of the mark container, not a child, so it is never subject to the
[plot-region clip](#rows-outside-the-domain--overflow-and-clip) that
keeps out-of-domain marks inside the plot rect — its labels and title
still draw in the margin. Under `layer` and `facet` a *shared* axis is
emitted after every cell and therefore always draws above the marks,
whatever its `zindex`. That shared-axis ordering is a known fidelity
gap, not a clipping one: a shared axis lives outside every cell's mark
container, so the clip never reaches it either way. Honouring `zindex`
there would move the shared axes ahead of the cells and restack every
layered and faceted chart, so it is held for a change of its own, made
on purpose.

**Truncation is measured with a fixed heuristic.** Prism runs no text
measurement pass; `label_limit` (like overlap detection) estimates 6 px
per character. Truncation happens once, at encode time, so the label
text in the Scene IR is already shortened and every renderer agrees.

**Spec beats theme.** `tick_size` and `label_padding` also exist as the
theme tokens `--prism-axis-tick-size` and `--prism-axis-label-padding`.
The `axis` block wins outright wherever it states a value; the theme
token applies only where it says nothing, and Prism's built-in metric
is the floor. See [Themes](./themes.md#axis-geometry-precedence).

All seven keys survive composition. Under `layer` and `facet` with the
default (shared) resolve mode they are folded from the children with
the same first-specified-wins rule as every other `axis` property —
see [Composition](./composition.md).

### Ticks and gridlines

**Gridline positions are tick positions.** Prism emits one gridline per
*major* tick, so whatever picks the ticks picks the gridlines with it.
That coupling is intentional and matches Vega-Lite — there is no
separate "grid values" knob, and there is not going to be one. Minor
ticks get a tick mark but never a gridline.

Three keys on the `axis` block choose the tick set:

| Key | Effect |
|---|---|
| `tick_count` | How many major ticks to aim for on a continuous axis. A *hint*: the generator rounds to a readable step (1/2/5 × 10ⁿ) and may land either side of the request. Default `5`. `0` removes every tick — and therefore every gridline — while keeping the domain line and title. |
| `values` | An explicit tick set that replaces the generated one. Pins exactly what it names, and overrides both `tick_count` and `tick_min_step`. |
| `tick_min_step` | The smallest gap, in domain units, allowed between adjacent *generated* ticks. |

```json
"y": {
  "field": "revenue", "type": "quantitative",
  "scale": {"domain": [0, 80]},
  "axis": {"values": [0, 25, 50, 75]}
}
```

That spec draws four gridlines, at 0, 25, 50 and 75, and nothing else.

`tick_count` applies to the continuous families — `linear`, `log`,
`pow`, `sqrt` and `time`. On a `log` axis the decades drive the tick
set, so the count acts as a ceiling: when there are more decades than
the count allows, every *k*-th decade survives and the mantissa minor
ticks are dropped. Discrete (`band`, `point`, `ordinal`) axes tick once
per category and ignore `tick_count`.

`values` works on every family. Pin numbers on a quantitative axis,
ISO-8601 date strings (or epoch milliseconds) on a temporal one, and
exact category names on a discrete one:

```json
"x": {"field": "month", "type": "temporal", "axis": {"values": ["2021-01-01", "2021-07-01"]}}
"x": {"field": "region", "type": "nominal",  "axis": {"values": ["north", "south"]}}
```

An entry the axis cannot place — outside the resolved scale domain,
unreadable for the scale family, or a category the domain does not
contain — is **dropped**, not drawn off-plot, and reported as
`PRISM_WARN_AXIS_VALUES_DROPPED` listing every casualty. A dropped
value takes its gridline with it. If the values are the ones you want,
widen the domain with [`scale.domain`](#scales).

**Pinning suppresses minor ticks.** Minor ticks are midpoints of a
generated nice sequence; an author-chosen set has no such sequence to
halve, and inventing midpoints between arbitrary pinned values would
add tick marks nobody asked for. So `values` yields exactly the ticks
it names — all major, all with gridlines. `tick_count` and
`tick_min_step` leave the generator in charge, so minor ticks are
recomputed from whatever majors come out.

`tick_min_step` is enforced by asking the generator for *fewer* ticks
until the gap opens, rather than by thinning the result, so the
survivors stay round numbers (0/50/100, not 0/40/80). It shapes
generated ticks only: `values` pins exactly what it names, spacing
included.

All three keys survive composition. Under the default (shared) resolve
mode a `layer` or `facet` folds its children's `axis` blocks together
property by property, first-specified-wins, so `tick_count` set on one
layer reaches the shared axis — see
[Composition](composition.md).

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
### Hiding an axis or legend

> **Two legend-suppression syntaxes exist.** `"legend": null` is the
> **canonical** form — it mirrors Vega-Lite and is symmetric with
> `"axis": null`. `{"legend": {"orient": "none"}}` is an accepted **alias**
> with identical behaviour. Prefer `null`; the alias exists because both
> landed in the same release, and it is kept for compatibility rather than
> because two spellings are desirable.

### Details

`"axis": null` on a position channel suppresses that axis entirely —
domain line, ticks, tick labels, title and grid lines all go. The
padding the axis reserved on its side of the plot is released, so the
plot rect expands into the freed space.

`"legend": null` on a mark channel suppresses that channel's legend.
Legends overlay the plot rather than reserving a side, so nothing
moves; the legend simply is not emitted.

```json
"encoding": {
  "x": {"field": "cat", "type": "nominal", "axis": null},
  "y": {"field": "val", "type": "quantitative"},
  "color": {"field": "cat", "type": "nominal", "legend": null}
}
```

**Omitting the key is not the same as `null`.** The two are distinct
states and Prism decodes them apart:

| Wire form | Meaning |
|---|---|
| key absent | Default axis / legend, rendered with Prism's defaults. |
| `"axis": {...}` / `"legend": {...}` | Configured axis / legend — including an empty `{}`, which still renders. |
| `"axis": null` / `"legend": null` | Suppressed. Nothing is drawn, and an axis releases its padding. |

The distinction survives a round-trip: re-serialising a spec re-emits
the explicit `null` and never invents one for an absent key.

This is block-level suppression and is unrelated to `axis.title:
false`, which drops only the axis *title* and leaves the ticks,
labels, line and grid in place — or to `axis.labels` / `axis.ticks` /
`axis.domain`, which each drop one component and leave the rest
standing. See [Axis components and geometry](#axis-components-and-geometry).

In a `layer`, the layers share one pair of axes, so the suppression
has to be unanimous: an axis is hidden only when **every** layer that
binds the channel sets `"axis": null`. In a `facet` or `repeat`, every
cell renders the same child spec, so its suppression applies to the
whole grid — the shared axis is dropped and each cell expands.

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

## Stacking

A `bar` or `area` whose measure channel is **aggregated** and whose
marks are split by a grouping channel (`color` or `detail`) stacks by
default, matching Vega-Lite. Without stacking, each segment would be
drawn from the axis baseline and the taller ones would simply cover
the shorter ones.

```json
"encoding": {
  "x": {"field": "quarter", "type": "nominal"},
  "y": {"aggregate": "sum", "field": "revenue", "type": "quantitative"},
  "color": {"field": "segment", "type": "nominal"}
}
```

The `stack` key on the measure channel controls it explicitly:

| Value | Meaning |
|---|---|
| absent | The default above: stack when the shape qualifies, otherwise don't. |
| `"zero"` / `true` | Stack from the baseline. |
| `"normalize"` | Rescale each stack onto `0…1` — the 100% stacked chart. |
| `"center"` | Reserved for the streamgraph offset; currently resolves to **no stacking**. |
| `null` / `false` | Opt out; every segment returns to the baseline. |

Semantics:

| Aspect | Behaviour |
|---|---|
| Marks affected | `bar` and `area` only. Every other mark ignores `stack`. |
| Stack key | The **other** position channel — the dimension axis. One stack per distinct value. |
| Segment order | First-appearance order of the (colour, detail…) tuple across the whole table — the same order the mark partitioner and the legend use, so a segment sits in the same slot in every stack. |
| Axis domain | The stacked totals reach scale resolution, so the measure axis spans `0…sum`, not `0…max`. |
| Negatives | Positive and negative values accumulate independently from zero, so a mixed-sign stack grows in both directions. |
| Opt-outs | An explicit `x2` / `y2` span wins (the mark already knows both edges), as does a non-linear measure scale. |

Stacking is not a rendering trick: it compiles to a real
[`stack` transform](spec.md#stack-transform) node in the plan, whose
`<field>_start` / `<field>_end` output columns are bound to the
measure channel and its span companion before any scale resolves.
`prism execute` shows them, and any consumer pinning the plan's JSON
shape sees them too.

Composition note: stacking resolves per leaf spec, so each `layer` /
`concat` child stacks independently. A `facet` parent builds its
upstream pipeline once with the child encoding stripped, so a faceted
child does not stack — the same limitation that already applies to the
synthetic encoding aggregate.

## Further reading

- [Spec field reference](../reference/spec.md) — every channel
  property exhaustively.
- [Themes](themes.md) — how scale color schemes resolve.
