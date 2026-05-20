# Prism Documentation

mdBook source for the Prism user manual, published to GitHub Pages at
<https://frankbardon.github.io/prism/>.

## Local preview

```
$ make docs-serve
```

(Equivalent to `mdbook serve docs --open`.)

## One-shot build

```
$ make docs
```

Builds the `prism` binary, regenerates every gallery scene from its
`*.prism.json` source (incremental — only stale entries recompile),
then runs `mdbook build docs`. Output lands in `docs/book/`, which is
gitignored. `make docs-clean` removes the book and every generated
`*.scene.json`.

## Audience

This site documents the CLI, spec format, Go embedding, rendering
backends, and operations for human readers. The schema bundle in
`schema/` and the embedded skill / example surfaces in `cmd/prism/`
are the LLM-facing artifacts loaded by `prism init` and `prism mcp`.

## Live gallery + JS bundle

`docs/src/gallery/index.html` renders the fixture specs live via the
vendored `prism-chart` web component. Two automated couplings keep it
working with zero manual sync:

1. **JS bundle** — `docs/src/static` is a committed directory symlink
   pointing at `../../static`. mdBook follows the symlink during build
   and copies `static/vendor/prism/` into `docs/book/static/`. Edit
   `static/vendor/prism/*.mjs` and the next `make docs` picks it up.

2. **SceneDoc JSON** — the JS web component renders precompiled
   `SceneDoc` JSON (not raw spec). The `make docs-scenes` Make target
   walks `docs/src/gallery/**/*.prism.json` and runs `prism scene` on
   each, producing the `*.scene.json` files referenced by
   `<prism-chart src=...>`. These are gitignored; `make docs` (and
   `make docs-serve`) rebuild them incrementally. The
   `.github/workflows/docs.yml` workflow runs the same Make target,
   so GitHub Pages always serves fresh scenes.

If the symlink ever goes missing (e.g. a fresh clone on a system that
doesn't preserve it), recreate with:

```
ln -s ../../static docs/src/static
```
