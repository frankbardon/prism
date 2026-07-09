# Getting started

## Install

```
go install github.com/frankbardon/prism/cmd/prism@latest
prism version    # prism v0.2.0
```

## Bootstrap a project

```
mkdir my-project && cd my-project
prism init
```

This writes:

```
.prism/
├── schemas/         # JSON Schema files (offline validation + autocomplete)
├── examples/        # 8 curated starter specs
├── editor/          # VSCode / JetBrains / Neovim / Vim config templates
└── README.md
```

## First chart

```
cp .prism/examples/bar_basic.json my-chart.prism.json
prism plot my-chart.prism.json > chart.svg
open chart.svg
```

## Providing data

Prism never reads data files itself — a spec carries its rows inline, or
the host CLI supplies them at run time. The two ways to bind data:

- **Inline** — put the rows directly in the spec under `data.values`
  (optionally typed with `data.fields`). This needs no flag and is how
  every starter example works:

  ```json
  {
    "data": {"values": [{"brand": "alpha", "score": 0.42}]},
    "mark": "bar",
    "encoding": {
      "x": {"field": "brand", "type": "nominal"},
      "y": {"field": "score", "type": "quantitative"}
    }
  }
  ```

- **`--data rows.json`** — when a spec's `data` block names an external
  source (`data.source` / `data.ref`), pass a JSON rows file and the CLI
  feeds those rows to the resolver:

  ```
  prism plot chart.json --data rows.json > chart.svg
  ```

  where `rows.json` is a flat array of row objects:

  ```json
  [{"brand": "alpha", "score": 0.42}, {"brand": "beta", "score": 0.71}]
  ```

  The `--data` flag is accepted by `plot`, `plan`, `execute`, and
  `scene`.

## Editor setup

Each entry in `.prism/editor/` has a header comment with install
instructions. The fastest path:

- **VSCode** — copy `.prism/editor/vscode-settings.json` into
  `.vscode/settings.json`. `*.prism.json` files get autocomplete +
  inline validation from the embedded schema.
- **JetBrains** — copy `.prism/editor/jetbrains.xml` to `.idea/jsonSchemas.xml`.
- **Neovim** — paste the `.prism/editor/neovim.lua` snippet into your
  `init.lua` (requires `nvim-lspconfig`).
- **Vim** — paste the `.prism/editor/vim.alelint` block into your
  `.vimrc` (requires `dense-analysis/ale` and `prism` in PATH).

## Validating a spec

```
prism validate my-chart.prism.json
```

Returns `valid` on stdout (exit 0) or one or more `PRISM_*` errors with
fixup suggestions. Add `--json` for machine-readable envelopes.

## Rendering formats

```
prism plot my-chart.prism.json --format svg > chart.svg
prism plot dashboard.json --format svg > dashboard.svg
```

## Themes

```
prism plot bar.json --theme=dark > bar-dark.svg
prism plot bar.json --theme=print > bar-print.svg
```

Bundled themes: `light` (default), `dark`, `print`. Custom themes
via `theme.json` — see [Themes concepts](concepts/themes.md).

## Geographic charts

`geoshape` / `geopoint` marks need map tier geometry, which the host
binary loads at runtime rather than embedding. Point the loader at a
directory of tier files with `--geodata-dir` (or the `PRISM_GEODATA`
environment variable):

```
prism plot world.json --geodata-dir ./geodata > world.svg
PRISM_GEODATA=./geodata prism plot world.json > world.svg
```

A repo checkout already has the tiers in its `geodata/` directory; for a
standalone install, download `world-110m.geo.json` (and `world-50m` /
`admin1-50m` if your specs use them) from
`https://frankbardon.github.io/prism/static/prism/geodata/` into a folder
and pass that folder. Without a directory, rendering a geo mark fails with
`PRISM_GEODATA_DIR_UNSET`. The flag is accepted by `plot`, `scene`,
`serve`, `mcp`, and `static-bundle`. See
[Geographic Marks](concepts/geo.md) for the full workflow.

## Embed in a static page (no server)

Prism ships as a WebAssembly module that renders client-side.
Build the bundle, copy it into your site:

```
make build-wasm
./bin/prism static-bundle --wasm ./public/prism
```

Then drop a `<prism-chart>` element into any HTML page:

```html
<script src="/prism/wasm_exec.js"></script>
<script type="module" src="/prism/prism-element.mjs"></script>
<prism-chart spec="/specs/my-chart.prism.json"></prism-chart>
```

See [Browser / WASM concepts](concepts/browser.md) and the
[static-site cookbook](cookbook/embed-wasm.md) for mdBook /
Astro / Hugo integration recipes.

## What's next

- Browse the [gallery](gallery/index.md) for spec patterns.
- Read [Spec concepts](concepts/spec.md) to learn the data → transform
  → mark → encoding pipeline.
- See [Multi-source](concepts/multi-source.md) to join multiple
  datasets in one chart.
- See [Browser / WASM](concepts/browser.md) for the standalone
  client-side rendering path.
- Read [Migration from Vega-Lite](migration-from-vega-lite.md) if you
  already know Vega-Lite.
