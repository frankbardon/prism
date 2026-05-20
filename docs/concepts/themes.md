# Themes

Themes drive colors + fonts + spacing across all renderers. Single Go
struct as the source of truth; emitted as CSS variables for the SVG
and browser ports.

## Bundled themes

| Name | When to use |
|---|---|
| `light` (default) | Standard web pages, light backgrounds. |
| `dark` | Dark dashboards, terminal embeds. |
| `print` | Reports, PDF output. Black ink, no gradients, no transparency. |

## Pick at plot time

```
prism plot bar.json --theme=dark > bar-dark.svg
```

## Sparse override at spec level

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "theme": {
    "name": "light",
    "overrides": {
      "axis_color": "#1f2937",
      "color_scheme_categorical": ["#2563eb", "#10b981", "#f97316"]
    }
  },
  ...
}
```

Spec-level overrides merge onto the named theme without redefining
the whole struct.

## Custom theme via JSON

```
prism plot bar.json --theme=./brand.theme.json > bar.svg
```

Theme JSON shape:

```json
{
  "name": "brand",
  "extends": "light",
  "overrides": {
    "axis_color": "#0f172a",
    "font_sans": "Source Sans 3, sans-serif",
    "color_scheme_categorical": ["#0ea5e9", "#a855f7", "#22c55e", "#f43f5e"]
  }
}
```

## CSS variables emitted

Every SVG (and live web component shadow root) carries a `<style>`
block with these variables:

```
--prism-color-axis     --prism-color-grid     --prism-color-text
--prism-color-bg       --prism-color-selected
--prism-font-sans      --prism-font-mono
--prism-font-size-label  --prism-font-size-title  --prism-font-size-axis-title
```

Override at runtime via DOM style assignment to live-switch themes
without re-render.

## Worked examples

- [bar_light](../gallery/themes/bar_light.prism.json)
- [bar_dark](../gallery/themes/bar_dark.prism.json)
- [bar_print](../gallery/themes/bar_print.prism.json)
