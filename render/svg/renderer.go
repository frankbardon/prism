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

	// Compute viewBox. For 1×1 grids (the P05–P07 default) the math
	// collapses to the first (and only) cell's Frame — byte-identical
	// to the pre-P08 path. For N×M grids we take the max of all cell
	// frames so the SVG spans every cell + any shared axis on the
	// outer edge.
	frame := outerFrame(doc.Grid)
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

	// Walk grid cells in row-major order. Each cell's Scene already
	// carries pre-offset coordinates from EncodeComposite — the
	// renderer never has to wrap cells in a <g transform=> group.
	for _, cell := range doc.Grid.Cells {
		renderScene(w, cell.Scene)
	}

	// Shared axes (D051): emit once at the grid edge, outside any
	// cell. Skipped when nil; common for 1×1 grids (per-cell axes
	// suffice with one cell).
	renderSharedAxes(w, doc.Grid)

	// Facet headers (P09 / T09.07): grid-edge row + column labels.
	// Repeat does NOT emit headers (the substituted field name is
	// implicit in each cell's axis title); the encoder leaves
	// Layout.Headers zero for repeat so this block is a no-op there.
	renderGridHeaders(w, doc.Grid)

	w.EndTag("svg")
	w.Newline()
	return w.Bytes(), nil
}

// outerFrame computes the bounding rectangle covering every cell in
// the grid. For 1×1 grids it returns the single cell's frame
// unchanged (preserving P05–P07 SVG goldens byte-for-byte). For N×M
// grids it spans from (0,0) to (max-right, max-bottom).
func outerFrame(g scene.SceneGrid) scene.Rect {
	if len(g.Cells) == 0 {
		return scene.Rect{}
	}
	if len(g.Cells) == 1 {
		return g.Cells[0].Scene.Frame
	}
	var maxX, maxY float64
	for _, c := range g.Cells {
		right := c.Scene.Frame.X + c.Scene.Frame.W
		bottom := c.Scene.Frame.Y + c.Scene.Frame.H
		if right > maxX {
			maxX = right
		}
		if bottom > maxY {
			maxY = bottom
		}
	}
	return scene.Rect{W: maxX, H: maxY}
}

// renderSharedAxes emits Grid.Shared.X and Grid.Shared.Y once outside
// the cell loop. Each axis is wrapped in its own prism-axes group so
// the structural class is consistent with per-cell axes, but with an
// extra data-shared="true" attribute for diagnostic + test scraping.
func renderSharedAxes(w *Writer, g scene.SceneGrid) {
	if g.Shared.X == nil && g.Shared.Y == nil {
		return
	}
	// Approximate the plot rect that anchors the shared axis. For
	// 1×1 grids the cell's plot rect is fine; for multi-cell grids
	// the axis was built against the single-cell plot so spans the
	// expected width / height per its position.
	plot := scene.Rect{}
	if len(g.Cells) > 0 {
		plot = g.Cells[0].Scene.Plot
	}
	w.Newline()
	w.Indent(2)
	w.OpenTag("g")
	w.Attr("class", "prism-axes")
	w.Attr("data-shared", "true")
	w.CloseTagOpen()
	w.Newline()
	if g.Shared.X != nil {
		w.Indent(4)
		renderAxis(w, *g.Shared.X, plot)
		w.Newline()
	}
	if g.Shared.Y != nil {
		w.Indent(4)
		renderAxis(w, *g.Shared.Y, plot)
		w.Newline()
	}
	w.Indent(2)
	w.EndTag("g")
	w.Newline()
}

// renderGridHeaders emits facet row + column header labels at the
// grid edge. Headers live in their own `<g class="prism-grid-headers">`
// block so test scrapers can locate them deterministically. The
// helper is a no-op when both Top and Left header slices are empty
// (the standard concat / layer / repeat case).
//
// Layout:
//   - Top labels: centered above each cell column at y = 14.
//   - Left labels: anchored at x = 6 (left-text-anchor) on each
//     cell row's vertical midpoint.
//
// The encoder reserves space for these via Layout.headerLeft /
// headerTop offsets when populating cells (encode_facet.go), so the
// labels do not overlap cell content.
func renderGridHeaders(w *Writer, g scene.SceneGrid) {
	if len(g.Layout.Headers.Top) == 0 && len(g.Layout.Headers.Left) == 0 {
		return
	}
	if len(g.Cells) == 0 {
		return
	}
	w.Newline()
	w.Indent(2)
	w.OpenTag("g")
	w.Attr("class", "prism-grid-headers")
	w.CloseTagOpen()
	w.Newline()

	// Column headers along the top edge. Use the first cell in each
	// column to anchor the x coordinate (cells share x within a
	// column).
	if labels := g.Layout.Headers.Top; len(labels) > 0 {
		// Build a quick map: col index → cell.Scene.Frame.X + W/2.
		colCenters := map[int]float64{}
		for _, c := range g.Cells {
			if _, ok := colCenters[c.Col]; !ok {
				colCenters[c.Col] = c.Scene.Frame.X + c.Scene.Frame.W/2
			}
		}
		for ci, label := range labels {
			cx, ok := colCenters[ci]
			if !ok {
				continue
			}
			w.Indent(4)
			w.OpenTag("text")
			w.Attr("class", "prism-facet-header prism-facet-header-top")
			w.AttrFloat("x", cx)
			w.AttrFloat("y", 14)
			w.Attr("text-anchor", "middle")
			w.CloseTagOpen()
			w.Text(label)
			w.EndTag("text")
			w.Newline()
		}
	}
	// Row headers along the left edge.
	if labels := g.Layout.Headers.Left; len(labels) > 0 {
		rowCenters := map[int]float64{}
		for _, c := range g.Cells {
			if _, ok := rowCenters[c.Row]; !ok {
				rowCenters[c.Row] = c.Scene.Frame.Y + c.Scene.Frame.H/2
			}
		}
		for ri, label := range labels {
			cy, ok := rowCenters[ri]
			if !ok {
				continue
			}
			w.Indent(4)
			w.OpenTag("text")
			w.Attr("class", "prism-facet-header prism-facet-header-left")
			w.AttrFloat("x", 6)
			w.AttrFloat("y", cy)
			w.Attr("text-anchor", "start")
			w.CloseTagOpen()
			w.Text(label)
			w.EndTag("text")
			w.Newline()
		}
	}
	w.Indent(2)
	w.EndTag("g")
	w.Newline()
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
		// Hit-test scope per D077: always emit data-prism-layer (empty
		// string when ID unset) so selector queries stay stable across
		// the Go + JS ports.
		w.Attr("data-prism-layer", layer.ID)
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
