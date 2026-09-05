# Playground

Prism ships an [interactive playground](index.html) that runs the
full spec → validate → plan → compile → encode → render pipeline in
your browser via WASM. No server, no install.

[**→ Open the playground**](index.html)

## What it does

- **Live render.** Edit a JSON spec on the left; the rendered SVG
  on the right updates after a short debounce. Errors surface
  inline with their canonical `PRISM_*` code, message, and any
  attached fixups — the same envelope you get from the CLI or the
  MCP tool.
- **Curated examples.** ~25 specs across basic marks, distributions,
  composition operators, transforms, scales, and themes. Click an
  entry in the sidebar to load it.
- **Theme switch.** Flip between the `light`, `dark`, and `print`
  themes without reloading.
- **Inspector tabs.** Below the preview: the rendered **SVG**
  source, the resolved **Scene IR**, the plan **DAG** node list,
  and the **raw spec** as the WASM bridge sees it.
- **Share.** "Share" copies a URL with the spec encoded in the
  fragment (deflate-raw + base64url). Past a couple-kilobyte spec
  it stays comfortably within URL-length budgets, and Discord /
  Slack / mail clients will preserve it on copy/paste.
- **Local persistence.** Edits survive a reload via `localStorage`;
  hit "Reset" or click any sidebar entry to reload a clean
  example.
- **Keyboard.** `Tab` / `Shift+Tab` indent; `Ctrl/⌘+S` formats the
  spec; `Ctrl/⌘+Enter` forces an immediate render.

## What it doesn't do (yet)

- **External data sources.** Every example inlines its rows via
  `data.values` / `datasets.*.values`. Prism only ever consumes
  already-materialized rows — the playground has no data-fetch path,
  so the curated examples stay self-contained. To render data from a
  live source, materialize it upstream and inline the rows (`prism
  plot` / `prism serve` accept the same inline specs locally).
- **Selection events.** Pointer hit-testing is part of the
  `<prism-chart>` web component (see the [gallery live
  demo](../gallery/live.html)). The playground mounts the raw SVG so
  it stays a focused spec-to-SVG editor; selection wiring lands in v2.

## Where the bytes come from

The playground loads the same `prism.wasm` that powers the gallery
(`docs/src/static/prism/prism.wasm`, served via the `docs/src/static`
symlink to `static/vendor/prism/`). The WASM binary contains every
stage of the pipeline; the playground JS is a ~15 KiB shell that
debounces keystrokes, marshals JSON across the bridge, and updates
the DOM.
