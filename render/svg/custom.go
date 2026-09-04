package svg

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/theme"
)

// renderCustom emits s.Custom's registered CustomRenderer output at
// the scene's plot origin (E2). Unlike scene.Table (HTML-only,
// checkMarkSupport rejects it outright), a custom mark IS supported
// directly by the SVG backend:
//
//   - A renderer implementing SVGCustomRenderer is preferred — its
//     fragment is spliced verbatim into the SVG tree, no wrapper.
//   - A renderer offering only HTMLCustomRenderer falls back to a
//     <foreignObject>-wrapped emission (with an inner XHTML-
//     namespaced element, since the standalone-SVG document is parsed
//     as XML and a bare HTML fragment is not well-formed in the SVG
//     namespace).
//
// Both paths are positioned via a wrapping `<g transform="translate(x
// y)">`: Box (custommark's contract) carries dimensions only, no
// position, so Prism itself supplies the plot-origin offset. That
// translate is the only Prism-computed coordinate pair here — it goes
// through render.FormatFloat for RenderPrecision quantisation — the
// spliced/wrapped fragment's own content is opaque and passed through
// untouched.
func renderCustom(w *Writer, s scene.Scene, sceneTheme *scene.Theme) error {
	c := s.Custom
	if c == nil {
		return nil
	}

	renderer, ok := custommark.Lookup(c.Renderer)
	if !ok {
		return prismerrors.New(
			"PRISM_RENDER_CUSTOM_MARK_NOT_FOUND",
			fmt.Sprintf("Custom mark renderer %q is not registered.", c.Renderer),
			map[string]any{"Renderer": c.Renderer, "Available": strings.Join(custommark.Names(), ", ")},
		)
	}

	tokens := themeTokensFromScene(sceneTheme)

	var fragment string
	var wrapHTML bool
	switch impl := renderer.(type) {
	case custommark.SVGCustomRenderer:
		frag, err := impl.RenderSVG(c.Rows, c.Box, tokens)
		if err != nil {
			return fmt.Errorf("custom mark %q: RenderSVG: %w", c.Renderer, err)
		}
		fragment = frag
	case custommark.HTMLCustomRenderer:
		frag, err := impl.RenderHTML(c.Rows, c.Box, tokens)
		if err != nil {
			return fmt.Errorf("custom mark %q: RenderHTML: %w", c.Renderer, err)
		}
		fragment = frag
		wrapHTML = true
	default:
		// Register already rejects a renderer implementing neither
		// interface, so this only fires if a caller constructed the
		// registry entry some other way.
		return prismerrors.New(
			"PRISM_RENDER_CUSTOM_MARK_NOT_FOUND",
			fmt.Sprintf("Custom mark renderer %q implements neither SVGCustomRenderer nor HTMLCustomRenderer.", c.Renderer),
			map[string]any{"Renderer": c.Renderer, "Available": strings.Join(custommark.Names(), ", ")},
		)
	}

	w.OpenTag("g")
	w.Attr("class", "prism-custom-mark")
	w.Attr("data-prism-renderer", c.Renderer)
	w.OpenAttr("transform")
	w.Raw("translate(")
	w.Raw(render.FormatFloat(s.Plot.X))
	w.Raw(" ")
	w.Raw(render.FormatFloat(s.Plot.Y))
	w.Raw(")")
	w.CloseAttr()
	w.CloseTagOpen()

	if wrapHTML {
		w.OpenTag("foreignObject")
		w.AttrFloat("width", c.Box.W)
		w.AttrFloat("height", c.Box.H)
		w.CloseTagOpen()
		w.OpenTag("div")
		w.Attr("xmlns", "http://www.w3.org/1999/xhtml")
		w.CloseTagOpen()
		w.Raw(fragment)
		w.EndTag("div")
		w.EndTag("foreignObject")
	} else {
		w.Raw(fragment)
	}

	w.EndTag("g")
	return nil
}

// themeTokensFromScene builds a best-effort *theme.Theme from the
// resolved *scene.Theme in scope at render time. render/svg only ever
// receives the pared-down wire-form scene.Theme (RenderOpts.Theme /
// SceneDoc.Theme) — the fully resolved theme.Theme (per-mark blocks,
// scheme registry, range slots) lives only at encode time and is not
// threaded through the Scene IR (theme depends on encode/scene, so
// the reverse dependency isn't available here without a cycle). This
// reconstructs the legacy flat fields theme.Theme still carries for
// exactly this kind of downstream consumer, which is what a custom
// mark actually needs to stay visually consistent — see
// docs/FOLLOWUPS in the E2-S2 story for the fuller tradeoff.
func themeTokensFromScene(t *scene.Theme) *theme.Theme {
	if t == nil {
		t = scene.Default()
	}
	out := &theme.Theme{
		Name:            t.Name,
		BackgroundColor: t.Background,
		FontSans:        t.FontSans,
		FontMono:        t.FontMono,
	}
	if t.ColorAxis != nil {
		out.AxisColor = t.ColorAxis.Hex()
	}
	if t.ColorGrid != nil {
		out.GridColor = t.ColorGrid.Hex()
	}
	if t.ColorText != nil {
		out.TextColor = t.ColorText.Hex()
	}
	return out
}
