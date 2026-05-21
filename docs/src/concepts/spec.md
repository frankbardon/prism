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

## Further reading

- [Marks](marks.md), [Encoding](encoding.md), [Composition](composition.md).
- [Spec field reference](../reference/spec.md) — every field with type + description.
- [Gallery](../gallery/index.md) — 59 worked examples.
