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

## Notes

- All bundled themes (`light`, `dark`, `print`) live in `theme/` and
  emit identical CSS variable manifests for the SVG + browser ports.
- Browser theme switching is one DOM attribute away:
  ```js
  document.querySelector("prism-chart").setAttribute("theme", "dark");
  ```
  No re-render needed; CSS variables swap.
