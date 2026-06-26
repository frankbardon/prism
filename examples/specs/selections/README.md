# Selection fixtures

Three demoable specs that exercise P13 selections (D004 / D077–D082):

| Fixture | Selection kind | Demo |
| --- | --- | --- |
| `selection_point_bar.json` | point on `brand_id` | click a bar → highlight (CSS class flip) |
| `selection_interval_brush.json` | interval on `x` | drag across plot → range filter |
| `selection_cross_chart_overview.json` + `selection_cross_chart_detail.json` | shared point selection ID `brand_focus` | overview-detail filter via `<prism-coordinator>` |

The cross-chart pair share the same selection ID (`brand_focus`). When
wrapped in a `<prism-coordinator>`, clicking a bar in the overview chart
re-dispatches the selection to the detail chart, which then highlights
the matching points across its three-month time series.

The browser demo lives at `testdata/browser/selections-demo.html`.
Serve the repo root with any static HTTP server (e.g. `python3 -m
http.server`) and visit `/testdata/browser/selections-demo.html`.

Each fixture validates + encodes through the gate
`TestPrismSelectionFixturesEncode` (in `encode/`). Run via
`go test ./encode/... -run TestPrismSelectionFixtures`.
