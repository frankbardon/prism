package svg

import (
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// renderGeoshape emits one <path d="M…Z M…Z"/> per polygon mark.
// Holes are appended as additional sub-paths and rendered with the
// SVG even-odd fill rule so cutouts work without an explicit
// boolean op. Coordinates pass through render.FormatFloat for the
// pinned 3-decimal precision contract.
func renderGeoshape(w *Writer, m scene.Mark) {
	g := m.Geoshape
	if g == nil || len(g.Outer) < 3 {
		return
	}
	w.OpenTag("path")
	w.Attr("class", "prism-mark-geoshape")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	w.Attr("d", buildPolygonPath(g))
	w.Attr("fill-rule", "evenodd")
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("path")
		return
	}
	w.SelfClose()
}

func buildPolygonPath(g *scene.PolygonGeom) string {
	var b strings.Builder
	writeRing(&b, g.Outer)
	for _, hole := range g.Holes {
		if len(hole) < 3 {
			continue
		}
		writeRing(&b, hole)
	}
	return b.String()
}

func writeRing(b *strings.Builder, ring [][2]float64) {
	if len(ring) == 0 {
		return
	}
	for i, p := range ring {
		if i == 0 {
			b.WriteByte('M')
		} else {
			b.WriteByte('L')
		}
		b.WriteString(render.FormatFloat(p[0]))
		b.WriteByte(',')
		b.WriteString(render.FormatFloat(p[1]))
	}
	b.WriteByte('Z')
}
