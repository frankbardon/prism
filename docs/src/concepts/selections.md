# Selections

Selections drive interactive scene filtering. Two kinds (`point` and
`interval`), two reactive modes (`client` and `server`), one wire
protocol (`CustomEvent('prism:select')`).

## Declaring a selection

```json
{
  "selection": {
    "brush": {"type": "interval", "encodings": ["x"]},
    "click": {"type": "point", "encodings": ["color"]}
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "brand_id", "type": "nominal"},
    "y": {"field": "score", "type": "quantitative"},
    "color": {
      "condition": {"selection": "click", "field": "category"},
      "value": "#d1d5db"
    }
  }
}
```

## Point vs interval

| Kind | Trigger | State |
|---|---|---|
| `point` | Click on a mark | `{points: [{layerID, rowID}]}` |
| `interval` | Drag-brush on plot region | `{range: {channel, min, max}}` |

## Reactive modes

| Mode | Loop |
|---|---|
| `client` | Brush/click → DOM class toggle on marks. Zero network. |
| `server` | Brush/click → POST `/prism/scene` with synthesized filter → re-render. |
| `both` | Apply client immediately, server in background. |

## Cross-chart filtering

```html
<prism-coordinator>
  <prism-chart spec="overview.prism.json"></prism-chart>
  <prism-chart spec="detail.prism.json"></prism-chart>
</prism-coordinator>
```

Both charts declaring the same selection ID synchronize via the
coordinator. A brush on the overview filters the detail.

## URL state

Selection state round-trips through `window.location.hash` so
shareable links restore the brush:

```
https://your-app.example/dashboard#prism-sel:<base64>
```

Falls back to `localStorage` when the encoded state exceeds 1024
characters.

## Hit-test attributes

Every SVG mark carries:

- `data-prism-layer="<layer-id>"`
- `data-prism-datum-row="<row-id>"`

The JS port reads these to resolve clicks back to source rows.

## Driving conditional encodings

A selection name can drive a per-channel
[`condition`](encoding.md#conditions) clause so marks switch fills,
strokes, or opacities live as the selection state changes. See the
[brush_highlight gallery fixture](../gallery/conditions/brush_highlight.prism.json)
and the [highlight-on-brush cookbook recipe](../cookbook/highlight-on-brush.md).

## Worked examples

- [selection_point_bar](../gallery/selections/selection_point_bar.prism.json)
- [selection_interval_brush](../gallery/selections/selection_interval_brush.prism.json)
- [selection_cross_chart_overview](../gallery/selections/selection_cross_chart_overview.prism.json) + [_detail](../gallery/selections/selection_cross_chart_detail.prism.json)
