# Consume Structured Selection Events

Every `prism:select` CustomEvent carries a structured `Event`
payload (mirrors the Go `selection.Event` struct). The same shape
travels across browser, Go-native, and Twirp contexts so one
handler works against any binding.

## The event shape

```jsonc
{
  "scene_id":     "scene-0",
  "selection_id": "brush",
  "kind":         "point",          // "point" | "interval" | "lasso"
  "timestamp":    1716826200000,
  "marks": [
    { "mark_index": 0, "instance_key": "layer-0:42" }
  ],
  "data_rows": [
    { "dataset_name": "cohort.pulse", "row_index": 42 }
  ],
  "data_extent":  { "x": { "min": 10, "max": 50 } },
  "pixel_extent": { "x": { "min": 120, "max": 480 } },
  "spec_path":    "/selection/brush"
}
```

`mark_index` is the layer's index in the spec's `layer` array (or
`0` for unlayered charts). `instance_key` is stable across
re-renders for the same source row — derive joins and lookups
from it.

## Browser: forward selections to a sidebar

```html
<prism-chart id="chart" spec="./bar.prism.json"></prism-chart>
<aside id="sidebar"></aside>

<script type="module">
  const chart   = document.getElementById("chart");
  const sidebar = document.getElementById("sidebar");

  chart.addEventListener("prism:select", (ev) => {
    const e = ev.detail;
    if (e.kind === "point") {
      sidebar.innerHTML = e.marks
        .map(m => `<div>${m.instance_key}</div>`)
        .join("");
    } else if (e.kind === "interval" && e.data_extent?.x) {
      const { min, max } = e.data_extent.x;
      sidebar.textContent = `x ∈ [${min}, ${max}]`;
    }
  });
</script>
```

The event bubbles + composes through Shadow DOM, so listening on
`document` or any ancestor also works.

## Browser: cross-app forwarding (Slack, websocket, postMessage)

Because the event is fully structured, you can serialise it
directly:

```js
chart.addEventListener("prism:select", (ev) => {
  socket.send(JSON.stringify(ev.detail));
});
```

No translation step — the receiver gets the same `selection.Event`
shape the renderer emitted.

## Go: build an event from raw input

The Go side exposes the same shape via the `selection` package.
Use it from a Twirp handler, MCP tool, or any server-side
selection synthesis path:

```go
import "github.com/frankbardon/prism/selection"

ev, err := selection.Build(selection.BuildInput{
    SceneID:     "scene-0",
    SelectionID: "brush",
    Kind:        selection.KindPoint,
    Points: []selection.PointHit{
        {LayerID: "layer-0", RowID: 42},
    },
}, sceneDoc, spec)
if err != nil {
    return err
}
body, _ := json.Marshal(ev)
// body is byte-identical to the browser-side CustomEvent.detail.
```

`selection.Build` walks the SceneDoc to resolve `mark_index` and
`dataset_name` from the (layer_id, row_id) pair. Unknown layers
(stale events after re-render) come back with `mark_index = -1`
so the consumer can decide whether to drop the entry.

## Back-compat

Pre-existing handlers that consumed `{id, state}` keys still work
— those fields are retained on the event payload alongside the
new structured ones.

## Worked examples

- [Highlight-on-brush](highlight-on-brush.md) — wires a selection
  to conditional encoding on the same chart.
- [Selection point bar fixture](../gallery/selections/selection_point_bar.prism.json)
  — minimal spec that emits point selection events.
