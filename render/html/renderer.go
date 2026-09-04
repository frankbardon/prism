// Package html implements render.Renderer for HTML output — the
// third render backend alongside render/svg (canonical, vector) and
// render/canvas (vendored browser bridge). It mirrors render/svg's
// shape: a stateless Renderer struct, New(), MimeType(), and Render.
//
// Today's Render wraps the same SceneDoc through render/svg's own
// emitters and splices the resulting <svg> into a small standalone
// HTML document. Delegating to svg.New().Render (rather than
// re-serialising scene geometry by hand) means every coordinate
// still passes through render.FormatFloat / RenderPrecision and any
// future svg.Renderer change (theme CSS, mark emitters, precision)
// is inherited automatically — CLAUDE.md's "Pinned coordinate
// precision" convention names this as the only sanctioned path.
//
// The table mark (landing in E1-S2/E1-S3) is the reason this backend
// exists: a top-level table renders as DOM/CSS markup with no SVG
// geometry equivalent (see render/svg's PRISM_RENDER_MARK_UNSUPPORTED
// guard). This story's HTML output has no scene.Table IR to consume
// yet, so it renders every scene as an inline-SVG chart wrapped in an
// HTML shell; a later story adds a <table> branch alongside/instead
// of the inline SVG once scene.Table exists.
package html

import (
	"fmt"
	gohtml "html"
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
)

// Renderer is the HTML implementation of render.Renderer. Stateless;
// safe to share across goroutines (it only delegates to svg.Renderer,
// itself stateless).
type Renderer struct{}

// New returns the HTML renderer.
func New() *Renderer { return &Renderer{} }

// MimeType implements render.Renderer.
func (r *Renderer) MimeType() string { return "text/html" }

// defaultDocTitle is used when no cell in the scene carries a Title.
const defaultDocTitle = "Prism chart"

// Render implements render.Renderer. Produces:
//
//	<!doctype html>
//	<html>
//	<head><meta charset="utf-8"><title>...</title><style>...</style></head>
//	<body><div class="prism-html-chart">
//	<svg ...>...</svg>
//	</div></body>
//	</html>
//
// opts.Format is forced to "svg" for the embedded delegate so a
// caller passing opts.Format="html" through doesn't leak into
// svg.Renderer's own (currently unused) format-sensitive branches.
func (r *Renderer) Render(doc *scene.SceneDoc, opts render.RenderOpts) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("html.Render: nil SceneDoc")
	}
	svgOpts := opts
	svgOpts.Format = "svg"
	body, err := svg.New().Render(doc, svgOpts)
	if err != nil {
		return nil, err
	}

	title := defaultDocTitle
	for _, cell := range doc.Grid.Cells {
		if cell.Scene.Title != nil && cell.Scene.Title.Content != "" {
			title = cell.Scene.Title.Content
			break
		}
	}

	var w strings.Builder
	w.WriteString("<!doctype html>\n<html>\n<head>\n")
	w.WriteString(`<meta charset="utf-8">` + "\n")
	w.WriteString("<title>")
	w.WriteString(gohtml.EscapeString(title))
	w.WriteString("</title>\n")
	w.WriteString("<style>html,body{margin:0;padding:0}.prism-html-chart{max-width:100%}</style>\n")
	w.WriteString("</head>\n<body>\n")
	w.WriteString(`<div class="prism-html-chart">` + "\n")
	w.WriteString(string(body))
	w.WriteString("\n</div>\n</body>\n</html>\n")

	return []byte(w.String()), nil
}
