package svg

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// Renderer is the SVG implementation of render.Renderer. Stateless;
// safe to share across goroutines.
type Renderer struct{}

// New returns the SVG renderer.
func New() *Renderer { return &Renderer{} }

// MimeType implements render.Renderer.
func (r *Renderer) MimeType() string { return "image/svg+xml" }

// Render implements render.Renderer. Walks the SceneDoc top-down:
//
//	<svg viewBox="0 0 W H" width="..." height="...">
//	  <style>:root{ --prism-color-axis: ...; ... }
//	          .prism-axis-label { ... } ...</style>
//	  <defs>...</defs>
//	  <g class="prism-scene" data-scene-id="...">
//	    <text class="prism-title">...</text>
//	    <g class="prism-axes">...</g>
//	    <g class="prism-plot"><g class="prism-layer">...</g></g>
//	  </g>
//	</svg>
//
// Coordinates are pinned to RenderPrecision; attributes are
// XML-escaped via the writer helpers; layer-by-layer dispatch hands
// each mark to the per-geom renderer in marks.go.
func (r *Renderer) Render(doc *scene.SceneDoc, opts render.RenderOpts) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("svg.Render: nil SceneDoc")
	}
	theme := opts.Theme
	if theme == nil {
		theme = doc.Theme
	}
	if theme == nil {
		theme = scene.Default()
	}

	w := NewWriter()

	// Compute viewBox from the first (and only, in P05) cell's Frame.
	var frame scene.Rect
	if len(doc.Grid.Cells) > 0 {
		frame = doc.Grid.Cells[0].Scene.Frame
	}
	if frame.W == 0 || frame.H == 0 {
		// Defensive fallback.
		frame = scene.Rect{W: 800, H: 600}
	}

	// Use opts.Width/Height when supplied; otherwise the frame's
	// natural dimensions. The viewBox always matches the frame so
	// scaling is uniform.
	width := opts.Width
	if width == 0 {
		width = frame.W
	}
	height := opts.Height
	if height == 0 {
		height = frame.H
	}

	w.OpenTag("svg")
	w.Attr("xmlns", "http://www.w3.org/2000/svg")
	w.OpenAttr("viewBox")
	w.Raw("0 0 ")
	w.Raw(render.FormatFloat(frame.W))
	w.Raw(" ")
	w.Raw(render.FormatFloat(frame.H))
	w.CloseAttr()
	w.AttrFloat("width", width)
	w.AttrFloat("height", height)
	w.CloseTagOpen()
	w.Newline()

	// Style block + defs.
	writeStyleBlock(w, theme)

	// Walk grid cells.
	for _, cell := range doc.Grid.Cells {
		renderScene(w, cell.Scene)
	}

	w.EndTag("svg")
	w.Newline()
	return w.Bytes(), nil
}

// renderScene emits the per-Scene structural tree.
func renderScene(w *Writer, s scene.Scene) {
	w.Newline()
	w.Indent(2)
	w.OpenTag("g")
	w.Attr("class", "prism-scene")
	if s.ID != "" {
		w.Attr("data-scene-id", s.ID)
	}
	w.CloseTagOpen()
	w.Newline()

	// Title.
	if s.Title != nil {
		w.Indent(4)
		w.OpenTag("text")
		w.Attr("class", "prism-title")
		w.AttrFloat("x", s.Title.X)
		w.AttrFloat("y", s.Title.Y)
		w.Attr("text-anchor", "middle")
		w.CloseTagOpen()
		w.Text(s.Title.Content)
		w.EndTag("text")
		w.Newline()
	}

	// Axes group.
	if len(s.Axes) > 0 {
		w.Indent(4)
		w.OpenTag("g")
		w.Attr("class", "prism-axes")
		w.CloseTagOpen()
		w.Newline()
		for _, a := range s.Axes {
			w.Indent(6)
			renderAxis(w, a, s.Plot)
			w.Newline()
		}
		w.Indent(4)
		w.EndTag("g")
		w.Newline()
	}

	// Scene-level defs (gradients used by legends + marks).
	if s.Defs != nil {
		w.Indent(4)
		renderDefs(w, s.Defs)
		w.Newline()
	}

	// Plot group with layered marks.
	w.Indent(4)
	w.OpenTag("g")
	w.Attr("class", "prism-plot")
	w.CloseTagOpen()
	w.Newline()
	for _, layer := range s.Layers {
		if layer.Hidden {
			continue
		}
		w.Indent(6)
		w.OpenTag("g")
		w.Attr("class", "prism-layer")
		if layer.ID != "" {
			w.Attr("data-layer-id", layer.ID)
		}
		w.CloseTagOpen()
		w.Newline()
		for _, m := range layer.Marks {
			w.Indent(8)
			renderMark(w, m)
			w.Newline()
		}
		w.Indent(6)
		w.EndTag("g")
		w.Newline()
	}
	w.Indent(4)
	w.EndTag("g")
	w.Newline()

	// Legends group.
	if len(s.Legends) > 0 {
		w.Indent(4)
		renderLegends(w, s.Legends)
		w.Newline()
	}

	w.Indent(2)
	w.EndTag("g")
	w.Newline()
}
