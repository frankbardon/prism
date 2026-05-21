# Cookbook: handling missing data after a join

Left and outer joins return null cells for unmatched rows. The
encoder skips marks whose channel-bound fields are null and surfaces
`PRISM_WARN_NULL_DROPPED` with the row count + offending channels.
This cookbook shows how to handle the warning intentionally.

## Spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "datasets": {
    "sales":  {"source": "cohorts/q1_sales.pulse"},
    "quotas": {"source": "cohorts/q1_quotas.pulse"}
  },
  "data": {"name": "joined"},
  "transform": [{
    "join": {
      "left":  "sales",
      "right": "quotas",
      "on":    ["region"],
      "kind":  "left"
    },
    "as": "joined"
  }],
  "mark": "bar",
  "encoding": {
    "x": {"field": "region", "type": "nominal"},
    "y": {"field": "amount", "type": "quantitative"},
    "color": {"field": "quota", "type": "quantitative"}
  }
}
```

A region with no quota row returns `quota: null` after the join.
The encoder drops that mark and emits a warning instead of silently
painting a bar with `color = 0` (which would look like "this region
has a quota of zero", a wrong reading).

## Variations

**Impute missing values before encoding** with a calculate
transform:

```json
"transform": [
  {"join": {...}, "as": "joined"},
  {"calculate": "quota == null ? 0 : quota", "as": "quota_padded"}
]
```

Pulse expressions are strict on null inputs (any null input
propagates to null), so use the comparison form above to default
before downstream channels read the field.

**Filter empty groups** when an aggregate over an all-null group
would otherwise return null:

```json
"transform": [
  {"aggregate": [{"op": "mean", "field": "quota", "as": "quota_mean"}], ...},
  {"filter": "quota_mean != null"}
]
```

## See also

- [Multi-source › Null handling](../concepts/multi-source.md#null-handling)
- `PRISM_WARN_NULL_DROPPED` — emitted when the encoder drops rows.
- `PRISM_WARN_NULL_AGG_ALL` — emitted when an aggregate group's
  every input row was null.
