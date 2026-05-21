# Prism

Prism is a visualization library for `.pulse` files. It compiles
declarative JSON specs into charts — server-side SVG/PNG/PDF via Go,
and live in-browser via web components — using Vega-Lite-inspired
vocabulary with snake_case naming and Pulse expression syntax.

## Install

```
go install github.com/frankbardon/prism/cmd/prism@latest
```

## 60-second tour

```
prism init                          # writes .prism/ with schemas + examples
prism plot .prism/examples/bar_basic.json > bar.svg
prism plot --theme=dark bar.json > bar-dark.svg
prism serve --addr :8080            # Twirp + /prism/scene endpoint
prism mcp                            # MCP server over stdio
```

## Try it now

- [**Interactive Playground**](playground/about.md) — edit a spec
  and see it render live, entirely in your browser via WASM. ~25
  curated examples covering marks, composition, transforms, and
  themes.

## Where to go next

- [Getting started](getting-started.md) — install, first chart, editor setup.
- [Playground](playground/about.md) — live spec editor (WASM, no install).
- [Gallery](gallery/index.md) — 59 fixture specs with rendered SVGs.
- [Concepts](concepts/) — Spec, marks, encoding, composition, selections, themes, multi-source.
- [Reference](reference/) — spec field reference + error code catalog.
- [Cookbook](cookbook/) — recipes for common patterns.
- [Migration from Vega-Lite](migration-from-vega-lite.md).
