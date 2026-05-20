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
```

Exactly one of `mark | layer | concat | hconcat | vconcat | facet | repeat`
must be present. The validator enforces this with `PRISM_SPEC_*` codes.

## Strict by default

- Unknown fields error (typos like `xfield` vs `x.field` caught at parse).
- Semantic violations error (agg op on incompatible field type, etc.).
- 21+ `PRISM_SPEC_*` rules cover field-existence, channel-for-mark,
  selection refs, expression parsing, scale type compatibility, and
  more. Run `prism errors lookup <code>` for details on any.

## Validate a spec

```
prism validate my-chart.prism.json
prism validate --json my-chart.prism.json
```

## Further reading

- [Marks](marks.md), [Encoding](encoding.md), [Composition](composition.md).
- [Spec field reference](../reference/spec.md) — every field with type + description.
- [Gallery](../gallery/index.md) — 59 worked examples.
