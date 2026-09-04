// Package custommark holds the registration surface for a `custom`
// mark (E2): the SVGCustomRenderer / HTMLCustomRenderer interfaces a
// downstream consumer implements, and the process-global registry
// mapping a spec's mark_def.renderer name to a concrete
// implementation.
//
// This started life in the root prism package (E2-S1) but moved here
// in E2-S2: the SVG render backend (render/svg) must resolve a
// registered renderer at render time regardless of who calls it (the
// root prism package's Compile/RenderPlan convenience wrapper, or a
// consumer that calls render/svg directly, as cmd/prism, rpc/, and
// mcp/ all do). The root prism package already imports render/svg,
// so housing the registry there would make render/svg -> prism an
// import cycle. The root package re-exports every name here via type
// aliases and thin wrapper functions (prism.RegisterCustomMark,
// prism.SVGCustomRenderer, ...) so a consumer's public API is
// unchanged.
package custommark

import (
	"fmt"
	"sort"
	"sync"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// SVGCustomRenderer is implemented by a `custom` mark (E2) that can
// render itself as a raw SVG fragment. Rows is the resolved,
// upstream-filtered/sorted/limited table's row-oriented view; Box is
// the content area the SVG backend allots to this mark (custom marks
// are freeform/document-flow and never resolve a shared x/y scale, so
// Box carries dimensions only, no position); tokens is the active
// theme, resolved the same way any other mark's style resolves, so
// freeform output can stay visually consistent across light/dark/
// print themes.
//
// The SVG backend splices the returned fragment directly into the SVG
// tree with no wrapper (E2-S2). An implementation is responsible for
// escaping any row data it interpolates into the fragment — Prism
// places the returned string verbatim.
type SVGCustomRenderer interface {
	RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error)
}

// HTMLCustomRenderer is implemented by a `custom` mark (E2) that can
// render itself as an HTML fragment. Used directly by the HTML render
// backend, and as a <foreignObject>-wrapped fallback under the SVG
// backend when a renderer implements only this method (E2-S2).
//
// Prism's contract is limited to placing the returned fragment
// verbatim in the DOM (no tag stripping), so a caller-authored
// <script> tag is picked up and executed by the browser normally.
// Wiring interaction logic beyond that — and escaping any row data
// interpolated into the fragment — is the implementation's
// responsibility, not Prism's.
type HTMLCustomRenderer interface {
	RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error)
}

// CustomRenderer is the registration-time constraint for a `custom`
// mark implementation. It intentionally declares no methods of its
// own: a concrete renderer may implement SVGCustomRenderer,
// HTMLCustomRenderer, or both — "optional" is expressed by having two
// single-method interfaces rather than one interface with two
// required methods. Register rejects a value implementing neither.
// Call sites that need to invoke a registered renderer type-assert to
// the interface matching the active backend, e.g.:
//
//	if r, ok := renderer.(custommark.SVGCustomRenderer); ok {
//	    frag, err := r.RenderSVG(rows, box, tokens)
//	}
type CustomRenderer any

// mu guards registry. This registry is an intentional, explicitly-
// accepted deviation from the rest of the codebase's hermetic/no-
// global-mutable-state convention (validate/resolve/plan/compile all
// thread dependencies explicitly) — chosen for simpler consumer call
// sites: a downstream application registers its custom marks once
// (e.g. from its own init/main), then compiles and renders specs that
// reference them by name, with no registry object to thread through
// every call. See .planning/html-renderer/interview.md's Risks
// section. A fuller registry reset/test-isolation helper for
// downstream test suites lands in E2-S4 — Unregister below only
// covers this package's (and the root prism package's) own tests.
var (
	mu       sync.RWMutex
	registry = map[string]CustomRenderer{}
)

// Register registers renderer under name for spec references of the
// form {"type": "custom", "renderer": "<name>"} (E2-S2 wires the
// lookup into render/svg). A consuming application calls this before
// compiling/rendering any spec that references name — typically once,
// from its own init or main. Re-registering the same name replaces
// the prior renderer. Safe for concurrent use.
//
// renderer must implement at least one of SVGCustomRenderer or
// HTMLCustomRenderer; otherwise Register returns an error and leaves
// the registry unchanged.
func Register(name string, renderer CustomRenderer) error {
	if name == "" {
		return fmt.Errorf("custommark: Register: empty name")
	}
	if renderer == nil {
		return fmt.Errorf("custommark: Register %q: nil renderer", name)
	}
	_, svgOK := renderer.(SVGCustomRenderer)
	_, htmlOK := renderer.(HTMLCustomRenderer)
	if !svgOK && !htmlOK {
		return fmt.Errorf("custommark: Register %q: renderer implements neither SVGCustomRenderer nor HTMLCustomRenderer", name)
	}

	mu.Lock()
	defer mu.Unlock()
	registry[name] = renderer
	return nil
}

// Lookup returns the renderer registered under name, and true if one
// is registered. Safe for concurrent use.
func Lookup(name string) (CustomRenderer, bool) {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := registry[name]
	return r, ok
}

// Names returns the registered custom-mark names in sorted order.
// Mirrors theme.Names().
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Unregister removes name from the registry, if present. Exists
// chiefly for test isolation (a fuller reset/test-isolation helper is
// E2-S4's job); safe for concurrent use.
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(registry, name)
}
