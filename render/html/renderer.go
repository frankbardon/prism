// Package html implements render.Renderer for HTML output — the
// third render backend alongside render/svg (canonical, vector) and
// render/canvas (vendored browser bridge). It mirrors render/svg's
// shape: a stateless Renderer struct, New(), MimeType(), and Render.
//
// Non-table scenes wrap the same SceneDoc through render/svg's own
// emitters and splice the resulting <svg> into a small standalone
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
// guard). E1-S4 adds that <table> path: when a cell's Scene.Table is
// populated, Render emits a semantic <table> (header row from
// TableColumn.Header, body rows from TableRow.Values) instead of
// delegating the whole doc to svg.Render (which would reject the
// table-tagged layer via checkMarkSupport). A column bound to a
// sub-mark (e.g. "sparkline") still reuses render/svg for that one
// cell: its TableCell.Marks are re-wrapped in a small standalone
// SceneDoc sized to scene.TableCellSparkWidth/Height (the same
// coordinate space encode/table.go resolved the sub-mark's scales
// against) and rendered via svg.New().Render, so the inline <svg>
// spliced into the <td> inherits the exact same emitters/precision
// path as any top-level chart.
//
// A custom mark (E2-S1/E2-S3, custom.go) gets the same "no SVG
// geometry of its own" treatment as a table: when a cell's
// Scene.Custom is populated, Render resolves the registered
// custommark.CustomRenderer and emits its output directly instead of
// delegating the whole doc to svg.Render. Unlike the SVG backend
// (render/svg/custom.go), which prefers a splice-able SVGCustom-
// Renderer and falls back to a <foreignObject>-wrapped HTMLCustom-
// Renderer, this native HTML host prefers HTMLCustomRenderer — its
// output lands verbatim in the document (any author <script> tag
// executes normally) — and falls back to an inline <svg> fragment
// (via svg.New().Render, the same sub-mark-embedding mechanism above)
// when only SVGCustomRenderer is implemented.
package html

