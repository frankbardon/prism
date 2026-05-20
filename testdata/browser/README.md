# Browser smoke

`demo.html` is the literal PHASE.md demoable artifact for P12. It
loads `<prism-chart>` against the same Scene JSON fixtures the
cross-impl parity harness gates on (`testdata/cross_impl/<fixture>/scene.json`)
so what you see in the browser is exactly what the byte-diff test
sees from the JS port.

`selections-demo.html` is the P13 demoable artifact. It exercises
the four mandatory selection acceptance criteria: point click,
interval brush, cross-chart broadcast via `<prism-coordinator>`,
and URL hash state. Scene JSON inputs live under
`selections-scenes/` (regenerate via the helper in
`testdata/specs/selections/README.md`).

## Running locally

From the repo root:

```
python3 -m http.server 8080
```

Then open <http://localhost:8080/testdata/browser/demo.html>
or <http://localhost:8080/testdata/browser/selections-demo.html>.

You should see five charts (bar / line / layered / pie /
sankey) plus three more using a shared dataset registry.

DevTools → Network: open the page and confirm:

- One fetch per unique Scene JSON URL (the `<prism-dataset>` +
  three-chart block at the bottom should produce **two** fetches
  total for `bar_basic` + `line_basic`, not five — that's the
  D074 dedupe contract in action).
- Open any `<prism-chart>` in the Elements panel → expand the
  shadow root → confirm the `<svg>` tree mirrors what the Go
  renderer would produce (verified by `TestCrossImplSVGParity`).

## CLI alternative

If you don't want a static server, you can render each fixture
straight to SVG via the Go renderer and view it in a browser
without any JS at all:

```
bin/prism plot testdata/specs/bar_basic.json > /tmp/bar.svg
open /tmp/bar.svg
```

This is the Go side of the parity contract; the browser path is
what proves the contract holds for the JS port too.

## Refreshing fixtures

If you change a fixture spec or the Scene IR, refresh both the
committed `scene.json` + `go.svg` inputs the demo references:

```
PRISM_CROSS_IMPL=1 PRISM_CROSS_IMPL_REGEN=1 \
  go test ./internal/devtools/... -run TestCrossImplSVGParity
```

Then commit the refreshed inputs. The demo page picks them up on
the next reload.
