# Incremental Edits with Spec Patches

For interactive scenes — change one encoding, swap a data source,
toggle a layer — sending the full spec across the wire is
wasteful. Prism speaks [RFC 6902 JSON Patch][rfc6902] so callers
transmit just the delta.

[rfc6902]: https://www.rfc-editor.org/rfc/rfc6902

## The shape

```json
[
  { "op": "replace", "path": "/mark", "value": "area" },
  { "op": "add",     "path": "/encoding/color",
                     "value": { "field": "category", "type": "nominal" } },
  { "op": "test",    "path": "/data/name", "value": "current_window" },
  { "op": "remove",  "path": "/title" }
]
```

Six op types — `add`, `remove`, `replace`, `move`, `copy`,
`test`. Paths are JSON Pointers (RFC 6901): `/encoding/x/field`,
`/layer/0/mark`, `/datasets/main/values/-` (the `-` token appends
to an array).

## Browser: optimistic incremental edit

```html
<prism-chart id="chart" spec="./bar.prism.json"></prism-chart>

<script type="module">
  const chart = document.getElementById("chart");
  const initialSpec = chart.getAttribute("spec");

  async function switchToArea() {
    const patch = JSON.stringify([
      { op: "replace", path: "/mark", value: "area" },
    ]);
    const nextSpec = prism.applyPatch(initialSpec, patch);
    chart.setAttribute("spec", nextSpec);  // triggers re-render
  }
</script>
```

`prism.applyPatch` returns the patched spec as JSON. Hand it
straight back to the chart element or feed it into
`prism.compile` for inspection without re-rendering.

## Atomic semantics + `test`

Either every op applies cleanly or no state changes. Use `test`
to fail-fast on optimistic-concurrency violations:

```js
const patch = JSON.stringify([
  { op: "test",    path: "/encoding/x/field", value: "brand_id" },
  { op: "replace", path: "/encoding/x/field", value: "category" },
]);
const out = prism.applyPatch(currentSpec, patch);
const parsed = JSON.parse(out);
if (parsed.ok === false) {
  // PRISM_SPEC_PATCH_001 — current value drifted; refresh and retry.
}
```

A failing op surfaces as `PRISM_SPEC_PATCH_001` with the offending
op's index in `error.Context.OpIndex`.

## Diff helper — think in specs, transmit deltas

```js
const before = JSON.stringify(originalSpec);
const after  = JSON.stringify(editedSpec);
const patchJSON = prism.diffSpecs(before, after);

// Apply remotely:
socket.send(patchJSON);
```

`prism.diffSpecs` produces a correct (but not necessarily minimal)
patch. The other side calls `prism.applyPatch(local, patchJSON)`
and lands on the same spec.

## Go-native: stateful Scene

The `prism.Scene` struct wraps a spec + its last compiled plan:

```go
import (
    "context"
    prism "github.com/frankbardon/prism"
)

scn, err := prism.NewScene(ctx, s, prism.CompileOptions{})
if err != nil {
    return err
}

// Swap the mark type — atomic re-compile under the hood.
if err := scn.Apply(prism.Patch{
    {Op: "replace", Path: "/mark", Value: "area"},
}); err != nil {
    // Failed patches leave scn.Spec() and scn.Plan() unchanged.
    return err
}

plan := scn.Plan()  // freshly compiled
```

`scn.ApplyAndRender(patch)` is shorthand for `Apply` + `Plan()`.
Hand the returned plan to a renderer for pixel bytes.

## Building a patch from scratch

For programmatic edits, build the patch slice directly:

```go
patch := prism.Patch{
    {Op: "replace", Path: "/data/ref", Value: "live_window"},
    {Op: "add",     Path: "/encoding/color", Value: map[string]any{
        "field": "segment",
        "type":  "nominal",
    }},
}
```

Or compute it from two known specs:

```go
patch, err := prism.DiffSpecs(before, after)
```

## Performance note

This first cut applies every patch by re-decoding the patched
spec and re-running the full compile pipeline. Partial
re-validation and per-mark re-compilation (touched layers only)
are tracked as a follow-up — the patch API contract is stable;
the optimisation lands transparently underneath.

## Error reference

`prism errors lookup PRISM_SPEC_PATCH_001` lists fixup guidance.
The envelope's `Details` carries:

| Key | Meaning |
|---|---|
| `OpIndex` | Zero-based index of the failing op in the patch array |
| `Op` | The op name (`add` / `replace` / …) |
| `Path` | The JSON Pointer at fault |
