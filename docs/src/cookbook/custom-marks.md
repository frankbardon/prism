# Cookbook: custom marks

A `custom` mark is the escape hatch for a visualization Prism's
built-in marks can't express — a consuming application registers its
own render function under a name, and a spec references that name.
Prism resolves the name at render time and splices the function's
output into the document.

> ## ⚠️ Security: you own escaping and script execution
>
> Prism's contract for a `custom` mark is **verbatim insertion, and
> nothing more**. Whatever string your `RenderSVG` / `RenderHTML` /
> JS callback returns is spliced into the SVG or HTML document
> byte-for-byte — no escaping, no tag stripping, no sanitization.
>
> - **You escape row data.** If your renderer interpolates a field
>   value into markup (`fmt.Sprintf("<div>%s</div>", row["name"])`),
>   *you* are responsible for escaping it (`html.EscapeString` in Go,
>   a manual escaper or safe DOM APIs in JS). An unescaped value from
>   an untrusted data source is a real XSS hole — Prism will not catch
>   it for you.
> - **You own script execution.** A `<script>` tag your renderer
>   returns is inserted verbatim and the browser executes it normally
>   — under the `html` render backend directly, and under WASM's
>   `<prism-chart>` element once its markup mounts. This is
>   deliberate (it's how interactive custom marks attach behavior),
>   but it means a `custom` mark is exactly as trustworthy as the code
>   you registered under its name. Never register a renderer that
>   builds markup from data you don't control without escaping it
>   first, and treat `mark.renderer` name resolution as you would any
>   other plugin-loading surface.
>
> This is not a Prism limitation to be fixed later — it is the
> design. A `custom` mark is arbitrary caller code; Prism's job stops
> at "call it and place what it returns."

## Registering a renderer (Go)

Implement at least one of two single-method interfaces from the root
`prism` package (thin re-exports of `github.com/frankbardon/prism/custommark`,
which is where the real registry and interfaces live):

```go
// SVGCustomRenderer — raw SVG fragment.
type SVGCustomRenderer interface {
    RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error)
}

// HTMLCustomRenderer — HTML fragment (scripts execute verbatim).
type HTMLCustomRenderer interface {
    RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error)
}
```

A renderer may implement one or both — "optional" is expressed by
having two single-method interfaces rather than one interface with
two required methods. `rows` is the mark's resolved,
upstream-filtered/sorted/limited data (one `table.Row` — a
`map[string]any` — per row); `box` carries the content area's `W`/`H`
in pixels (a custom mark is freeform/document-flow and never resolves
a position, so there's no `X`/`Y`); `tokens` is the active theme, so
freeform output can stay visually consistent across light/dark/print.

Register the implementation once, before compiling/rendering any spec
that references it (typically from your program's own `init` or
`main`):

```go
package main

import (
	"fmt"
	"html"

	"github.com/frankbardon/prism"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// badge renders the first row's "label" field as a small pill. It
// implements SVGCustomRenderer only.
type badge struct{}

func (badge) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	label := ""
	if len(rows) > 0 {
		if v, ok := rows[0]["label"].(string); ok {
			label = v
		}
	}
	// html.EscapeString is not SVG-specific, but it neutralizes the
	// same &<>"' characters that would otherwise break out of the
	// <text> element or attribute value below. Escaping row data is
	// this renderer's job, not Prism's — see the security note above.
	safe := html.EscapeString(label)
	return fmt.Sprintf(
		`<rect width="%.0f" height="%.0f" rx="6" fill="%s"/><text x="8" y="18" fill="white">%s</text>`,
		box.W, box.H, tokens.AxisColor, safe,
	), nil
}

func main() {
	if err := prism.RegisterCustomMark("badge", badge{}); err != nil {
		panic(err)
	}
	// ...compile/render specs referencing {"mark": {"type": "custom", "renderer": "badge"}}
}
```

Reference the registered name from a spec — `renderer` is always a
plain string key, never executable code; the spec JSON never carries
the implementation, only the name it was registered under (this
preserves Prism's no-expression-language invariant just as `filter` /
`calculate` do):

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"label": "Beta"}]},
  "mark": {"type": "custom", "renderer": "badge"},
  "encoding": {}
}
```

An unregistered `renderer` name fails at render time (not decode time)
with `PRISM_RENDER_CUSTOM_MARK_NOT_FOUND`, naming the requested
renderer and listing every currently-registered name. Re-registering
the same name replaces the prior renderer; `custommark.Register` is
safe for concurrent use.

### Test isolation

The registry is process-global mutable state — a deliberate deviation
from the rest of Prism's hermetic, dependency-threaded convention (see
`custommark`'s package doc comment), accepted for simpler call sites.
If your own test suite registers a custom mark, call
`custommark.ResetForTest(t)` at the top of the test: it snapshots the
current registry, clears it for the test, and restores the snapshot
via `t.Cleanup` — so a registration made by one test can never bleed
into a sibling test that runs later in the same binary.

## The SVG / HTML dual-method contract

A `custom` mark can render under **either** Go backend
(`render/svg` or `render/html` — see [Themes: Rendering
backends](../concepts/themes.md#rendering-backends)) regardless of
which method(s) its renderer implements. Each backend prefers its
*native* method when both are implemented, and falls back to
wrapping/re-encoding the other when only one is:

| Renderer implements | Under the `svg` backend | Under the `html` backend |
|---|---|---|
| `SVGCustomRenderer` only | Spliced directly into the SVG tree — no wrapper. | Re-encoded as a standalone `<svg>...</svg>` fragment and inserted inline. |
| `HTMLCustomRenderer` only | Wrapped in `<foreignObject>` (an inner XHTML-namespaced `<div>`, since a standalone SVG document is parsed as XML and a bare HTML fragment isn't well-formed in the SVG namespace). | Spliced directly into a wrapping `<div>` — verbatim, including any `<script>` tag, which the browser executes normally. |
| Both | `SVGCustomRenderer` preferred (direct splice). | `HTMLCustomRenderer` preferred (verbatim insertion). |
| Neither | Rejected by `custommark.Register` before it ever reaches the registry. | Same. |

That gives four concrete combinations:

1. **SVG-only, under `svg`** — the cheapest path: your fragment
   becomes part of the SVG document with no wrapper at all.
2. **SVG-only, under `html`** — your fragment is still pure SVG
   markup, so the HTML backend re-encodes it as an inline
   `<svg>...</svg>` element sized to `box` and inserts that.
3. **HTML-only, under `svg`** — `<foreignObject>` is how SVG embeds
   foreign markup. This works in browsers, but has real portability
   limits: some non-browser SVG viewers and print pipelines don't
   support `<foreignObject>`. Prefer implementing `RenderSVG` too if
   your custom mark needs to work as a standalone SVG file outside a
   browser.
4. **HTML-only, under `html`** — the native case: your HTML (and any
   `<script>` it contains) lands in the document exactly as returned.

In every case, Prism positions the fragment (translating it to the
mark's plot origin) but never inspects, escapes, or transforms its
contents — see the security note at the top of this page.

## Registering a renderer from JS (WASM)

The Go-side registry above is compiled into `bin/prism.wasm` at build
time — a browser page loading that shared, prebuilt binary has no way
to call a Go function to register into it. `prism.registerCustomMark`
is the parallel path a browser page *can* use, with no rebuild:

```js
prism.registerCustomMark("badge", (rows, box) => {
  const label = rows.length > 0 ? String(rows[0].label ?? "") : "";
  // Same rule as the Go example: escaping is the renderer's job.
  const safe = label.replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[c]);
  return `<div style="width:${box.w}px;height:${box.h}px;` +
         `background:#0f172a;color:#fff;border-radius:6px;` +
         `display:flex;align-items:center;padding:0 8px">${safe}</div>`;
});
```

```html
<prism-chart spec='{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"label": "Beta"}]},
  "mark": {"type": "custom", "renderer": "badge"},
  "encoding": {}
}'></prism-chart>
```

`fn` must be a **synchronous** function of shape `(rows, box) ->
string` — a `Promise` return isn't awaitable across the WASM bridge
and surfaces as a type error at render time. `rows` is a plain JS
array of plain objects (one per data row — the same JSON shape as any
other Scene IR data); `box` is a plain `{w, h}` object.

A JS-registered renderer bridges only to the `HTMLCustomRenderer`
contract — there is no raw-SVG-fragment JS renderer today. That means
a JS-registered custom mark always takes the **HTML-only** row of the
table above: spliced verbatim under the `html` backend, and
`<foreignObject>`-wrapped under the `svg` backend. If you need a
JS-driven custom mark to render as pure SVG (portable to non-browser
viewers), implement it in Go (`SVGCustomRenderer`) and compile it into
your own WASM build instead.

`custommark.RegisterJS`/`Lookup` resolve through
`LookupWithJSFallback`: a `custom` mark's renderer name is looked up
against the Go-side registry first, then — only in the WASM build —
against this JS-side registry. That means a name registered only from
the browser resolves correctly with no Go-side
`custommark.Register` call at all; it's what makes `custom` marks
usable against the shared prebuilt `bin/prism.wasm` runtime rather
than requiring a bring-your-own TinyGo build.

## Errors

| Code | When |
|---|---|
| `PRISM_RENDER_CUSTOM_MARK_NOT_FOUND` | The spec's `mark.renderer` name has no matching entry in the registry at render time (Go or JS side), or — defensively — matches an entry that implements neither interface (unreachable through `Register`'s own public API, which already rejects such a value). |

Look up any code's full message and fixups with `prism errors lookup
PRISM_RENDER_CUSTOM_MARK_NOT_FOUND`.

## See also

- [Marks: Custom](../concepts/marks.md#custom) — the `custom` mark's
  spec shape and validate rules.
- [Themes: Rendering backends](../concepts/themes.md#rendering-backends)
  — how `svg` vs `html` are selected.
- [Browser / WASM](../concepts/browser.md) — the wider WASM bridge
  `prism.registerCustomMark` is part of.
