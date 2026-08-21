# Themes

Themes drive colors, fonts, spacing, and per-mark defaults across
all renderers. A single Go struct (`theme.Theme`) is the source of
truth; resolved tokens emit as CSS variables that the SVG output
and the live browser component both consume.

## Bundled themes

| Name | When to use |
|---|---|
| `light` (default) | Standard web pages, light backgrounds. Prism categorical + Viridis sequential. Ships a dark companion (see [Dark mode](#dark-mode)). |
| `dark` | Dark dashboards, terminal embeds. Prism categorical lifted for a dark ground + Magma sequential. |
| `print` | Reports, print-ready output. Grayscale only, no transparency on lines, hatch-friendly. |
| `high_contrast` | Projector / presentation, low-vision readers. Pure black/white, bold weights, no grid lines. |
| `colorblind` | Colorblind-safe defaults. Okabe-Ito categorical + Cividis sequential (deuteranopia-tuned). |

Two further themes — `high_contrast_dark` and `colorblind_dark` —
are registered but are not meant to be selected directly. They exist
so their light counterparts can emit a dark companion token set; see
[Dark mode](#dark-mode).

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
    "title_color":   "#111827",
    "title_font_size": 12,
    "title_padding": 8
  },

  "legend": {
    "label_color":      "#111827",
    "title_font_weight":"600",
    "symbol_size":      64,
    "padding":          8
  },

  "title": {
    "color":      "#111827",
    "font_size":  16,
    "font_weight":"600",
    "anchor":     "start"
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
  }
}
```

### Block reference

| Block | Drives |
|---|---|
| `mark`     | Default style applied to every mark unless `marks.<type>` overrides. |
| `marks.<type>` | Per-mark-type defaults. Key matches the spec's `mark.type` (bar, line, area, point, rule, text, tick, rect, arc, geoshape, geopoint, ...). |
| `axis`     | Axis domain, ticks, grid, labels, titles, the zero baseline, and categorical band geometry. |
| `legend`   | Legend fills, symbols, labels, padding, row pitch, and the gap to the plot. |
| `title`    | Chart title typography. |
| `view`     | Chart-rect background, stroke, padding. |
| `range`    | Default color scheme per scale role (category, ordinal, ramp, heatmap, diverging, symbol, cyclic). |
| `schemes`  | Per-theme custom named-scheme registry. Entries shadow the global catalogue. |
| `style`    | Named-style registry — marks reference an entry via their `style` attr. |
| `states`   | State overlays (selected, deselected, hover, focus). Materialise as `.prism-<state>` CSS classes. |

### Layout tokens

Layout is measured, not fixed. Prism resolves the plot rectangle from
the labels it is about to draw — it renders the axes once against a
provisional rect, measures the resulting labels with
`encode.TextMetrics`, then places the real rect around them. Every
number it measures with is a token, so raising a label size widens the
margin that holds it rather than overflowing into the plot.

| Token | Default | Drives |
|---|---|---|
| `view.padding` | `12` | Outer inset kept clear on every side of the frame. |
| `axis.label_padding` | `8` | Gap between the plot edge (or tick) and a tick label. |
| `axis.title_padding` | `10` | Gap between the label column and the axis title. |
| `axis.tick_size` | `4` | Length of a tick mark, where one is drawn. |
| `axis.band_padding` | `0.28` | Fraction of each categorical step left empty between bars. Raise for lighter bars, lower for a dense dashboard. |
| `axis.band_max_width` | `96` | Cap on one band's pixel width. Stops a one-category chart drawing a single bar across the whole plot. `0` disables. |
| `axis.zero_color` / `axis.zero_width` | axis colour / `1.5` | The emphasised baseline drawn where a domain crosses zero. |
| `legend.gap` | `16` | Space between the plot's edge and the legend block. |
| `legend.row_height` | `18` | Pitch of one legend row. |
| `legend.symbol_extent` | `10` | Swatch side length. Distinct from `legend.symbol_size`, which is an *area* (Vega-Lite's convention, `64` = 8×8) and is not interchangeable. |
| `legend.symbol_corner_radius` | `2` | Rounds the swatch. Match it to `marks.bar.corner_radius` so the swatch reads as a sample of the mark. |
| `title.anchor` | `start` | `start` / `middle` / `end`. Honoured by the renderer. |

### Typography hierarchy

The chrome is meant to recede so the data comes forward, and it does
that through weight and colour rather than size:

| Element | Size | Weight | Colour token |
|---|---|---|---|
| Chart title | `title.font_size` (15) | 600 | `color` (`#0F172A`) |
| Legend label | `legend.label_font_size` (11) | 400 | `color` |
| Axis title | `axis.title_font_size` (11) | 500 | `color_muted` (`#64748B`) |
| Axis label | `axis.label_font_size` (11) | 400 | `color_muted` |
| Legend title | `legend.title_font_size` (11) | 500 | `color_muted` |

`color_muted` is the token that makes this work. Setting only `color`
leaves the chrome at full strength; set both, or set neither and
inherit the base's pair.

Numeric axis labels render with `font-variant-numeric: tabular-nums`
so a right-aligned tick column aligns on its digits.

## Dark mode

A chart rendered on a light-family base carries **both** token sets.
The `:root` block holds the light values; a companion block holds the
dark ones, and the host says which applies:

```html
<!-- light: nothing to do -->
<div>…chart…</div>

<!-- dark: the host sets the class, the chart repaints via CSS -->
<div class="prism-dark">…same chart bytes…</div>

<!-- follow the OS setting: opt in explicitly -->
<div class="prism-auto">…same chart bytes…</div>
```

No re-render and no round trip: a chart already streamed into a page
flips with everything around it.

`prefers-color-scheme` alone is **not** enough to flip a chart, and
that is on purpose. An SVG is inlined into whatever page embeds it, so
a light page viewed on a machine whose OS is set to dark would
otherwise render dark-theme charts on white — light grey labels,
near-invisible grid. The OS setting says nothing about the background
this particular chart landed on; the host does know, so the host says.
`prism-auto` is there for a host that genuinely wants to follow the OS.

Which bases carry a companion:

| Base | Companion | Why |
|---|---|---|
| `light` | `dark` | The common case. |
| `high_contrast` | `high_contrast_dark` | Inverted rather than muted — nothing in that base is a mid-tone. |
| `colorblind` | `colorblind_dark` | Same Okabe-Ito hues, lifted where one would vanish into near-black. |
| `dark` | — | Already an explicit choice; a host must not re-flip it. |
| `print` | — | Paper has no dark mode. |

Two rules the companion block obeys:

- **Colour only.** Geometry does not depend on the ground, so no
  measurement appears in the companion. This is a correctness
  requirement, not a size one: the companion wins over `:root` by
  specificity, so a geometry token in it would silently overwrite an
  organisation's own override on dark hosts only.
- **Differences only.** A token whose value is identical in both bases
  is written once.

Series colours are the exception that stays put. The encoder resolves
categorical colours to literal `fill` / `stroke` attributes, so they do
not flip — which is the right behaviour for brand colours, and why the
default palette is chosen to clear both grounds (≥ 2.4:1 against white
and against `#0F1620`).

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
    }
  }
}
```

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
--prism-color-text-muted  --prism-color-bg       --prism-font-sans
--prism-font-mono

--prism-axis-domain-color --prism-axis-tick-size --prism-axis-label-color
--prism-axis-title-color  --prism-axis-label-padding --prism-axis-title-padding
--prism-axis-zero-color   --prism-axis-zero-width
--prism-axis-band-padding --prism-axis-band-max-width
--prism-grid-color        --prism-grid-width     --prism-grid-dash

--prism-mark-fill         --prism-mark-bar-fill  --prism-mark-line-stroke
--prism-mark-bar-corner-radius --prism-mark-point-size
--prism-mark-area-fill-opacity --prism-mark-area-stroke

--prism-legend-padding    --prism-legend-symbol-size --prism-legend-symbol-extent
--prism-legend-symbol-corner-radius --prism-legend-gap --prism-legend-row-height
--prism-title-font-size   --prism-title-anchor
--prism-view-bg           --prism-view-padding

--prism-selected-opacity  --prism-deselected-opacity
```

The emitted block always begins `<style>:root{`, with exactly one
`:root` rule. Consumers that scope tokens per chart rewrite that one
selector; a second `:root` would leave half the tokens out of scope.

The full set scales with the tokens the active theme defines —
unset tokens omit the variable so renderers fall back to hard-coded
defaults inside the CSS class declarations.

## Worked examples

- [bar_light](../gallery/themes/bar_light.prism.json)
- [bar_dark](../gallery/themes/bar_dark.prism.json)
- [bar_print](../gallery/themes/bar_print.prism.json)
- [bar_high_contrast](../gallery/themes/bar_high_contrast.prism.json)
- [bar_colorblind](../gallery/themes/bar_colorblind.prism.json)
