//go:build js && wasm

// js_bridge_js.go is the real half of the JS-side custom-mark
// registration seam (E2-S5). It exists because the Go-side
// custommark.Register call is compiled into bin/prism.wasm at build
// time — a browser page loading that shared, prebuilt binary has no
// way to call it (there is no "eval a Go func from JS" mechanism), so
// custom marks were only reachable from a bring-your-own TinyGo build.
// This file gives cmd/prismwasm/main.go's exported
// prism.registerCustomMark(name, fn) something to register into: a
// process-global (module-global, in WASM terms) map of name -> JS
// callback, consulted by LookupWithJSFallback only after the ordinary
// Go-side registry misses.
//
// custommark/js_bridge_other.go stubs jsLookup to always miss on every
// other build, so syscall/js never enters the host import graph.
package custommark

import (
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// jsMu guards jsRegistry, mirroring mu/registry above. A second mutex
// rather than reusing mu: the two registries are independent maps with
// independent lifetimes (Unregister never touches jsRegistry and vice
// versa), and keeping them separate means a JS-heavy test can reset one
// without disturbing the other.
var (
	jsMu       sync.RWMutex
	jsRegistry = map[string]js.Value{}
)

// RegisterJS registers fn under name in the JS-side registry —
// cmd/prismwasm/main.go's prism.registerCustomMark(name, fn) export
// calls this directly. fn must be a synchronous JS function of shape
// (rows, box) -> string: rows is a plain JS array of plain objects (one
// per table.Row) and box is a plain {w, h} object (scene.Box's own JSON
// shape); the function's return value is the fragment jsRenderer.
// RenderHTML hands back to the render backend. A Promise return is not
// awaitable from this synchronous bridge and surfaces as a type error
// at render time (mirrors setDataResolverFunc's own sync-only
// contract). Re-registering the same name replaces the prior callback.
// Safe for concurrent use.
func RegisterJS(name string, fn js.Value) error {
	if name == "" {
		return fmt.Errorf("custommark: RegisterJS: empty name")
	}
	if fn.Type() != js.TypeFunction {
		return fmt.Errorf("custommark: RegisterJS %q: fn must be a function", name)
	}
	jsMu.Lock()
	defer jsMu.Unlock()
	jsRegistry[name] = fn
	return nil
}

// UnregisterJS removes name from the JS-side registry, if present.
// Chiefly for test isolation, mirroring Unregister.
func UnregisterJS(name string) {
	jsMu.Lock()
	defer jsMu.Unlock()
	delete(jsRegistry, name)
}

// jsLookup returns a CustomRenderer bridging to the JS callback
// registered under name, if any. See LookupWithJSFallback.
func jsLookup(name string) (CustomRenderer, bool) {
	jsMu.RLock()
	fn, ok := jsRegistry[name]
	jsMu.RUnlock()
	if !ok {
		return nil, false
	}
	return jsRenderer{name: name, fn: fn}, true
}

// jsRenderer bridges a JS-registered custom-mark callback into the
// HTMLCustomRenderer contract. HTMLCustomRenderer (not SVGCustomRenderer)
// is the closest existing shape: a JS-authored custom mark is naturally
// HTML-shaped (interaction logic, including any <script> tag the
// callback returns, runs client-side only — see
// .planning/html-renderer/interview.md's "Script execution
// responsibility" decision), and both render backends already know how
// to place an HTMLCustomRenderer's output: render/html splices it
// verbatim, and render/svg falls back to its existing
// <foreignObject>-wrapped path (render/svg/custom.go) — exactly "splice
// or embed as the host Go path already does".
//
// Marshalling goes through encoding/json + JSON.parse rather than a
// hand-rolled js.ValueOf walk over table.Row's `map[string]any` values:
// js.ValueOf's type switch only matches the exact dynamic type
// map[string]interface{}, which a named map type or nested non-JSON Go
// value (e.g. a temporal column's time.Time) would miss. Routing
// through JSON — the same round-trip wasmDataResolver already uses in
// the opposite direction — guarantees every cell that can appear in a
// table.Row marshals the same way it does anywhere else in the Scene
// IR, and lands in JS as genuine plain objects/arrays, not opaque Go
// value wrappers.
type jsRenderer struct {
	name string
	fn   js.Value
}

// jsBridgePayload is the wire shape marshalled to JSON and then handed
// to JS's JSON.parse, so the callback receives real JS values (a plain
// array of plain objects for rows, {w,h} for box — scene.Box's own JSON
// tags already produce exactly that shape) rather than a JSON string it
// would have to parse itself.
type jsBridgePayload struct {
	Rows []table.Row `json:"rows"`
	Box  scene.Box   `json:"box"`
}

// RenderHTML invokes the registered JS callback as (rows, box) ->
// string and returns its result verbatim. tokens is accepted to satisfy
// HTMLCustomRenderer but unused: theme tokens are a Go-side
// reconstruction of the resolved scene theme (see
// render/svg.ThemeTokensFromScene's doc comment) that has no
// established JS-consumable shape yet: threading it across the bridge
// is future work, tracked as a follow-up rather than invented ad hoc
// here.
func (r jsRenderer) RenderHTML(rows []table.Row, box scene.Box, _ *theme.Theme) (string, error) {
	body, err := json.Marshal(jsBridgePayload{Rows: rows, Box: box})
	if err != nil {
		return "", fmt.Errorf("custom mark %q: marshal JS bridge payload: %w", r.name, err)
	}
	parsed := js.Global().Get("JSON").Call("parse", string(body))
	jsRows := parsed.Get("rows")
	jsBox := parsed.Get("box")

	res := r.fn.Invoke(jsRows, jsBox)
	if res.Type() != js.TypeString {
		return "", fmt.Errorf("custom mark %q: JS renderer callback must return a string, got %s", r.name, res.Type().String())
	}
	return res.String(), nil
}
