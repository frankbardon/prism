# Cookbook: multi-source join

Compare two cohorts side-by-side via hash join.

## Spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "datasets": {
    "current": {"values": [
      {"brand_id": "alpha", "score": 0.62},
      {"brand_id": "beta",  "score": 0.48},
      {"brand_id": "alpha", "score": 0.66}
    ]},
    "prior": {"values": [
      {"brand_id": "alpha", "score": 0.55},
      {"brand_id": "beta",  "score": 0.51},
      {"brand_id": "beta",  "score": 0.47}
    ]}
  },
  "transform": [
    {"data": "current", "groupby": ["brand_id"],
     "aggregate": [{"op": "mean", "field": "score", "as": "current_score"}],
     "as": "cur"},
    {"data": "prior", "groupby": ["brand_id"],
     "aggregate": [{"op": "mean", "field": "score", "as": "prior_score"}],
     "as": "pri"},
    {"join": {"left": "cur", "right": "pri", "on": "brand_id"}, "as": "joined"},
    {"data": "joined", "calculate": {"op": "sub", "operands": [{"field": "current_score"}, {"field": "prior_score"}]}, "as": "delta"}
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
- `PRISM_QUERY_WORKERS` (defaults to `NumCPU`) controls the executor
  worker pool — both group-aggregates run concurrently.
- The rows here are inlined for illustration; in production the caller
  materializes each cohort upstream and inlines it via `datasets`
  (or supplies a `DataResolver` bound to a `ref`).
