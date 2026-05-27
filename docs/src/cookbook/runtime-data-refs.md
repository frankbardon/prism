# Runtime Data References (`setDataResolver`)

A `{data: {ref: "<name>"}}` spec leaves data binding to the
rendering environment. The spec describes *what to draw*; a
caller-supplied resolver provides *the data to draw it with*.
Lets one spec render in a browser, server, and test harness
without modification.

## The spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"ref": "current_window"},
  "mark": "line",
  "encoding": {
    "x": {"field": "ts",   "type": "temporal"},
    "y": {"field": "rate", "type": "quantitative"}
  }
}
```

The string `current_window` is opaque to Prism — it's whatever
identifier the caller's resolver understands.

## Browser: live data from a fetch

Synchronous return is required (Go-WASM cannot await a Promise
mid-execute). Pre-resolve the async data and register a sync
getter:

```html
<prism-chart id="chart" spec="./live.prism.json"></prism-chart>

<script type="module">
  const data = await fetch("/api/window.json").then(r => r.json());

  prism.setDataResolver((ref) => {
    if (ref === "current_window") return { values: data };
    return null;  // unresolved refs fall back to PRISM_RESOLVE_REF_UNRESOLVED
  });

  document.getElementById("chart").reload();
</script>
```

The callback returns the same shape as inline `data: {values:
[...]}` — an object with `values` (row array) and optional
`fields` (column-type hints).

## Browser: chart-driven refresh on a timer

```js
async function refresh() {
  const window = await fetchWindow();
  prism.setDataResolver(ref => ref === "current_window"
    ? { values: window } : null);
  chart.reload();
}
setInterval(refresh, 60_000);
```

Each `chart.reload()` re-runs the compile pipeline; the resolver
is consulted afresh and returns the most recent rows.

## Go-native: in-process resolver

```go
import (
    "context"

    prism "github.com/frankbardon/prism"
    "github.com/frankbardon/prism/plan"
    "github.com/frankbardon/prism/plan/build"
    "github.com/frankbardon/prism/resolve"
    "github.com/frankbardon/prism/spec"
)

func compile(ctx context.Context, body []byte, live []map[string]any) (*prism.CompiledPlan, error) {
    s, err := spec.DecodeBytes(body)
    if err != nil {
        return nil, err
    }
    resolver := resolve.MapDataResolver{
        "current_window": {Values: live},
    }
    return prism.Compile(ctx, s, prism.CompileOptions{
        Build: build.Options{DataResolver: resolver},
        Exec:  plan.ExecOpts{Workers: 1},
    })
}
```

`resolve.MapDataResolver` is the static map-backed implementation.
For dynamic lookups (e.g. database, cache layer) wrap your logic
in `resolve.DataResolverFunc`:

```go
resolver := resolve.DataResolverFunc(func(ctx context.Context, ref string) (*resolve.Dataset, error) {
    rows, err := db.QueryWindow(ctx, ref)
    if err != nil {
        return nil, err
    }
    return &resolve.Dataset{Values: rows}, nil
})
```

Chain multiple resolvers (e.g. cache → DB → fallback) with
`resolve.ChainDataResolvers(cache, primary)`.

## Test fixtures

```go
func TestChartShape(t *testing.T) {
    body := mustReadSpec(t, "testdata/live.prism.json")
    plan, err := prism.CompileJSON(context.Background(), body, prism.CompileOptions{
        Build: build.Options{
            DataResolver: resolve.MapDataResolver{
                "current_window": {Values: []map[string]any{
                    {"ts": "2026-01-01", "rate": 0.42},
                }},
            },
            Backend:  inmem.New(),
        },
    })
    if err != nil { t.Fatal(err) }
    if plan.Marks[0].InstanceCount != 1 { t.Errorf("rows = %d", plan.Marks[0].InstanceCount) }
}
```

The same spec drives every environment. No fixture fork, no
URL rewrite.

## Error surface

| Condition | Code |
|---|---|
| No resolver installed | `PRISM_RESOLVE_REF_UNRESOLVED` |
| Resolver returned null / `ErrDataRefUnresolved` | `PRISM_RESOLVE_REF_UNRESOLVED` |
| Resolver returned undecodable JSON (WASM) | `PRISM_RESOLVE_REF_UNRESOLVED` |
| Async/Promise callback | Surfaces as undecodable → unresolved |

Run `prism errors lookup PRISM_RESOLVE_REF_UNRESOLVED` for fixup
guidance.
