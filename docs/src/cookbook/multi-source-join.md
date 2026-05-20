# Cookbook: multi-source join

Compare two cohorts side-by-side via hash join.

## Spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "datasets": {
    "current": {"source": "cohorts/q1.pulse"},
    "prior":   {"source": "cohorts/q4_2025.pulse"}
  },
  "transform": [
    {"data": "current", "groupby": ["brand_id"],
     "aggregate": [{"op": "mean", "field": "score", "as": "current_score"}],
     "as": "cur"},
    {"data": "prior", "groupby": ["brand_id"],
     "aggregate": [{"op": "mean", "field": "score", "as": "prior_score"}],
     "as": "pri"},
    {"join": {"left": "cur", "right": "pri", "on": "brand_id"}, "as": "joined"},
    {"data": "joined", "calculate": "current_score - prior_score", "as": "delta"}
  ],
  "mark": "bar",
  "encoding": {
    "x": {"field": "brand_id", "type": "nominal", "sort": "-y"},
    "y": {"field": "delta", "type": "quantitative", "title": "Score delta vs Q4"}
  }
}
```

## Notes

- Hash join is in-memory. Cardinality ceiling is `PRISM_JOIN_MAX_ROWS`
  (5M default; override via env).
- The optimizer's `AggregateFusion` pass would collapse the two
  group-aggregates if they shared an input; here they're on different
  sources so both run in parallel.
- `parallel.PRISM_QUERY_WORKERS` (defaults to `NumCPU`) controls the
  worker pool — both Pulse opens run concurrently.
