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
>   intentional (it's how interactive custom marks attach behavior),
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

## Worked examples: quote and stat cards

Two more complete examples, in the same style as `badge` above,
showing `custom` used for the shape of thing it's most often reached
for in practice: an HTML dashboard card. Both are `HTMLCustomRenderer`
only — a pull-quote or a metric tile is inherently HTML-shaped
(`<blockquote>`/`<footer>`, a bordered `<div>` with a large number)
with no natural non-browser SVG equivalent, unlike `badge`, which
intentionally implements both interfaces to demonstrate the dual-method
contract below. Rendered output for both lives in the gallery under
[Gallery › Custom marks](../gallery/index.md#custom-marks).

### Quote card

Renders the first row's `quote` field as a pull-quote, with `author`
(and an optional `role`) as the attribution line:

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

// quoteCard renders row 0's "quote"/"author"/"role" fields as a
// pull-quote block. It implements HTMLCustomRenderer only.
type quoteCard struct{}

func (quoteCard) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	quote, _ := rows[0]["quote"].(string)
	author, _ := rows[0]["author"].(string)
	role, _ := rows[0]["role"].(string)

	// Every interpolated field is row data, so every interpolated
	// field gets escaped — same rule as badge's RenderSVG above.
	safeQuote := html.EscapeString(quote)
	attribution := html.EscapeString(author)
	if role != "" {
		attribution = fmt.Sprintf("%s, %s", html.EscapeString(author), html.EscapeString(role))
	}

	return fmt.Sprintf(
		`<blockquote style="margin:0;max-width:%.0fpx;padding:20px 24px;`+
			`border-left:4px solid %s;background:%s;font:italic 16px/1.5 %s;color:%s">`+
			`<p style="margin:0 0 12px 0">&#8220;%s&#8221;</p>`+
			`<footer style="font-style:normal;font-weight:600;font-size:13px;color:%s">&mdash; %s</footer>`+
			`</blockquote>`,
		box.W, tokens.AxisColor, tokens.GridColor, tokens.FontSans, tokens.TextColor,
		safeQuote, tokens.AxisColor, attribution,
	), nil
}

func main() {
	if err := prism.RegisterCustomMark("quote-card", quoteCard{}); err != nil {
		panic(err)
	}
}
```

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [{
      "quote": "Design is not just what it looks like and feels like. Design is how it works.",
      "author": "Steve Jobs",
      "role": "Co-founder, Apple & NeXT"
    }]
  },
  "mark": {"type": "custom", "renderer": "quote-card"},
  "encoding": {}
}
```

Note the literal `&` in `"role"` above: it comes through the output as
`Apple &amp; NeXT`, not a raw `&` — proof the renderer escapes row
data rather than trusting it. See
[`render/html/gallery_custom_cards_test.go`](https://github.com/frankbardon/prism/blob/main/render/html/gallery_custom_cards_test.go)
for the exact tested implementation (with thousands-separator/delta
helpers factored out) and its escaping-proof unit tests.

### Stat card

Renders the first row as a dashboard-style single-metric tile: a
`label`, a large `value`, and an optional `delta` (a signed percent —
sign picks the up/down arrow and color):

```go
package main

import (
	"fmt"
	"html"
	"strconv"

	"github.com/frankbardon/prism"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// statCard renders row 0's "label"/"value"/"delta" fields as a
// metric tile. It implements HTMLCustomRenderer only.
type statCard struct{}

func (statCard) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	label, _ := rows[0]["label"].(string)
	safeLabel := html.EscapeString(label)

	value := ""
	switch v := rows[0]["value"].(type) {
	case string:
		value = v
	case float64:
		value = strconv.FormatFloat(v, 'f', -1, 64)
	}
	safeValue := html.EscapeString(value)

	var deltaHTML string
	if d, ok := rows[0]["delta"].(float64); ok {
		arrow, color := "▲", "#16a34a"
		if d < 0 {
			arrow, color = "▼", "#dc2626"
		}
		deltaHTML = fmt.Sprintf(
			`<div style="margin-top:6px;font-size:13px;font-weight:600;color:%s">%s %s</div>`,
			color, arrow, html.EscapeString(fmt.Sprintf("%.1f%%", d)),
		)
	}

	return fmt.Sprintf(
		`<div style="max-width:%.0fpx;padding:16px 20px;border:1px solid %s;border-radius:8px;`+
			`font-family:%s">`+
			`<div style="font-size:12px;text-transform:uppercase;letter-spacing:.05em;color:%s">%s</div>`+
			`<div style="margin-top:4px;font-size:28px;font-weight:700;color:%s">%s</div>%s`+
			`</div>`,
		box.W, tokens.GridColor, tokens.FontSans,
		tokens.AxisColor, safeLabel, tokens.TextColor, safeValue, deltaHTML,
	), nil
}

func main() {
	if err := prism.RegisterCustomMark("stat-card", statCard{}); err != nil {
		panic(err)
	}
}
```

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [{"label": "Monthly Active Users", "value": 128400, "delta": 4.2}]
  },
  "mark": {"type": "custom", "renderer": "stat-card"},
  "encoding": {}
}
```

Note there's no `"unit"`/currency formatting here — `value` is
rendered as-is (a real integration would pre-format it, e.g. `"128.4K"`,
before it ever reaches the row; recall Prism has
[no expression language](../concepts/spec.md), so any such formatting
is the caller's job upstream, same as everywhere else in Prism).

### Why no gallery `*.prism.json` for these two

Every other gallery category pairs a `*.prism.json` spec with output
the shared `prism` CLI binary produced (`prism plot`/`--format html`).
That binary has no renderer registered under `quote-card` or
`stat-card` — registration is the Go-level `custommark.Register` call
shown above, made by a specific process before it renders, not
something a bare JSON spec can trigger. Shipping a `*.prism.json` next
to these two `.html` files would misleadingly imply
`prism plot custom-marks/quote_card.prism.json` works out of the box;
it doesn't, and can't, without a caller-supplied binary that first
calls `RegisterCustomMark`. The two JSON blocks above are the specs
that produced the committed gallery HTML — copy one verbatim, register
the matching renderer under the name it references, and
`prism.RenderPlan` (or the equivalent CLI flow in your own binary)
produces the same output.

### Test isolation

The registry is process-global mutable state — an intentional deviation
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
- [Gallery › Custom marks](../gallery/index.md#custom-marks) — the
  rendered quote-card and stat-card fixtures from the worked examples
  above.
