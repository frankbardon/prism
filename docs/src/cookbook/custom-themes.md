# Cookbook: custom themes

Brand a chart with company colors + fonts without touching code.

## Theme JSON

```json
{
  "name": "brand",
  "extends": "light",
  "overrides": {
    "axis_color": "#0f172a",
    "text_color": "#1e293b",
    "text_muted_color": "#64748b",
    "grid_color": "#e2e8f0",
    "font_sans": "Source Sans 3, system-ui, sans-serif",
    "color_scheme_categorical": [
      "#0ea5e9",
      "#a855f7",
      "#22c55e",
      "#f43f5e",
      "#fb923c"
    ]
  }
}
```

Save as `brand.theme.json`.

## Use it

```
prism plot bar.json --theme=./brand.theme.json > bar.svg
```

## Sparse override at spec level

If only one chart needs a tweak, override inline:

```json
{
  "theme": {
    "name": "light",
    "overrides": {
      "color_scheme_categorical": ["#0ea5e9", "#22c55e"]
    }
  },
  ...
}
```

## Beyond colour

Chart proportions are tokens too, so a house style can set them once
rather than per chart. The blocks that matter most:

```json
{
  "name": "brand",
  "extends": "light",
  "overrides": {
    "text_muted_color": "#64748b",
    "axis": {
      "band_padding": 0.36,
      "band_max_width": 72,
      "grid_color": "#f1f5f9",
      "label_padding": 10,
      "zero_color": "#334155"
    },
    "legend": {
      "gap": 24,
      "row_height": 22,
      "symbol_extent": 12,
      "symbol_corner_radius": 3
    },
    "marks": {
      "bar":   {"corner_radius": 4},
      "line":  {"stroke_width": 2.5},
      "point": {"size": 120}
    },
    "title": {"anchor": "middle", "font_size": 17}
  }
}
```

Raising a font size widens the margin that holds it — the layout
measures the labels it is about to draw, so nothing overflows and
nothing is left over. See
[Themes → Layout tokens](../concepts/themes.md#layout-tokens) for the
full list.

## Notes

- Set `text_color` and `text_muted_color` together. Setting only the
  first flattens the hierarchy between a chart's data and its
  scaffolding, which is the difference the muted token exists to make.
- All bundled themes live in `theme/` and emit identical CSS variable
  manifests for the SVG + browser ports.
- Browser theme switching is one DOM attribute away:
  ```js
  document.querySelector("prism-chart").setAttribute("theme", "dark");
  ```
  No re-render needed; CSS variables swap.
- A light-family theme also carries a dark companion, so a host can
  flip light/dark with a container class and no re-render at all — see
  [Themes → Dark mode](../concepts/themes.md#dark-mode). A custom theme
  that `extends` a light base inherits that behaviour; its own colour
  overrides land in the `:root` block and survive the flip only for the
  tokens it did not restate.
