package html

// custom.go implements the HTML backend's handling of a scene.Custom
// node (E2-S3). It mirrors render/svg/custom.go's shape but with the
// preference reversed: the HTML backend is a native HTML host, so a
// renderer implementing HTMLCustomRenderer is called directly and its
// output is spliced VERBATIM into the document — including any
// author-supplied <script> tag, which the browser then executes
// normally. That is the *fallback* path under the SVG backend
// (foreignObject-wrapped); here it is preferred.
//
// A renderer offering only SVGCustomRenderer falls back to an inline
// <svg>...</svg> fragment. That fragment is produced by wrapping the
// same scene.Custom in a small standalone SceneDoc (the exact shape
// encode/custom.go's buildCustomSceneDoc already establishes: a 1x1
// grid, one scene tagged with a zero-Marks SceneLayer of
// scene.MarkCustom, Frame/Plot sized to the mark's Box) and delegating
// to svg.New().Render — the same "wrap a sub-scene + delegate to
// svg.Render" mechanism renderSubMarkSVG (renderer.go) already
// established for embedding a table sub-mark's SVG inside HTML
// output. That keeps the fragment on render/svg's real custom-mark
// splice logic and render.FormatFloat precision, rather than hand-
// serialising it here.
import (
	"fmt"
	gohtml "html"
	"strings"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
)

// firstCustomScene returns the first grid cell's Scene carrying a
// populated Custom, or nil when doc has none. Mirrors firstTableScene;
// buildCustomSceneDoc (encode/custom.go) always produces a 1x1 grid
// with the custom-mark IR on its single cell, so in practice this
// returns either that one Scene or nil.
func firstCustomScene(doc *scene.SceneDoc) *scene.Scene {
	for i := range doc.Grid.Cells {
		if doc.Grid.Cells[i].Scene.Custom != nil {
			return &doc.Grid.Cells[i].Scene
		}
	}
	return nil
}

// renderCustomDoc renders a custom-mark SceneDoc as a standalone HTML
// document. Delegating straight to svg.Render for the whole doc would
// still succeed (render/svg supports scene.MarkCustom directly), but
// it would apply the SVG backend's own preference order (SVGCustom-
// Renderer spliced, HTMLCustomRenderer foreignObject-wrapped) — the
// opposite of what a native HTML host wants (E2-S3), so this backend
// resolves the renderer itself instead of delegating the top-level
// dispatch.
func (r *Renderer) renderCustomDoc(doc *scene.SceneDoc, customScene *scene.Scene, opts render.RenderOpts) ([]byte, error) {
	title := defaultDocTitle
	if customScene.Title != nil && customScene.Title.Content != "" {
		title = customScene.Title.Content
	}

	sceneTheme := opts.Theme
	if sceneTheme == nil {
		sceneTheme = doc.Theme
	}
	if sceneTheme == nil {
		sceneTheme = scene.Default()
	}

	fragment, err := renderCustomMarkup(customScene.Custom, sceneTheme, opts)
	if err != nil {
		return nil, err
	}

	var w strings.Builder
	w.WriteString("<!doctype html>\n<html>\n<head>\n")
	w.WriteString(`<meta charset="utf-8">` + "\n")
	w.WriteString("<title>")
	w.WriteString(gohtml.EscapeString(title))
	w.WriteString("</title>\n")
	w.WriteString("<style>html,body{margin:0;padding:0}.prism-html-custom-mark{max-width:100%}</style>\n")
	w.WriteString("</head>\n<body>\n")
	w.WriteString(fragment)
	w.WriteString("\n</body>\n</html>\n")

	return []byte(w.String()), nil
}

