# Embed Prism in a Static Site

Prism ships as a WebAssembly module that renders charts entirely
in the browser. This recipe walks through dropping Prism into a
static site (plain HTML, mdBook, Astro, Hugo, GitHub Pages) with
no backend.

## Prerequisites

- Go 1.24+ to produce `prism.wasm` and host CLI.
- A static file server (anything that serves
  `Content-Type: application/wasm` correctly — GitHub Pages,
  Netlify, Vercel, S3, nginx, `python -m http.server`, all work).

`file://` does **not** work — browsers refuse to instantiate WASM
from local file URLs. Run a local server during development.

## 1. Build the bundle

From a Prism checkout:

```bash
make build-wasm
./bin/prism static-bundle --wasm ./public/prism
```

That writes:

```
public/prism/
├── index.html           # minimal loader example
├── prism.wasm           # ~12 MiB gzipped
├── wasm_exec.js         # Go toolchain runtime
├── prism.mjs            # bootstrapper
├── prism-element.mjs    # web components
├── prism-resolver.mjs   # dataset registry
└── prism-selection.mjs  # interaction wiring
```

Copy `public/prism/` into your static site's deployed root. Or
serve it from any path — references inside the bundle are
relative, so `/static/prism/`, `/assets/prism/`, etc. all work.

## 2. Drop a chart into a page

```html
<!doctype html>
<html>
<head>
  <link rel="preload" as="fetch" type="application/wasm"
        href="/prism/prism.wasm" crossorigin>
  <script src="/prism/wasm_exec.js"></script>
  <script type="module" src="/prism/prism-element.mjs"></script>
</head>
<body>
  <prism-chart spec='{
    "$schema": "urn:prism:schema:v1:spec",
    "data": {"values": [
      {"region": "NA", "score": 0.82},
      {"region": "EU", "score": 0.74},
      {"region": "APAC", "score": 0.68}
    ]},
    "mark": "bar",
    "encoding": {
      "x": {"field": "region", "type": "nominal"},
      "y": {"field": "score", "type": "quantitative"}
    }
  }'></prism-chart>
</body>
</html>
```

The `spec=` attribute accepts inline JSON or a URL pointing at a
`.prism.json` file. The first call to `<prism-chart>` triggers
the WASM download; subsequent charts on the same page reuse the
loaded instance.

## 3. Share datasets across charts

`<prism-dataset>` declares an alias the WASM bridge resolves at
fetch time:

```html
<prism-dataset name="current" src="/data/q1.pulse"></prism-dataset>
<prism-dataset name="bench"   src="/data/industry.pulse"></prism-dataset>

<prism-chart spec="/specs/actual_vs_benchmark.prism.json"></prism-chart>
<prism-chart spec="/specs/trend.prism.json"></prism-chart>
```

Both charts share fetches: the page issues one HTTP request per
unique `src`, not one per chart.

## 4. mdBook integration

Drop the bundle under `theme/`:

```
mybook/
├── book.toml
├── src/
│   ├── SUMMARY.md
│   └── chapter1.md
└── theme/
    └── prism/...      # contents of public/prism/
```

In `book.toml`, declare the additional JS:

```toml
[output.html]
additional-js = [
  "theme/prism/wasm_exec.js",
  "theme/prism/prism-element.mjs"
]
```

Then in any chapter:

```markdown
<prism-chart src-spec="/charts/example.prism.json"></prism-chart>
```

## 5. Astro / Hugo / static-site generators

Treat the bundle as a static asset directory. In Astro put it
under `public/prism/`; in Hugo under `static/prism/`. Include the
two script tags from step 2 in your base layout. The web
components register globally; any page that uses
`<prism-chart>` renders without additional wiring.

## Tuning

- **Preload the wasm**:
  `<link rel="preload" as="fetch" type="application/wasm" href="prism.wasm" crossorigin>`
  starts the download in parallel with the page parse.
- **Set CORS**: when the dataset origin differs from the page
  origin, the `.pulse` host must return `Access-Control-Allow-
  Origin` matching the page. Errors surface as `PRISM_WASM_001`.
- **Theme switching**: set `theme="dark"` on `<prism-chart>` —
  the browser re-runs `executeSpec` with the new theme. Fast
  because the WASM instance + dataset cache stay warm.

## Limits

- Initial WASM download is ~12 MiB gzipped. Cache aggressively
  (immutable hashed filename + 1-year Cache-Control).
- Large `.pulse` files (>50 MB) decode slowly in the browser on
  mid-range hardware. Pre-aggregate at build time when the
  chart's audience is mobile.
- No PDF renderer in the browser. `prism plot --format pdf` is
  host-only; serve pre-rendered PDFs as static files if the page
  needs them.
