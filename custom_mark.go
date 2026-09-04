package prism

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
// tree with no wrapper (E2-S3). An implementation is responsible for
// escaping any row data it interpolates into the fragment — Prism
// places the returned string verbatim.
type SVGCustomRenderer interface {
	RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error)
}

// HTMLCustomRenderer is implemented by a `custom` mark (E2) that can
// render itself as an HTML fragment. Used directly by the HTML render
// backend, and as a <foreignObject>-wrapped fallback under the SVG
// backend when a renderer implements only this method (E2-S3).
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
// required methods. RegisterCustomMark rejects a value implementing
// neither. Call sites that need to invoke a registered renderer type-
// assert to the interface matching the active backend, e.g.:
//
//	if r, ok := renderer.(prism.SVGCustomRenderer); ok {
//	    frag, err := r.RenderSVG(rows, box, tokens)
//	}
type CustomRenderer any

// customMarkMu guards customMarkRegistry. This registry is an
// intentional, explicitly-accepted deviation from the rest of the
// codebase's hermetic/no-global-mutable-state convention (validate/
// resolve/plan/compile all thread dependencies explicitly) — chosen
// for simpler consumer call sites: a downstream application registers
// its custom marks once (e.g. from its own init/main), then compiles
// and renders specs that reference them by name, with no registry
// object to thread through every call. See
// .planning/html-renderer/interview.md's Risks section. A registry
// reset/test-isolation helper for downstream test suites lands in
// E2-S4 — this story only guarantees concurrency-safe access.
var (
	customMarkMu       sync.RWMutex
	customMarkRegistry = map[string]CustomRenderer{}
)

// RegisterCustomMark registers renderer under name for spec
// references of the form {"type": "custom", "renderer": "<name>"}
// (E2-S2 wires the lookup into encode/render). A consuming
// application calls this before compiling/rendering any spec that
// references name — typically once, from its own init or main.
// Re-registering the same name replaces the prior renderer. Safe for
// concurrent use.
//
// renderer must implement at least one of SVGCustomRenderer or
// HTMLCustomRenderer; otherwise RegisterCustomMark returns an error
// and leaves the registry unchanged.
func RegisterCustomMark(name string, renderer CustomRenderer) error {
	if name == "" {
		return fmt.Errorf("prism: RegisterCustomMark: empty name")
	}
	if renderer == nil {
		return fmt.Errorf("prism: RegisterCustomMark %q: nil renderer", name)
	}
	_, svgOK := renderer.(SVGCustomRenderer)
	_, htmlOK := renderer.(HTMLCustomRenderer)
	if !svgOK && !htmlOK {
		return fmt.Errorf("prism: RegisterCustomMark %q: renderer implements neither SVGCustomRenderer nor HTMLCustomRenderer", name)
	}

	customMarkMu.Lock()
	defer customMarkMu.Unlock()
	customMarkRegistry[name] = renderer
	return nil
}

// LookupCustomMark returns the renderer registered under name, and
// true if one is registered. Safe for concurrent use.
func LookupCustomMark(name string) (CustomRenderer, bool) {
	customMarkMu.RLock()
	defer customMarkMu.RUnlock()
	r, ok := customMarkRegistry[name]
	return r, ok
}

// CustomMarkNames returns the registered custom-mark names in sorted
// order. Mirrors theme.Names().
func CustomMarkNames() []string {
	customMarkMu.RLock()
	defer customMarkMu.RUnlock()
	out := make([]string, 0, len(customMarkRegistry))
	for name := range customMarkRegistry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
