# Themes

Themes drive colors, fonts, spacing, and per-mark defaults across
all renderers. A single Go struct (`theme.Theme`) is the source of
truth; resolved tokens emit as CSS variables that the SVG output
and the live browser component both consume.

## Bundled themes

| Name | When to use |
|---|---|
| `light` (default) | Standard web pages, light backgrounds. Tableau10 categorical + Viridis sequential. |
| `dark` | Dark dashboards, terminal embeds. Observable10 categorical + Magma sequential. |
| `print` | Reports, print-ready output. Grayscale only, no transparency on lines, hatch-friendly. |
| `high_contrast` | Projector / presentation, low-vision readers. Pure black/white, bold weights, no grid lines. |
| `colorblind` | Colorblind-safe defaults. Okabe-Ito categorical + Cividis sequential (deuteranopia-tuned). |

## Pick at plot time

```
prism plot bar.json --theme=dark > bar-dark.svg
prism plot bar.json --theme=colorblind > bar-cb.svg
```

## Theme structure

`theme.Theme` is composed of nested blocks. Every field is optional
— absent fields inherit from the registered base.

```json
{
  "name": "my_theme",
  "base": "light",

  "mark":   { "fill": "#4c78a8", "opacity": 1 },
  "marks": {
    "bar":  { "fill": "#4c78a8", "corner_radius": 2 },
    "line": { "stroke": "#4c78a8", "stroke_width": 1.5, "fill": "transparent" },
    "area": { "fill": "#4c78a8", "opacity": 0.7 },
    "point":{ "fill": "#4c78a8", "size": 64 }
  },

  "axis": {
    "domain_color":  "#6b7280",
    "tick_color":    "#6b7280",
    "tick_size":     5,
    "grid_color":    "#e5e7eb",
    "label_color":   "#111827",
    "label_font_size": 11,
    "label_line_height": 1.2,
    "label_letter_spacing": 0.1,
    "title_color":   "#111827",
    "title_font_size": 12,
    "title_padding": 8,
    "title_line_height": 1.2,
    "title_letter_spacing": 0.1
  },

  "legend": {
    "label_color":      "#111827",
    "label_line_height": 1.2,
    "title_font_weight":"600",
    "title_line_height": 1.2,
    "symbol_size":      64,
    "padding":          8
  },

  "title": {
    "color":      "#111827",
    "font_size":  16,
    "font_weight":"600",
    "anchor":     "start",
    "line_height":    1.25,
    "letter_spacing": 0.2
  },

  "view": {
    "background":   "transparent",
    "padding":      0
  },

  "range": {
    "category":  { "scheme": "tableau10" },
    "ordinal":   { "scheme": "blues" },
    "ramp":      { "scheme": "viridis" },
    "heatmap":   { "scheme": "viridis" },
    "diverging": { "scheme": "rdbu" }
  },

  "schemes": {
    "brand_primary": ["#001eff", "#33ffaa", "#ff3366"]
  },

  "style": {
    "rule_emphasis": { "stroke": "#000000", "stroke_width": 2 }
  },

  "states": {
    "selected":   { "opacity": 1 },
    "deselected": { "opacity": 0.3 }
  },

  "filters": {
    "soft_shadow": "<feDropShadow dx=\"0\" dy=\"2\" stdDeviation=\"2\" flood-opacity=\"0.3\"/>"
  },

  "raw_css": ".prism-mark-bar:hover { filter: brightness(1.1); }",

  "gradients": {
    "brand_fade": {
      "type": "linear",
      "angle": 90,
      "stops": [
        { "offset": 0,   "color": "#4c78a8" },
        { "offset": 1,   "color": "#f58518" }
      ]
    },
    "spot_glow": {
      "type": "radial",
      "cx": 0.5, "cy": 0.5, "radius": 0.75,
      "stops": [
        { "offset": 0, "color": "#ffffff" },
        { "offset": 1, "color": "#4c78a8" }
      ]
    }
  },

  "patterns": {
    "hatch": { "type": "cross-hatch", "color": "#6b7280", "spacing": 6, "size": 1 },
    "custom_dots": { "content": "<circle cx=\"2\" cy=\"2\" r=\"1\" fill=\"#4c78a8\"/>" }
  },

  "category_styles": {
    "Origin": {
      "USA":    { "fill": "#4c78a8" },
      "Europe": { "fill": "#f58518" },
      "Japan":  { "fill": "#e45756" }
    }
  }
}
```

### Block reference

