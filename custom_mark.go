package prism

import "github.com/frankbardon/prism/custommark"

// SVGCustomRenderer, HTMLCustomRenderer, CustomRenderer, and the
// Register/Lookup/Names functions below are thin re-exports of
// github.com/frankbardon/prism/custommark (E2-S2 moved the actual
// registry there — see that package's doc comment for why: render/svg
// must resolve a registered renderer at render time regardless of
// caller, and this root package already imports render/svg, so a
// registry housed here would make render/svg -> prism an import
// cycle). Type aliases keep the public API a consumer imports
// unchanged.

// SVGCustomRenderer is implemented by a `custom` mark (E2) that can
// render itself as a raw SVG fragment. See custommark.SVGCustomRenderer.
type SVGCustomRenderer = custommark.SVGCustomRenderer

// HTMLCustomRenderer is implemented by a `custom` mark (E2) that can
// render itself as an HTML fragment. See custommark.HTMLCustomRenderer.
type HTMLCustomRenderer = custommark.HTMLCustomRenderer

// CustomRenderer is the registration-time constraint for a `custom`
// mark implementation. See custommark.CustomRenderer.
type CustomRenderer = custommark.CustomRenderer

// RegisterCustomMark registers renderer under name for spec
// references of the form {"type": "custom", "renderer": "<name>"}.
// See custommark.Register.
func RegisterCustomMark(name string, renderer CustomRenderer) error {
	return custommark.Register(name, renderer)
}

// LookupCustomMark returns the renderer registered under name, and
// true if one is registered. See custommark.Lookup.
func LookupCustomMark(name string) (CustomRenderer, bool) {
	return custommark.Lookup(name)
}

// CustomMarkNames returns the registered custom-mark names in sorted
// order. See custommark.Names.
func CustomMarkNames() []string {
	return custommark.Names()
}
