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

## Structured event shape

Every `prism:select` CustomEvent carries the same structured payload
across browser, Go, and Twirp contexts. The shape mirrors the Go
`selection.Event` struct (package `github.com/frankbardon/prism/selection`):

```jsonc
{
  "scene_id":     "scene-0",
  "selection_id": "brush",
  "kind":         "point",          // "point" | "interval" | "lasso"
  "timestamp":    1716826200000,    // ms since epoch
  "marks": [
    { "mark_index": 0, "instance_key": "layer-0:42" }
  ],
  "data_rows": [
    { "dataset_name": "cohort.pulse", "row_index": 42 }
  ],
  "data_extent": { "x": { "min": 10, "max": 50 } },   // interval/lasso
  "pixel_extent": { "x": { "min": 120, "max": 480 } },// interval/lasso, optional
  "spec_path": "/selection/brush"
}
```

`mark_index` is the index of the layer in the spec's `layer` array
(or `0` for a single-mark spec). `instance_key` is `<layer_id>:<row_id>`
and is stable across re-renders for the same source row. `data_extent`
is the canonical (renderer-size-independent) representation of an
interval brush; `pixel_extent` is best-effort UI-overlay info.

The browser handler:

```js
chart.addEventListener("prism:select", (ev) => {
  for (const mark of ev.detail.marks) {
    // mark.mark_index, mark.instance_key
  }
});
```

The Go side builds the same shape from raw input via
`selection.Build(...)`. Legacy `id` and `state` keys are retained on
the event payload for back-compat with handlers written before the
structured-event upgrade.

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