import (
	"encoding/json"
	"fmt"
	gohtml "html"
	"strconv"
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

	if tableScene := firstTableScene(doc); tableScene != nil {
		return r.renderTableDoc(doc, tableScene, opts)
	}

	if customScene := firstCustomScene(doc); customScene != nil {
		return r.renderCustomDoc(doc, customScene, opts)
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

// firstTableScene returns the first grid cell's Scene carrying a
// populated Table, or nil when doc has none. buildTableSceneDoc
// (encode/table.go) always produces a 1×1 grid with the table IR on
// its single cell, so in practice this returns either that one Scene
// or nil — the loop tolerates a richer composition shape (e.g. a
// table alongside chart cells) without assuming today's 1×1 layout.
func firstTableScene(doc *scene.SceneDoc) *scene.Scene {
	for i := range doc.Grid.Cells {
		if doc.Grid.Cells[i].Scene.Table != nil {
			return &doc.Grid.Cells[i].Scene
		}
	}
	return nil
}

// renderTableDoc renders a table-mark SceneDoc as a standalone HTML
// document with a semantic <table> body. Delegating straight to
// svg.Render for the whole doc would fail: the table-tagged layer
// trips render/svg's checkMarkSupport guard
// (PRISM_RENDER_MARK_UNSUPPORTED), since a table has no SVG geometry
// of its own. Per-cell sub-marks (e.g. a sparkline column) are the
// exception — those still go through svg.Render, just scoped to one
// cell's Scene IR (see renderSubMarkSVG).
func (r *Renderer) renderTableDoc(doc *scene.SceneDoc, tableScene *scene.Scene, opts render.RenderOpts) ([]byte, error) {
	title := defaultDocTitle
	if tableScene.Title != nil && tableScene.Title.Content != "" {
		title = tableScene.Title.Content
	}

	layerID := ""
	if len(tableScene.Layers) > 0 {
		layerID = tableScene.Layers[0].ID
	}
	tableHTML, err := renderTableMarkup(doc, tableScene.Table, tableScene.ID, layerID, opts)
	if err != nil {
		return nil, err
	}

	var w strings.Builder
	w.WriteString("<!doctype html>\n<html>\n<head>\n")
	w.WriteString(`<meta charset="utf-8">` + "\n")
	w.WriteString("<title>")
	w.WriteString(gohtml.EscapeString(title))
	w.WriteString("</title>\n")
	w.WriteString("<style>html,body{margin:0;padding:0}" +
		".prism-html-table{border-collapse:collapse}" +
		".prism-html-table th,.prism-html-table td{padding:4px 8px;text-align:left;vertical-align:middle}" +
		".prism-html-table th{cursor:pointer;user-select:none}" +
		".prism-html-table-pagination{display:flex;align-items:center;gap:8px;padding:4px 8px}" +
		".prism-html-table tr.prism-selected{background:rgba(76,120,168,0.15)}" +
		"</style>\n")
	w.WriteString("</head>\n<body>\n")
	w.WriteString(tableHTML)
	w.WriteString("\n</body>\n</html>\n")

	return []byte(w.String()), nil
}

// renderTableMarkup builds the interactive table fragment (E1-S5):
// a wrapping <div data-prism-table-root> containing the <table> plus
// (when the row count exceeds tbl.PageSize) a sibling pagination
// control block. One <th> per TableColumn (header text), one <tr>
// per TableRow. A column with no sub-mark bound renders its row's
// plain scalar (TableRow.Values); a column bound to a sub-mark
// (TableRow.Cells) renders that cell's nested Scene IR as inline SVG
// instead.
//
// Attributes stamped for static/vendor/prism/prism-table.mjs to read
// (mirrors the data-prism-* convention prism-selection.mjs already
// relies on for chart marks):
//
//   - <table data-prism-scene="..."> — the enclosing scene's ID, so a
//     row-click selection event can populate SceneID the same way
//     the SVG backend's data-scene-id does for chart marks.
//   - <table data-prism-layer="..."> — the table's single scene
//     layer ID, so a row-click selection event can populate
//     SelectedMark the same way a chart mark's click handler does.
//   - <table data-prism-page-size="N"> — mirrors scene.Table.PageSize
//     so the client can slice the already-rendered rows into pages
//     without another round trip.
//   - <th data-prism-field="..."> — the column's underlying table
//     field, so a header click can look up the right value per row.
//   - <td data-prism-sort-value="..."> — the column's underlying
//     value for this row (JSON-encoded TableRow.Values[field]),
//     *not* the cell's rendered display. This is what lets a
//     sub-mark column (e.g. a sparkline, whose visible content is an
//     <svg> with no plain text/number to sort by) sort correctly:
//     the JS side reads this attribute instead of the cell's
//     rendered content. Every column gets one (not just sub-mark
//     columns) so the sort comparator has one uniform source of
//     truth regardless of column kind.
func renderTableMarkup(doc *scene.SceneDoc, tbl *scene.Table, sceneID, layerID string, opts render.RenderOpts) (string, error) {
	var w strings.Builder
	w.WriteString(`<div class="prism-html-table-wrap" data-prism-table-root>` + "\n")
	w.WriteString(`<table class="prism-html-table"`)
	if sceneID != "" {
		w.WriteString(` data-prism-scene="`)
		w.WriteString(gohtml.EscapeString(sceneID))
		w.WriteString(`"`)
	}
	if layerID != "" {
		w.WriteString(` data-prism-layer="`)
		w.WriteString(gohtml.EscapeString(layerID))
		w.WriteString(`"`)
	}
	w.WriteString(` data-prism-page-size="`)
	w.WriteString(strconv.Itoa(tbl.PageSize))
	w.WriteString(`">` + "\n<thead>\n<tr>\n")
	for _, col := range tbl.Columns {
		w.WriteString(`<th data-prism-field="`)
		w.WriteString(gohtml.EscapeString(col.Field))
		w.WriteString(`">`)
		w.WriteString(gohtml.EscapeString(col.Header))
		w.WriteString("</th>\n")
	}
	w.WriteString("</tr>\n</thead>\n<tbody>\n")

	for _, row := range tbl.Rows {
		w.WriteString(`<tr data-prism-datum-row="`)
		w.WriteString(strconv.FormatInt(row.ID, 10))
		w.WriteString("\">\n")
		for _, col := range tbl.Columns {
			sortValue, err := json.Marshal(row.Values[col.Field])
			if err != nil {
				return "", fmt.Errorf("html.renderTableMarkup: marshal sort value for field %q: %w", col.Field, err)
			}
			w.WriteString(`<td data-prism-sort-value="`)
			w.WriteString(gohtml.EscapeString(string(sortValue)))
			w.WriteString(`">`)
			if cell, ok := row.Cells[col.Field]; ok && cell != nil {
				svgFrag, err := renderSubMarkSVG(doc, opts, cell)
				if err != nil {
					return "", err
				}
				w.Write(svgFrag)
			} else {
				w.WriteString(gohtml.EscapeString(formatCellValue(row.Values[col.Field])))
			}
			w.WriteString("</td>\n")
		}
		w.WriteString("</tr>\n")
	}
	w.WriteString("</tbody>\n</table>\n")

	if tbl.PageSize > 0 && len(tbl.Rows) > tbl.PageSize {
		totalPages := (len(tbl.Rows) + tbl.PageSize - 1) / tbl.PageSize
		w.WriteString(`<div class="prism-html-table-pagination" data-prism-table-pagination>` + "\n")
		w.WriteString(`<button type="button" data-prism-page-prev disabled>Prev</button>` + "\n")
		w.WriteString(`<span data-prism-page-indicator>Page 1 of `)
		w.WriteString(strconv.Itoa(totalPages))
		w.WriteString("</span>\n")
		w.WriteString(`<button type="button" data-prism-page-next>Next</button>` + "\n")
		w.WriteString("</div>\n")
	}

	w.WriteString("</div>\n")
	return w.String(), nil
}

// renderSubMarkSVG re-encodes one table cell's sub-mark Scene IR
// (e.g. a sparkline column's per-row line geometry) as a standalone
// <svg>...</svg> fragment. It wraps cell.Marks in a fresh 1×1-grid
// SceneDoc, sized to scene.TableCellSparkWidth/Height — the exact
// coordinate space encode/table.go's buildTableSubMarkCell resolved
// the sub-mark's X/Y scales against — and hands that doc to
// svg.New().Render, so the emitted geometry inherits render/svg's
// real mark emitters and render.FormatFloat precision rather than
// being hand-serialised here. Returns (nil, nil) for an empty cell.
func renderSubMarkSVG(doc *scene.SceneDoc, opts render.RenderOpts, cell *scene.TableCell) ([]byte, error) {
	if cell == nil || len(cell.Marks) == 0 {
		return nil, nil
	}

	frame := scene.Rect{W: scene.TableCellSparkWidth, H: scene.TableCellSparkHeight}
	cellDoc := &scene.SceneDoc{
		Version: scene.CurrentVersion,
		Theme:   doc.Theme,
		Grid: scene.SceneGrid{
			Layout: scene.GridLayout{Rows: 1, Cols: 1},
			Cells: []scene.SceneCell{{
				Row: 0, Col: 0,
				Scene: scene.Scene{
					ID:     "table-cell",
					Frame:  frame,
					Plot:   frame,
					Layers: []scene.SceneLayer{{ID: "table-cell-layer", Marks: cell.Marks}},
				},
			}},
		},
	}

	cellOpts := opts
	cellOpts.Format = "svg"
	// A cell's inline SVG must stay sized to its own tiny coordinate
	// space, never the enclosing document's requested chart
	// dimensions (RenderOpts.Width/Height apply to the outer chart,
	// not a per-cell fragment); the wrapper's Frame drives the
	// viewBox instead.
	cellOpts.Width = 0
	cellOpts.Height = 0
	return svg.New().Render(cellDoc, cellOpts)
}

// formatCellValue renders a TableRow.Values scalar as display text.
// Mirrors the plain scalar shapes a materialised table cell
// realistically carries (see encode/table.go's parseNumericSeries
// for the array-shaped counterpart used by sub-mark columns).
func formatCellValue(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case float64:
		return render.FormatFloat(val)
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return fmt.Sprintf("%v", val)
	}
}
