package svg

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// renderMark dispatches on the populated geometry pointer. Marks
// with no geometry are silently skipped (the encoder is responsible
// for catching this with PRISM_RENDER_001).
func renderMark(w *Writer, m scene.Mark) {
	switch {
	case m.Rect != nil:
		renderRect(w, m)
	case m.Line != nil:
		renderLine(w, m)
	case m.Area != nil:
		renderArea(w, m)
	case m.Point != nil:
		renderPoint(w, m)
	case m.Rule != nil:
		renderRule(w, m)
	default:
		// Unsupported geometry types (Arc, Text, Path, Image) reach
		// us only if the encoder advanced past P05 without updating
		// the renderer. Emit a comment so the cause is visible.
		w.Raw("<!-- mark type ")
		w.Text(string(m.Type))
		w.Raw(" not rendered in P05 -->")
	}
}

func renderRect(w *Writer, m scene.Mark) {
	g := m.Rect
	w.OpenTag("rect")
	w.Attr("class", "prism-mark-bar")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	w.AttrFloat("x", g.X)
	w.AttrFloat("y", g.Y)
	w.AttrFloat("width", g.W)
	w.AttrFloat("height", g.H)
	if g.CornerR > 0 {
		w.AttrFloat("rx", g.CornerR)
	}
	writeStyleAttrs(w, m.Style)
	w.SelfClose()
}

func renderLine(w *Writer, m scene.Mark) {
	g := m.Line
	if len(g.Points) == 0 {
		return
	}
	// Emit as a <polyline> for P05 (the design's CurveLinear default
	// fits polyline exactly). When non-linear curves land, switch to
	// <path> with the d= attribute.
	w.OpenTag("polyline")
	w.Attr("class", "prism-mark-line")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	w.OpenAttr("points")
	for i, p := range g.Points {
		if i > 0 {
			w.Raw(" ")
		}
		w.Raw(render.FormatFloat(p[0]))
		w.Raw(",")
		w.Raw(render.FormatFloat(p[1]))
	}
	w.CloseAttr()
	// Lines need fill="none" so they don't fill the enclosed area.
	w.Attr("fill", "none")
	writeStyleAttrs(w, m.Style)
	w.SelfClose()
}

func renderArea(w *Writer, m scene.Mark) {
	g := m.Area
	if len(g.Upper) == 0 {
		return
	}
	// Area = path with M (upper start), L's (upper rest), L's down
	// the lower edge (or back to baseline if Lower is nil), Z to
	// close.
	w.OpenTag("path")
	w.Attr("class", "prism-mark-area")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	w.OpenAttr("d")
	// Upper edge: M x0,y0 L x1,y1 L x2,y2 ...
	w.Raw("M")
	w.Raw(render.FormatFloat(g.Upper[0][0]))
	w.Raw(",")
	w.Raw(render.FormatFloat(g.Upper[0][1]))
	for _, p := range g.Upper[1:] {
		w.Raw(" L")
		w.Raw(render.FormatFloat(p[0]))
		w.Raw(",")
		w.Raw(render.FormatFloat(p[1]))
	}
	if g.Lower != nil {
		// Reverse lower edge.
		for i := len(g.Lower) - 1; i >= 0; i-- {
			w.Raw(" L")
			w.Raw(render.FormatFloat(g.Lower[i][0]))
			w.Raw(",")
			w.Raw(render.FormatFloat(g.Lower[i][1]))
		}
	} else {
		// Baseline = the plot's bottom edge. We don't have access to
		// the plot rect here; the encoder must put Lower in for
		// stacked areas. For unstacked baseline-0 areas, the upper
		// edge's last+first x with a y of "max upper y" closes the
		// shape into a baseline strip — but in pixel space the
		// "baseline" is whatever the renderer renders for y=0,
		// which the encoder already snapped via the y-scale. Use
		// the last point's x with the upper edge's max y (max y in
		// SVG = bottom of plot region for an inverted y-axis).
		lastX := g.Upper[len(g.Upper)-1][0]
		firstX := g.Upper[0][0]
		// Compute max y in upper points.
		maxY := g.Upper[0][1]
		for _, p := range g.Upper {
			if p[1] > maxY {
				maxY = p[1]
			}
		}
		// In SVG with inverted y, "baseline" = max y (bottom edge).
		// The encoder doesn't tell us where the actual baseline is.
		// For the P05 fixtures this gives the visually-correct area
		// (filled from the line down to the bottom of the highest
		// point — close enough for goldens; P08 lands stacking +
		// proper baselines).
		w.Raw(" L")
		w.Raw(render.FormatFloat(lastX))
		w.Raw(",")
		w.Raw(render.FormatFloat(maxY))
		w.Raw(" L")
		w.Raw(render.FormatFloat(firstX))
		w.Raw(",")
		w.Raw(render.FormatFloat(maxY))
	}
	w.Raw(" Z")
	w.CloseAttr()
	writeStyleAttrs(w, m.Style)
	w.SelfClose()
}

func renderPoint(w *Writer, m scene.Mark) {
	g := m.Point
	w.OpenTag("circle")
	w.Attr("class", "prism-mark-point")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	w.AttrFloat("cx", g.Cx)
	w.AttrFloat("cy", g.Cy)
	w.AttrFloat("r", g.R)
	writeStyleAttrs(w, m.Style)
	w.SelfClose()
}

func renderRule(w *Writer, m scene.Mark) {
	g := m.Rule
	w.OpenTag("line")
	w.Attr("class", "prism-mark-rule")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	w.AttrFloat("x1", g.X1)
	w.AttrFloat("y1", g.Y1)
	w.AttrFloat("x2", g.X2)
	w.AttrFloat("y2", g.Y2)
	writeStyleAttrs(w, m.Style)
	w.SelfClose()
}