// renderCustomMarkup resolves c.Renderer against the active
// custommark registry and returns the fragment to splice into the
// HTML document body:
//
//   - HTMLCustomRenderer is preferred: its output is inserted
//     verbatim inside a wrapping <div> — no escaping, no tag
//     stripping — so a caller-authored <script> tag executes
//     normally in the browser (E2-S3 acceptance criterion 1).
//   - A renderer offering only SVGCustomRenderer falls back to an
//     inline <svg>...</svg> fragment (E2-S3 acceptance criterion 2),
//     produced by renderCustomAsSVG.
//
// An unregistered name, or a registry entry implementing neither
// interface, produces the same PRISM_RENDER_CUSTOM_MARK_NOT_FOUND
// *errors.AppError render/svg's own renderCustom would raise (E2-S2)
// — same code, same shape, so a caller sees a consistent error
// regardless of which backend rendered the spec.
func renderCustomMarkup(c *scene.Custom, sceneTheme *scene.Theme, opts render.RenderOpts) (string, error) {
	// LookupWithJSFallback (not Lookup): see render/svg/custom.go's
	// matching call site (E2-S5) — a custom mark registered only via
	// the WASM entry's prism.registerCustomMark still resolves here.
	renderer, ok := custommark.LookupWithJSFallback(c.Renderer)
	if !ok {
		return "", prismerrors.New(
			"PRISM_RENDER_CUSTOM_MARK_NOT_FOUND",
			fmt.Sprintf("Custom mark renderer %q is not registered.", c.Renderer),
			map[string]any{"Renderer": c.Renderer, "Available": strings.Join(custommark.Names(), ", ")},
		)
	}

	tokens := svg.ThemeTokensFromScene(sceneTheme)

	var w strings.Builder
	w.WriteString(`<div class="prism-html-custom-mark" data-prism-renderer="`)
	w.WriteString(gohtml.EscapeString(c.Renderer))
	w.WriteString(`">` + "\n")

	switch impl := renderer.(type) {
	case custommark.HTMLCustomRenderer:
		frag, err := impl.RenderHTML(c.Rows, c.Box, tokens)
		if err != nil {
			return "", fmt.Errorf("custom mark %q: RenderHTML: %w", c.Renderer, err)
		}
		w.WriteString(frag)
	case custommark.SVGCustomRenderer:
		svgFrag, err := renderCustomAsSVG(c, sceneTheme, opts)
		if err != nil {
			return "", err
		}
		w.Write(svgFrag)
	default:
		// Register already rejects a renderer implementing neither
		// interface, so this only fires if a caller constructed the
		// registry entry some other way (mirrors render/svg's own
		// defensive default case).
		return "", prismerrors.New(
			"PRISM_RENDER_CUSTOM_MARK_NOT_FOUND",
			fmt.Sprintf("Custom mark renderer %q implements neither SVGCustomRenderer nor HTMLCustomRenderer.", c.Renderer),
			map[string]any{"Renderer": c.Renderer, "Available": strings.Join(custommark.Names(), ", ")},
		)
	}

	w.WriteString("\n</div>\n")
	return w.String(), nil
}

// renderCustomAsSVG re-encodes a custom mark's Scene IR as a
// standalone <svg>...</svg> fragment, for a renderer that implements
// only SVGCustomRenderer. Wraps c in a fresh 1x1-grid SceneDoc sized
// to c.Box (dimensions only — a custom mark never resolves a
// position, so Frame/Plot both start at the origin) and hands that
// doc to svg.New().Render, so the emitted geometry inherits render/
// svg's real custom-mark splice logic (render/svg/custom.go) and
// render.FormatFloat precision rather than being hand-serialised
// here — the same "wrap + delegate to svg.Render" mechanism
// renderSubMarkSVG (renderer.go) uses for a table sub-mark column.
func renderCustomAsSVG(c *scene.Custom, sceneTheme *scene.Theme, opts render.RenderOpts) ([]byte, error) {
	frame := scene.Rect{W: c.Box.W, H: c.Box.H}
	cellDoc := &scene.SceneDoc{
		Version: scene.CurrentVersion,
		Theme:   sceneTheme,
		Grid: scene.SceneGrid{
			Layout: scene.GridLayout{Rows: 1, Cols: 1},
			Cells: []scene.SceneCell{{
				Row: 0, Col: 0,
				Scene: scene.Scene{
					ID:     "custom-mark",
					Frame:  frame,
					Plot:   frame,
					Layers: []scene.SceneLayer{{ID: "custom-mark-layer", Mark: scene.MarkCustom}},
					Custom: c,
				},
			}},
		},
	}

	cellOpts := opts
	cellOpts.Format = "svg"
	// A custom mark's inline SVG fragment stays sized to its own Box,
	// never the enclosing document's requested chart dimensions
	// (RenderOpts.Width/Height apply to the outer chart, not a
	// per-fragment embed); the wrapper's Frame drives the viewBox
	// instead.
	cellOpts.Width = 0
	cellOpts.Height = 0
	return svg.New().Render(cellDoc, cellOpts)
}
