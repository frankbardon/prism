// Package render carries the cross-renderer surface: the Renderer
// interface every output backend satisfies, the RenderOpts shape, and
// the RenderPrecision constant that pins float formatting across the
// SVG (P05), PNG (P12), PDF (P15), and canvas-JSON (P12) backends.
//
// Concrete renderers live in subpackages (render/svg/, render/pdf/,
// render/canvas/). All three consume the same scene.SceneDoc shape.
package render

import "github.com/frankbardon/prism/encode/scene"

// Renderer is the contract every output-format backend satisfies. The
// SVG impl ships in P05; PNG (via resvg-go) lands in P12, PDF in
// P15, canvas-json in P12.
type Renderer interface {
	// Render produces output bytes for the SceneDoc + opts. The
	// returned bytes are renderer-format-specific; callers route them
	// through MimeType() for HTTP / display contexts.
	Render(doc *scene.SceneDoc, opts RenderOpts) ([]byte, error)
	// MimeType returns the canonical MIME type for the renderer's
	// output (image/svg+xml, image/png, application/pdf, etc.).
	MimeType() string
}

// RenderOpts carries the runtime knobs every renderer accepts. Zero
// values are sensible: 0 dimensions = use the scene's natural
// width/height; nil Theme = use the scene's Theme.
type RenderOpts struct {
	// Format is the requested output format ("svg" | "png" | "pdf" |
	// "canvas-json"). The CLI rejects formats the runtime cannot
	// produce with PRISM_RENDER_FORMAT_UNAVAILABLE.
	Format string
	// Width overrides the scene's natural width. 0 = use the scene's.
	Width float64
	// Height overrides the scene's natural height. 0 = use the scene's.
	Height float64
	// Theme overrides SceneDoc.Theme. nil = use the doc's.
	Theme *scene.Theme
	// Background overrides the theme's background ("transparent",
	// "#fff", "rgba(0,0,0,0.5)", etc.). "" = use the theme's.
	Background string
	// Paginate is PDF-only (P15). One chart per page.
	Paginate bool
	// DPI is PNG-only (P12).
	DPI float64
}
