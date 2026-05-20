# Cookbook: faceting by data values

Render one mini-chart per partition of a categorical field.

## Spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"source": "cohorts/q1.pulse"},
  "facet": {"column": {"field": "region"}},
  "spec": {
    "$schema": "urn:prism:schema:v1:spec",
    "mark": "bar",
    "encoding": {
      "x": {"field": "brand_id", "type": "nominal"},
      "y": {"field": "score", "type": "quantitative", "aggregate": "mean"}
    }
  },
  "resolve": {"scale": {"y": "shared"}}
}
```

## Notes

- The upstream Source + transform pipeline runs **once**; the
  resulting Table is partitioned at encode time.
- `resolve.scale.y: shared` (default for facet) computes the union
  y-domain so cells are visually comparable. Use `independent` for
  per-cell domains.
- Nested facets work (facet within facet within facet) — the inner
  `spec` field is recursive.
- For grid by **field list** instead of by data values, use
  [repeat](../concepts/composition.md#repeat).
