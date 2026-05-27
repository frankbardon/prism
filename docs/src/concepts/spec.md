# Spec

A Prism Spec is a JSON document describing one chart. It is the
contract between authors (humans / agents) and the Prism pipeline.

## Six-stage pipeline

```
Spec (JSON) → Parse → Validate → Plan → Compile → Encode → Render → Bytes
                                          │
                                          ├─→ Pulse engine (data ops)
                                          └─→ Renderer backend (SVG / Canvas / PDF)
```

## Minimum viable spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"source": "cohort.pulse"},
  "mark": "bar",
  "encoding": {
    "x": {"field": "brand_id", "type": "nominal"},
    "y": {"field": "score",    "type": "quantitative", "aggregate": "mean"}
  }
}
```

Five top-level keys are typically present:

| Key | Purpose |
|---|---|
| `$schema` | URN identifier (`urn:prism:schema:v1:spec`) for editor autocomplete + version pinning. |
| `data` | Where to read rows from — a `.pulse` source, an inline `values` array, a named alias, etc. |
| `transform` | Optional array of row-level operations (filter, calculate, aggregate, sort, ...). |
| `mark` | What to draw — `bar`, `line`, `point`, `pie`, `sankey`, ... |
| `encoding` | How to bind data fields to visual channels (x/y/color/size/...). |

## Full top-level field list

```
$schema       data            datasets        transform
mark          encoding        layer           concat
hconcat       vconcat         facet           repeat
spec          selection       resolve         theme
config        width           height          padding
background    title           subtitle        description
projection    animation
```

Exactly one of `mark | layer | concat | hconcat | vconcat | facet | repeat`
must be present. The validator enforces this with `PRISM_SPEC_*` codes.

## Animation

The optional `animation` block requests a client-side tween whenever the
spec swaps. Static SVG and PDF output is unaffected — both renderers
ignore the block entirely. Only the browser web component
(`<prism-chart>`) and the WASM runtime honour it.

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data":    {"name": "sales", "values": [...]},
  "mark":    "bar",
  "encoding": {
    "x": {"field": "region", "type": "nominal", "key": true},
    "y": {"aggregate": "mean", "field": "score", "type": "quantitative"}
  },
  "animation": {"duration_ms": 600, "easing": "cubic_in_out"}
}
```

Fields:

| Field | Default | Notes |
|---|---|---|
| `duration_ms` | `400` | Total tween length, capped at 5000. |
| `easing` | `cubic_in_out` | One of `linear`, `cubic_*`, `quad_*`, `sine_*`, `expo_*` (× `in`/`out`/`in_out`). |
| `stagger_ms` | `0` | Per-mark delay applied in document order. |
| `enter` | `fade` | `fade` or `none`. Marks that appear at scene-swap time. |
| `exit`  | `fade` | `fade` or `none`. Marks that disappear at scene-swap time. |

For the tween to match marks across scene swaps (object constancy),
declare a join key on one encoding channel via `"key": true`. Without a
key, validation fires `PRISM_SPEC_023`.

Animation respects the user's
[`prefers-reduced-motion`](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-reduced-motion)
setting: the animator snaps directly to the final state when the
preference is `reduce`.

When two scenes are structurally incompatible (different layer count,
different mark families, etc.) the animator falls back to an instant
replace and emits `PRISM_WARN_ANIM_FALLBACK` on the
`prism:warn` CustomEvent stream.

Spec rules that govern `animation`:

- `PRISM_SPEC_022` — unknown easing name.
- `PRISM_SPEC_023` — block declared but no channel has `key: true`.
- `PRISM_SPEC_024` — more than one channel carries `key: true`.

## Strict by default

- Unknown fields error (typos like `xfield` vs `x.field` caught at parse).
- Semantic violations error (agg op on incompatible field type, etc.).
- 24+ `PRISM_SPEC_*` rules cover field-existence, channel-for-mark,
  selection refs, expression parsing, scale type compatibility,
  animation easing / key constraints, and more. Run
  `prism errors lookup <code>` for details on any.

## Validate a spec

```
prism validate my-chart.prism.json
prism validate --json my-chart.prism.json
```

## Spec patches (RFC 6902)

Iterative edits to a rendered chart don't need a full spec re-send.
A caller can transmit an [RFC 6902 JSON Patch][rfc6902] and the
library applies it atomically, re-decodes, and re-compiles:

```json
[
  { "op": "replace", "path": "/mark", "value": "area" },
  { "op": "add",     "path": "/encoding/color",
                     "value": { "field": "category", "type": "nominal" } },
  { "op": "test",    "path": "/data/name", "value": "current_window" },
  { "op": "remove",  "path": "/title" }
]
```

Same protocol in Go and in WASM:

```go
next, err := prism.ApplyPatch(s, patch)
// or, statefully:
scn, _ := prism.NewScene(ctx, s, prism.CompileOptions{})
err := scn.Apply(patch)
```

```js
const newSpecJSON = prism.applyPatch(specJSON, JSON.stringify(patch));
const patchJSON   = prism.diffSpecs(beforeJSON, afterJSON);
```

**Atomic application.** Either every operation in the patch
succeeds and the new spec replaces the old, or no state changes.
A failing op surfaces as `PRISM_SPEC_PATCH_001` with the offending
op index in the envelope's `Details.OpIndex`.

**Test operations.** Include a `test` op to fail-fast on
optimistic-concurrency violations — the patch aborts if the
current spec value at `path` differs from the expected value.

**Diff helper.** `prism.DiffSpecs(before, after)` (Go) and
`prism.diffSpecs(beforeJSON, afterJSON)` (WASM) produce a patch
that transforms one spec into the other. Useful for callers that
think in full specs and only want to transmit the delta.

[rfc6902]: https://www.rfc-editor.org/rfc/rfc6902

## Further reading

- [Marks](marks.md), [Encoding](encoding.md), [Composition](composition.md).
- [Spec field reference](../reference/spec.md) — every field with type + description.
- [Gallery](../gallery/index.md) — 59 worked examples.
