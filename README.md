# Prism

Prism is a visualization library for [`.pulse`][pulse] files. It
compiles declarative JSON specs into charts — server-side SVG / PNG /
PDF via Go, and live in-browser via web components — using
Vega-Lite-inspired vocabulary with snake_case naming and Pulse
expression syntax.

[pulse]: https://github.com/frankbardon/pulse

## Install

```
go install github.com/frankbardon/prism/cmd/prism@latest
prism version   # prism v1.0.0
```

## First chart in 5 lines

```
prism init
cp .prism/examples/bar_basic.json my-chart.prism.json
prism plot my-chart.prism.json > chart.svg
open chart.svg
```

## What ships in v1.0

- **Six-stage pipeline** — Spec → Validate → Plan → Compile → Encode → Render.
- **20+ marks** — bar, line, area, point, rule, text, tick, rect, arc,
  pie, donut, histogram, heatmap, boxplot, violin, sankey, funnel,
  sparkline, image, path.
- **Composition** — `layer`, `concat`, `hconcat`, `vconcat`, `facet`,
  `repeat`. Cross-layer scale resolution (shared/independent).
- **Multi-source** — `datasets` block + hash join + parallel Source
  execution. Server-side + browser-side dataset registries.
- **Selections** — point + interval; client + server reactive modes.
- **Themes** — `light`, `dark`, `print` built-in. Sparse override via
  spec or custom `theme.json`.
- **Renderers** — SVG (Go), Canvas-via-JS (vendored ESM web
  component), PDF (`gopdf` with embedded fonts, paginated grids).
- **Service surface** — Twirp HTTP + MCP stdio. `prism serve`,
  `prism mcp`.
- **CLI** — `validate`, `plan`, `execute`, `plot`, `scene`,
  `serve`, `mcp`, `inspect`, `examples`, `schema`, `init`,
  `errors lookup`, `static-bundle`, `version`.

## Documentation

- [Getting started](docs/getting-started.md) — install + first chart + editor setup.
- [Gallery](docs/gallery/index.md) — 59 fixture specs with rendered SVGs.
- [Concepts](docs/concepts/) — spec, marks, encoding, composition, selections, themes, multi-source.
- [Cookbook](docs/cookbook/) — multi-source join, faceting, custom themes, MCP agent integration.
- [Migration from Vega-Lite](docs/migration-from-vega-lite.md).
- [Errors](docs/reference/errors.md) — every `PRISM_*` code with fixups.

## License

MIT — see [LICENSE](./LICENSE).

## Contributing

Open an issue or PR at the repo. The `.planning/` directory carries
the design discussion, phase plans, and locked decisions. New
features should land with a PHASE.md + PLAN.md following the existing
pattern.
