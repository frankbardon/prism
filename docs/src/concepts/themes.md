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
| `print` | Reports, PDF output. Grayscale only, no transparency on lines, hatch-friendly. |
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
| `axis`     | Axis domain, ticks, grid, labels, titles. |
| `legend`   | Legend fills, symbols, labels, padding. |
| `title`    | Chart title typography. |
| `view`     | Chart-rect background, stroke, padding. |
| `range`    | Default color scheme per scale role (category, ordinal, ramp, heatmap, diverging, symbol, cyclic). |
| `schemes`  | Per-theme custom named-scheme registry. Entries shadow the global catalogue. |
| `style`    | Named-style registry — marks reference an entry via their `style` attr. |
| `states`   | State overlays (selected, deselected, hover, focus). Materialise as `.prism-<state>` CSS classes. |

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
--prism-color-bg          --prism-font-sans      --prism-font-mono

--prism-axis-domain-color --prism-axis-tick-size --prism-axis-label-color
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

## Worked examples

- [bar_light](../gallery/themes/bar_light.prism.json)
- [bar_dark](../gallery/themes/bar_dark.prism.json)
- [bar_print](../gallery/themes/bar_print.prism.json)
- [bar_high_contrast](../gallery/themes/bar_high_contrast.prism.json)
- [bar_colorblind](../gallery/themes/bar_colorblind.prism.json)