| Block | Drives |
|---|---|
| `mark`     | Default style applied to every mark unless `marks.<type>` overrides. |
| `marks.<type>` | Per-mark-type defaults. Key matches the spec's `mark.type` (bar, line, area, point, rule, text, tick, rect, arc, geoshape, geopoint, ...). |
| `axis`     | Axis domain, ticks, grid, labels, titles — applied to **both** cartesian axes. |
| `axis_x` / `axis_y` | Same token set as `axis`, layered over it **per property** for one axis only (see [Per-axis blocks](#per-axis-blocks)). Because the x axis draws the vertical grid lines and the y axis the horizontal ones, these are also how grid lines are themed per orientation. |
| `legend`   | Legend fills, symbols, labels, padding. |
| `title`    | Chart title typography. |
| `view`     | Chart-rect background, stroke, padding. |
| `range`    | Default color scheme per scale role (category, ordinal, ramp, heatmap, diverging, symbol, cyclic). |
| `schemes`  | Per-theme custom named-scheme registry. Entries shadow the global catalogue. |
| `style`    | Named-style registry — marks reference an entry via their `style` attr. |
| `states`   | State overlays (selected, deselected, hover, focus). Materialise as `.prism-<state>` CSS classes. |
| `filters`  | Named registry of raw SVG `<filter>` inner-content bodies. `mark`/`marks.<type>`/`style.<name>`/`axis`/`legend`/`title`/`view` each carry a `filter` field naming an entry here. |
| `raw_css`  | Raw CSS string appended verbatim to the emitted `<style>` block. |
| `gradients` | Named registry of linear/radial gradient definitions, referenced via `url(#name)` fills (see [Gradients and patterns](#gradients-and-patterns)). |
| `patterns`  | Named registry of pattern fills — built-in catalogue or raw-SVG content — referenced via `url(#name)` fills (see [Gradients and patterns](#gradients-and-patterns)). |
| `dark_variant` | Name of a registered counterpart theme for automatic light/dark rendering (see [Dark variant pairing](#dark-variant-pairing)). |
| `category_styles` | Field name → field value → `MarkStyle` map for theme-level data-driven styling (see [Category styles](#category-styles)). |

### Mark style precedence

A mark's final style is resolved by folding four layers in a fixed
order, each shadowing the previous **per field**. A layer that leaves
a field unset never clears what an earlier layer wrote:

1. Prism's built-in fallback for the mark type (so a chart still
   renders under a theme that declares nothing).
2. The theme's `mark` block — the global default for every mark.
3. The theme's `marks.<type>` block — the per-mark-type override.
4. The spec's own `mark_def` — see
   [Marks: style properties](marks.md#style-properties).

**The spec wins.** Anything written in a spec's `mark: {…}` object
shadows the theme token of the same name; a theme can only supply the
default for a property the spec does not state. Encoding channels and
conditions resolve after all four and shadow the lot, since they vary
per row.

`theme.MarkStyle` and `spec.MarkDef` share field names by design
where they mean the same thing. The overlap is:

| Property | In theme `mark` / `marks.<type>` | In spec `mark_def` |
|---|---|---|
| `fill`, `stroke`, `stroke_width`, `stroke_dash` | ✅ | ✅ |
| `opacity`, `fill_opacity` | ✅ | ✅ |
| `corner_radius`, `size`, `shape` | ✅ | ✅ |
| `font_size`, `font_weight`, `font_style` | ✅ | ✅ |
| `align`, `baseline` | ✅ | ✅ |
| `line_height`, `letter_spacing`, `filter` | ✅ | — theme-only |
| `stroke_opacity`, `font` (family) | — spec-only | ✅ |
| `dx`, `dy`, `angle`, `pad_angle`, radii | — spec-only | ✅ |

So a theme can set a house `fill_opacity` for every `area` mark and an
individual chart can still override it:

```json
{
  "mark": {"type": "area", "fill_opacity": 0.9},
  "encoding": {"x": {"field": "t", "type": "temporal"},
               "y": {"field": "v", "type": "quantitative"}}
}
```

renders at `fill_opacity: 0.9` even under a theme declaring
`"marks": {"area": {"fill_opacity": 0.3}}`, while that theme's
`stroke` and `corner_radius` (which the spec does not mention) still
apply.

### Per-axis blocks

`axis` states house style for *both* cartesian axes at once. When the
two need to differ, `axis_x` and `axis_y` layer over it — carrying the
same token set, and overriding **per property**:

```json
{
  "axis":   { "grid_color": "#e5e7eb", "tick_color": "#6b7280" },
  "axis_x": { "grid_color": "#f3f4f6" }
}
```

The x axis draws a `#f3f4f6` grid and keeps the `#6b7280` tick colour
it inherited; the y axis takes both values from `axis` unchanged. A
block that sets one token does not reset the rest — nothing is
replaced wholesale.

**Grid orientation.** An x axis emits the **vertical** grid lines and a
y axis the **horizontal** ones, so `axis_x.grid_color` /
`axis_y.grid_color` (and `grid_width`, `grid_dash`, `grid_opacity`)
are how the two orientations are coloured apart — a common request that
`axis` alone cannot express. A frequent pattern is horizontal rules
only:

```json
{
  "axis_x": { "grid_color": "transparent" },
  "axis_y": { "grid_color": "#e5e7eb", "grid_dash": [2, 2] }
}
```

`x2` and `y2` resolve to the same block as their base channel — a span
channel never carries an axis of its own.

#### How it is applied

Most of these tokens ride the CSS cascade: `theme/css.go` emits the
per-axis block as custom-property declarations scoped to that axis's
own group, and the renderer already wraps each axis (its grid lines
included) in `<g class="prism-axis prism-axis-x">`:

```css
:root{--prism-grid-color:#e5e7eb;}
.prism-axis-x{--prism-grid-color:#f3f4f6;}
```

Custom properties inherit, so the scoped declaration shadows the
`:root` one for that axis alone. The per-property merge *is* the
cascade — nothing is pre-folded, and the same post-hoc restyling that
works on `--prism-grid-color` works on the scoped variable.

Tokens that move SVG coordinates (`tick_size`, `label_padding`,
`title_padding`) or that are emitted as SVG attributes rather than CSS
(`label_line_height`, `label_letter_spacing`, `title_line_height`,
`title_letter_spacing`) cannot travel that way — a CSS variable cannot
move a line endpoint or a text coordinate. Those ride the Scene IR instead, on
`scene.Theme.axis_x` / `axis_y`, and the renderer falls back to the
shared `axis` value property by property. `filter` lands on the axis's
own group and composes with the shared block's filter on the enclosing
`prism-axes` group.

A theme that declares neither block emits exactly the bytes it did
before the blocks existed: this is purely additive.

### Axis geometry precedence

Three `axis` tokens are **geometry**, not appearance: `tick_size` (the
major tick length), `label_padding` (the gap between the axis line and
its tick labels) and `title_padding` (the gap between those labels and
the axis title). They move SVG coordinates, so unlike the colour and
font tokens they cannot be re-styled after the fact by overriding a
CSS variable — the variables `--prism-axis-tick-size`,
`--prism-axis-label-padding` and `--prism-axis-title-padding` are
emitted for reference, but the geometry itself is resolved before the
SVG is written.

All three names also exist on a position channel's `axis` block (see
[Encoding: axis components and geometry](encoding.md#axis-components-and-geometry)).
Where they overlap, the resolution order is fixed, highest first:

1. **The spec's `axis` block** — `"axis": {"tick_size": 12}` on the
   channel.
2. **The theme's `axis_x` / `axis_y` block** — the per-axis override
   (see [Per-axis blocks](#per-axis-blocks)).
3. **The theme's `axis` block** — `"axis": {"tick_size": 9}`.
4. **Prism's built-in metric** — 5 px tick, 4 px label padding, 8 px
   title padding.

This is the one precedence chain every axis token follows, geometry or
not; only the machinery differs (the CSS cascade for colour and font
tokens, `resolveAxisMetric` in the SVG renderer for geometry). Each
level shadows the previous **per property**.

**The spec wins.** A theme can set the house tick length for every
chart, and an individual chart still overrides it per channel; the
theme value applies only where the `axis` block says nothing. This
matches [mark style precedence](#mark-style-precedence): the theme
supplies defaults, the spec states intent.

```json
{ "axis": { "tick_size": 9, "label_padding": 10 } }
```

under a spec whose x channel carries `"axis": {"tick_size": 12}`
renders a 12 px x tick (spec) with a 10 px label gap (theme, since the
spec left `label_padding` alone), while the y axis — which states
nothing — takes both theme values.

**`title_padding` is measured from the tick labels, not from the plot
edge**, and it is added to a fixed per-side text allowance rather than
replacing it, so the 8 px every built-in theme states reproduces the
title position Prism has always drawn. Note that a padding larger than
the default draws into the outer margin: the layout reserves a fixed
depth per side and does not grow it to follow a token, the same
text-metric deferral `tick_size` and `label_padding` are subject to.

Minor ticks are not separately tokenised; they render at 0.6× whatever
major tick size resolves, so the default 5 px still yields a 3 px minor
tick. The remaining axis knobs — `labels`, `ticks`, `domain`,
`label_limit` and `zindex` — are spec-only: they decide *whether* a
component is drawn and in what order, which is an authoring decision
rather than a house style, so no theme token shadows them.

### Typography tokens

`line_height` and `letter_spacing` are optional pointer-typed typography
tokens (same "absent means inherit" semantics as `font_size` and every
other sparse numeric token). The SVG renderer (and `render/html`, which
delegates to it) applies `letter_spacing` as a `letter-spacing`
presentation attribute and `line_height` as a `style="line-height:…"`
declaration directly on the resolved title / axis-label / axis-title /
legend-label / legend-title / text-mark `<text>` element — these are
per-element, conditional attributes (nothing is emitted when a token
is left unset), not the CSS-variable + fixed-class mechanism used for
`font_size`/`font_weight`/color tokens. `line_height` only has a
visible effect on multi-line text; Prism does not yet wrap title,
axis-label, or text-mark content onto multiple lines, so today the
property is present in the markup (ready for any future wrapping) but
inert for every element except where content already spans multiple
`<tspan>`s.

| Block | Fields carrying the tokens |
|---|---|
| `title` (`TitleStyle`) | `line_height`, `letter_spacing` — applies to the chart title text. |
| `axis` (`AxisStyle`) | `label_line_height`/`label_letter_spacing` (tick labels) and `title_line_height`/`title_letter_spacing` (axis title), mirroring the existing `label_font_size`/`title_font_size` split. |
| `legend` (`LegendStyle`) | `label_line_height`/`label_letter_spacing` (entry labels) and `title_line_height`/`title_letter_spacing` (legend title), mirroring the existing `label_font_size`/`title_font_size` split. |
| `mark`/`marks.text`/`style.<name>` (`MarkStyle`) | `line_height`, `letter_spacing` — applies to `text`-mark content. |

```json
{
  "mark": { "font_size": 12, "line_height": 1.3, "letter_spacing": 0.2 },
  "axis": {
    "label_font_size": 11, "label_line_height": 1.2, "label_letter_spacing": 0.1,
    "title_font_size": 12, "title_line_height": 1.2, "title_letter_spacing": 0.1
  }
}
```

No `@font-face` / font-loading support is implied or added by these
tokens — Prism only sets the CSS typography properties on already
font-resolved text elements.

### Raw CSS and filter escape hatch

`filters` and `raw_css`, plus the `filter` field on `mark` / `marks.<type>`
/ `style.<name>` / `axis` / `legend` / `title` / `view`, are an escape
hatch for visual effects Prism's typed tokens don't model directly
(drop shadows, blurs, hover states beyond `states`, arbitrary
selectors). A `filter` value must name a key present in the theme's
`filters` map — an unresolved reference **fails loudly** at theme load
(`PRISM_THEME_FILTER_UNKNOWN`) rather than silently rendering without
the effect, an intentional departure from `range.*`'s scheme-name
fallback behavior.

**Trust boundary:** theme JSON — including `raw_css` and every
`filters` body — is developer-authored and trusted the same as spec
JSON is today. Prism does not sanitize or sandbox this content before
it lands in the rendered `<style>`/`<filter>` markup. Never route
untrusted or attacker-influenced theme JSON (e.g. end-user-supplied
theme files in a multi-tenant service) through `theme.LoadFile` /
`theme.LoadBytes` / a spec's inline `theme.raw_css` / `theme.filters`
override.

**Rendering:** the SVG backend emits one `<filter id="prism-filter-<name>">`
element per entry in `filters`, wrapping the raw body verbatim inside
a single top-level `<defs>` block. Any style block whose resolved
`filter` names an entry gets `filter="url(#prism-filter-<name>)"` on
the corresponding element — the mark itself, the `<g class="prism-axes">`
wrapper, the `<g class="prism-legends">` wrapper, the title `<text>`,
and (only when `view.filter` is set) a `<rect class="prism-view">`
background rect sized to the chart frame. `raw_css` is appended
verbatim inside the `<style>` block, after the generated
`:root{--prism-*}` variable manifest and fixed class selectors.
`render/html/` inherits both automatically — it wraps `render/svg`'s
own emitters and splices the resulting bytes verbatim, so no separate
glue was needed. The Canvas backend does not implement this escape
hatch.

### Gradients and patterns

`gradients` and `patterns` declare named fill definitions on the
theme. A `mark.fill`/`stroke`, `marks.<type>.fill`/`stroke`, or
`view.background` value written as `url(#name)` — the same convention
native SVG `fill`/`stroke` use — resolves against these registries:
`theme.gradients` is checked first, then `theme.patterns`. Any other
value (a hex color, a CSS color keyword, `"transparent"`, …) is
unaffected and keeps resolving as a plain literal color exactly as
before. A resolved reference renders as an actual
`<linearGradient>`/`<radialGradient>`/`<pattern>` def, with the
resolved attribute rewritten to `fill="url(#prism-gradient-<name>)"`
or `fill="url(#prism-pattern-<name>)"` (same for `stroke`, and for the
view background rect). A `url(#name)` value that doesn't name a
registered gradient or pattern fails loud at theme load instead of
silently rendering nothing.

Note the per-type `marks.<type>` block always outranks the global
`mark` block for any field it sets (see [Theme
structure](#theme-structure)) — every built-in theme ships a
per-type default `fill` for common marks like `bar`, so a `url(#name)`
fill usually needs to go on `marks.bar.fill` (etc.) rather than the
global `mark.fill` to actually take effect.

A `GradientDef` is either `"linear"` (oriented by `angle`, in
degrees, 0 = left-to-right, clockwise) or `"radial"` (centered at
`cx`/`cy` — fractions of the shape's bounding box, default 0.5 each —
with a `radius` fraction). Every gradient needs at least two `stops`,
each an `{ "offset": 0-1, "color": "..." }` pair:

```json
{
  "gradients": {
    "brand_fade": {
      "type": "linear",
      "angle": 90,
      "stops": [
        { "offset": 0, "color": "#4c78a8" },
        { "offset": 1, "color": "#f58518" }
      ]
    }
  }
}
```

A `PatternDef` is either a built-in catalogue entry — `type` set to
one of `diagonal-stripes`, `dots`, `cross-hatch`, `grid`, tuned via
`color`, `spacing`, and `size` — or a bespoke pattern supplied as raw
SVG through `content` (the inner markup of the `<pattern>` element,
verbatim). Exactly one of `type` or `content` must be set. `spacing`
and `size` default to `8` and `4` (user-space pixels — pattern tiles
use `patternUnits="userSpaceOnUse"`, so they stay a fixed physical
size regardless of the shape they fill) and `color` defaults to
`#000000` for built-in types when unset:

- `diagonal-stripes` — a solid stripe of width `size` per tile
  (pitch `spacing`), tile rotated 45°.
- `dots` — one centered dot of diameter `size` per tile (pitch
  `spacing`).
- `cross-hatch` — an X across the tile (`size` stroke width, tile
  size `spacing`).
- `grid` — a lattice of `size`-wide lines at `spacing` pitch.

```json
{
  "patterns": {
    "hatch":       { "type": "cross-hatch", "color": "#6b7280", "spacing": 6, "size": 1 },
    "custom_dots": { "content": "<circle cx=\"2\" cy=\"2\" r=\"1\" fill=\"#4c78a8\"/>" }
  }
}
```

**Validation:** both maps are checked structurally at theme load
(`Register`, `LoadFile`/`LoadBytes`), the same fail-loud entry points
as the filter escape hatch. A gradient with an unrecognized `type`,
fewer than 2 `stops`, an out-of-range `offset`, or an empty stop
`color` fails with `PRISM_THEME_GRADIENT_INVALID`. A pattern that sets
both `type` and `content` (or neither), names a `type` outside the
built-in catalogue, or sets a non-positive `spacing`/`size` fails with
`PRISM_THEME_PATTERN_INVALID`. On top of that, every `fill`/`stroke`
on `mark`, `marks.<type>`, and `style.<name>`, plus `background` on
`view`, is checked for the `url(#name)` form; a reference that names
neither a `gradients` nor a `patterns` entry fails with
`PRISM_THEME_FILL_REF_UNKNOWN` (mirroring `PRISM_THEME_FILTER_UNKNOWN`
for the filter escape hatch).

**Trust boundary:** `content` on a `PatternDef` is the same trust
tier as `filters`/`raw_css` — developer-authored SVG that Prism does
not sanitize. Never route untrusted theme JSON through it.

**Rendering:** the SVG backend emits one `<linearGradient>`/
`<radialGradient id="prism-gradient-<name>">` element per entry in
`gradients` and one `<pattern id="prism-pattern-<name>">` element per
entry in `patterns`, inside the same top-level `<defs>` block the
filter escape hatch uses. A resolved `fill`/`stroke`/`background`
gets rewritten to `url(#prism-gradient-<name>)` / `url(#prism-pattern-<name>)`
on the corresponding element — the mark itself, or (only when
`view.background` resolves) a `<rect class="prism-view">` background
rect sized to the chart frame. `render/html/` inherits this
automatically, the same as the filter escape hatch. The Canvas
backend does not implement this escape hatch.

### Category styles

`category_styles` is a theme-level, reusable data-driven style map so
a chart author doesn't have to repeat the same per-value styling as a
spec-level [`condition`](encoding.md) block in every spec that
encodes a given field. The shape is a nested map — outer key is a
field name, inner key is the field's *stringified* value, leaf is a
full `MarkStyle` (not just a color, unlike [`range`](#theme-structure),
which is color-only and keyed by scale role rather than an actual data
value):

```json
{
  "category_styles": {
    "Origin": {
      "USA":    { "fill": "#4c78a8" },
      "Europe": { "fill": "#f58518" },
      "Japan":  { "fill": "#e45756" }
    },
    "Status": {
      "at_risk": { "fill": "#e45756", "stroke": "#7f1d1d", "stroke_width": 2 }
    }
  }
}
```

A nested `field → value → style` map is used instead of a flat
`"field=value"` string key on purpose — it avoids inventing a
mini string grammar to parse, consistent with the project's
no-expression-language stance (`filter`/`calculate`/condition `test`
are all structured JSON built-ins; see
[Spec format](spec.md#transforms)).

**Precedence (a spec's own condition wins):** once a chart encodes a
field that has a matching `category_styles` entry, the theme-level
style applies automatically as a default layer, with any spec-level
`condition` on the same channel — targeting the same field/value — 
winning over it if both apply (explicit beats theme default). This
mirrors the general cascade order elsewhere in `theme/` (a more
specific block always outranks a more general one for any field it
sets).

**Applied at encode time.** For every channel bound to a field
(`encode.categoryStyleFieldsAt` walks the same channel set
`encode/encode_condition.go` does — position channels plus
color/fill/stroke/opacity/size/shape), the encoder looks up each
datum's value for that field in `category_styles[field]` and, on a
match, merges the resolved `MarkStyle` onto the mark's already
-resolved style via `theme.MergeMarkStyle` (`encode/encode_category_styles.go`,
function `applyCategoryStyles`). Only the fields the theme author set
on that entry move — an entry that sets only `stroke` leaves whatever
fill the mark already had untouched. Data whose field value has no
matching entry renders with the base/default style, unchanged.
`applyCategoryStyles` always runs immediately before
`encode.applyConditions` in the pipeline, so a spec-level `condition`
targeting the same field/value is applied afterward and overwrites
whichever attrs it resolves — giving the condition precedence exactly
as designed.

### Dark variant pairing

`dark_variant` names a registered counterpart theme:

```json
{
  "name": "brand_light",
  "base": "light",
  "dark_variant": "brand_dark"
}
```

Setting `dark_variant` alone is the opt-in for automatic light/dark
rendering — there is no separate flag. A theme that declares one
signals the renderer to embed **both** palettes in a single SVG/HTML
output and switch between them at view time via
`prefers-color-scheme`, without a second `plot`/`render` call.

**Chrome dark-swap (live):** `CSSVariables()` emits the base
`:root{...}` block as before, then — only when `dark_variant`
resolves — a second rule appended inside the same `<style>` element:

```css
@media (prefers-color-scheme: dark) {
  :root {
    --prism-color-axis: #9ca3af;
    --prism-axis-domain-color: #9ca3af;
    /* ...legend/title/view/selection-state tokens... */
  }
}
```

This covers every token that already resolved through `var()`
end-to-end before this feature existed: axis strokes/ticks, grid
lines, title text, legend text, view background/stroke, and
`.prism-selected`/`.prism-deselected` opacity — computed by running
the same `theme/css.go` emission helpers against the paired theme.
The fixed class selectors (`.prism-axis-domain`, `.prism-grid-line`,
…) are not duplicated inside the media query — they already read
through `var()`, so only the custom-property *values* need to swap.
When `dark_variant` is unset (or names a theme that fails to
resolve), no media query is emitted and output is byte-identical to a
theme with no `dark_variant` at all.

**Mark colors dark-swap too (E4-S3).** When `dark_variant` resolves,
`encode/encode.go` resolves every mark color — both scale-driven
(categorical/sequential palette lookups in `encode/palette.go`) and
static per-mark-type theme colors (`theme.MarkStyle.Fill`/`Stroke` via
`applyThemeMarkStyle`) — against **both** the active theme and its
`dark_variant` counterpart. Each distinct `(light, dark)` pair
encountered gets a stable variable name in first-encounter order:
`--prism-resolved-0`, `--prism-resolved-1`, … (same input spec + theme
pairing → same names every render, so golden fixtures stay stable).
Both values land in the `<style>` block — light under the base
`:root{...}`, dark under the `@media` rule above — and the mark
element emits `fill="var(--prism-resolved-N)"` /
`stroke="var(--prism-resolved-N)"` instead of a baked hex literal:

```css
:root { --prism-resolved-0: #4c78a8; }
@media (prefers-color-scheme: dark) {
  :root { --prism-resolved-0: #4269d0; }
}
```

```xml
<rect class="prism-mark-bar" fill="var(--prism-resolved-0)" .../>
```

This is carried on the scene IR by `scene.Style.FillVar`/`StrokeVar`
(a CSS custom-property name, additive alongside the existing
`Fill`/`Stroke`/`FillRef`/`StrokeRef` fields — same "optional ref wins
over the baked value" precedent as the gradient/pattern `FillRef`).
`render/html/` inherits this automatically, same as every other
`render/svg` emission. When `dark_variant` is unset, none of this
runs: every mark keeps baking a literal hex exactly as before E4-S3.
Legend swatches are not yet re-plumbed onto resolved vars — they still
bake the light-theme hex, a known gap for a future story.

**Validation:** a non-empty `dark_variant` must name a theme already
present in the registry — checked at the same fail-loud entry points
as the filter/gradient/pattern escape hatches (`Register`,
`LoadFile`/`LoadBytes`). An unresolved name fails with
`PRISM_THEME_DARK_VARIANT_UNKNOWN` rather than silently rendering
without a dark counterpart. Because validation checks the registry
*as of registration time*, pairing only works in one direction per
`Register` call: the counterpart named by `dark_variant` must already
be registered (built-in themes register in a fixed order at package
`init()` — see `theme/registry.go`). None of Prism's built-in themes
(`light`, `dark`, `print`, `high_contrast`, `colorblind`) set
`dark_variant` on each other in this story; a future story that wants
to pair built-ins together needs either a two-pass registration (register
all themes first, then a second pass that only sets `dark_variant` and
re-validates) or an explicit `PairThemes(a, b string) error` helper —
not yet implemented.

## Color schemes

Prism ships the d3-scale-chromatic catalogue plus four
accessibility-focused additions. Reference any scheme by name in
`scale.scheme` or `theme.range.*.scheme`.

### Categorical
`category10`, `tableau10`, `observable10`, `accent`, `dark2`,
`paired`, `pastel1`, `pastel2`, `set1`, `set2`, `set3`,
`okabe_ito`, `tol_bright`, `tol_vibrant`, `tol_muted`.

### Sequential (single-hue)
`blues`, `greens`, `greys`, `oranges`, `purples`, `reds`.

### Sequential (multi-hue)
`bugn`, `bupu`, `gnbu`, `orrd`, `pubu`, `pubugn`, `purd`, `rdpu`,
`ylgn`, `ylgnbu`, `ylorbr`, `ylorrd`.

### Sequential (perceptually uniform)
`viridis`, `magma`, `plasma`, `inferno`, `cividis`, `turbo`,
`warm`, `cool`.

### Diverging (Brewer 9-class)
`rdbu`, `rdylbu`, `brbg`, `prgn`, `piyg`, `puor`, `rdgy`, `rdylgn`,
`spectral`.

### Cyclic
`rainbow`, `sinebow`.

### Accessibility note
The four Prism extensions — `okabe_ito`, `tol_bright`,
`tol_vibrant`, `tol_muted` — are colorblind-safe palettes from
peer-reviewed sources (Wong 2011, Tol 2018). The default
`colorblind` theme uses `okabe_ito` for categorical channels and
`cividis` for continuous channels.

### Where a scheme sits in the cascade

A named scheme is the **second** tier of palette resolution, not the
first. The full order, highest first:

1. `scale.range` on the channel — an inline color list supplied by the
   spec author. Nothing overrides it.
2. `scale.scheme` on the channel — a name from the catalogue above, or
   from the theme's own `schemes` registry (which shadows the global
   catalogue for that name).
3. The theme's `range` slot for the role — `range.category` for a
   discrete channel, `range.ramp` then `range.heatmap` for a
   continuous one.
4. The theme's legacy flat `color_scheme_categorical` /
   `color_scheme_sequential` field.
5. Prism's built-in default palette (a category10 derivative), or a
   9-stop `blues` ramp on the continuous side.

An unknown scheme name degrades quietly to the next tier so a
malformed spec still renders; validate reports it separately as
`PRISM_SPEC_028`. An inline `range` is the one tier that does not
degrade as a unit — unparseable entries are dropped and the rest still
win, and only a wholly unparseable range falls through.

### Traversing a ramp — `scale.interpolate`

A continuous ramp — whether it came from a scheme above or from an
inline `scale.range` — is traversed in the colorspace
`scale.interpolate` names: `rgb` (default), `hsl`, or `lab`.
Vega-Lite's `hcl` is not supported. `lab` is the one to reach for when
a ramp's steps need to read as evenly spaced rather than merely be
evenly spaced in sRGB.

Prism resamples the ramp at encode time, so the stops that reach a
renderer already trace the chosen space's curve. See
[Choosing the colors](./encoding.md#choosing-the-colors--range-scheme-interpolate)
in the encoding reference for the full treatment.

## Sparse override at spec level

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "theme": {
    "name": "light",
    "marks": {
      "bar": { "fill": "#2563eb", "corner_radius": 4 }
    },
    "range": {
      "category": { "scheme": "okabe_ito" }
    },
    "axis_y": { "grid_dash": [2, 2] }
  }
}
```

Every block the theme struct carries is available here, `axis_x` /
`axis_y` included — each merges against its own counterpart in the
base theme, never against the shared `axis` block.

Spec-level overrides merge over the named base theme without
restating the whole struct. Order of precedence:

```
hardcoded fallback
  ← theme.Mark
  ← theme.Marks[type]
  ← spec.theme overrides
  ← spec.mark.<field> (explicit per-spec override)
  ← per-row encoding
```

## Custom theme via JSON

```
prism plot bar.json --theme=./brand.theme.json > bar.svg
```

A theme JSON file is just a `theme.Theme` document with an optional
`base` field. When `base` names a registered theme, the file's
fields merge sparsely on top:

```json
{
  "name": "brand",
  "base": "light",
  "marks": {
    "bar": { "fill": "#001eff", "corner_radius": 6 }
  },
  "schemes": {
    "brand": ["#001eff", "#33ffaa", "#ff3366"]
  },
  "range": {
    "category": { "scheme": "brand" }
  }
}
```

## CSS variables emitted

Every SVG (and live web component shadow root) carries a `<style>`
block declaring `--prism-*` variables for every set token. Override
at runtime via DOM style assignment to live-switch theme aspects
without re-rendering.

```
--prism-color-axis        --prism-color-grid     --prism-color-text
--prism-color-bg          --prism-font-sans      --prism-font-mono

--prism-axis-domain-color --prism-axis-tick-size --prism-axis-label-color
--prism-axis-label-padding --prism-axis-title-padding
--prism-grid-color        --prism-grid-width     --prism-grid-dash

--prism-mark-fill         --prism-mark-bar-fill  --prism-mark-line-stroke
--prism-mark-bar-corner-radius --prism-mark-point-size

--prism-legend-padding    --prism-legend-symbol-size --prism-title-font-size
--prism-view-bg           --prism-view-padding

--prism-selected-opacity  --prism-deselected-opacity
```

The full set scales with the tokens the active theme defines —
unset tokens omit the variable so renderers fall back to hard-coded
defaults inside the CSS class declarations.

When the active theme sets `dark_variant` (see
[Dark variant pairing](#dark-variant-pairing)), the chrome-related
subset of these variables (axis/grid/legend/title/view/selection-state
— not the static `--prism-mark-*` defaults family) is emitted a second
time, inside an `@media (prefers-color-scheme: dark) { :root { ... } }`
rule appended after the base block in the same `<style>` element.
Resolved per-instance mark colors ride a separate
`--prism-resolved-N` family (E4-S3, see
[Dark variant pairing](#dark-variant-pairing)) that IS doubled the
same way — light value in the base block, dark value in the media
rule — since those are the actual colors marks paint with once
auto-dark is active.

`line_height` and `letter_spacing` (see [Typography tokens](#typography-tokens))
are the one exception: they render as direct per-element attributes
rather than `--prism-*` custom properties, so they are baked in at
render time and are not runtime-overridable via DOM style assignment
the way the tokens above are.

`--prism-axis-tick-size`, `--prism-axis-label-padding` and
`--prism-axis-title-padding` are emitted, but they are **reference
only**: a CSS variable cannot move an SVG line endpoint or a `<text>`
coordinate, so the geometry those three tokens describe is resolved at
render time (see
[Axis geometry precedence](#axis-geometry-precedence)). Overriding them
in the DOM restyles nothing.

A theme's `axis_x` / `axis_y` blocks (see
[Per-axis blocks](#per-axis-blocks)) emit the same `--prism-axis-*` /
`--prism-grid-*` names a second time, scoped to
`.prism-axis-x { ... }` / `.prism-axis-y { ... }` rather than `:root`.
Custom properties inherit, so the scoped declaration wins for that
axis's group and leaves the other axis on the `:root` value. Both are
runtime-overridable the same way — assigning to the group element's
style shadows the theme's scoped value.

## Rendering backends

Three backends consume the same `scene.SceneDoc` + theme tokens:

| Backend | Package | MIME type | Notes |
|---|---|---|---|
| `svg` (default) | `render/svg` | `image/svg+xml` | Canonical vector output; the `<style>` block above is emitted inline. |
| `html` | `render/html` | `text/html` | Wraps `render/svg`'s own output in a small standalone HTML document (`<!doctype html><html>…<div class="prism-html-chart"><svg>…</svg></div>…`) — the embedded SVG carries the identical theme `<style>` block, so a theme picked at plot time (`--theme=dark`, etc.) looks the same in either backend. |
| `canvas` | vendored ESM (`static/`) | n/a (DOM) | Browser-only web component bridge; not a Go backend. |

Select the backend the same way across every surface: `prism plot
--format html` (CLI), `format: "html"` on the Twirp `Plot` RPC / the
`prism_plot` MCP tool, or `opts.Format = "html"` passed to
`prism.RenderPlan` (library). An unsupported or misspelled format
returns `PRISM_RENDER_FORMAT_UNAVAILABLE` from all three surfaces.

A mark can also be unsupported by a specific backend rather than the
format itself being unavailable — the SVG backend rejects a top-level
`table` mark (DOM/CSS-driven, no SVG geometry equivalent) with
`PRISM_RENDER_MARK_UNSUPPORTED`, naming the mark and pointing at the
`html` backend instead. See [Marks](marks.md#renderer-compatibility).

## Worked examples

- [bar_light](../gallery/themes/bar_light.prism.json)
- [bar_dark](../gallery/themes/bar_dark.prism.json)
- [bar_print](../gallery/themes/bar_print.prism.json)
- [bar_high_contrast](../gallery/themes/bar_high_contrast.prism.json)
- [bar_colorblind](../gallery/themes/bar_colorblind.prism.json)
- [bar_filter](../gallery/themes/bar_filter.prism.json) — drop-shadow filter on the mark
- [bar_gradient](../gallery/themes/bar_gradient.prism.json) — `url(#name)` linear gradient fill on a bar mark
- [bar_pattern](../gallery/themes/bar_pattern.prism.json) — `url(#name)` built-in `diagonal-stripes` pattern fill on a bar mark
- [bar_dark_variant](../gallery/themes/bar_dark_variant.prism.json) — `theme: {"dark_variant": "dark"}`, doubled chrome + mark-color CSS in one SVG (see [Dark variant pairing](#dark-variant-pairing))
- [bar_category_styles](../gallery/themes/bar_category_styles.prism.json) — `theme.category_styles` colors each bar by its `quarter` value, with no spec-level `condition` block (see [Category styles](#category-styles))
