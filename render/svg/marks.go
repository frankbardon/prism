package svg

import (
	"math"
	"strconv"
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// formatInt64 returns the decimal string for n. Pure base-10 with no
// scientific notation — matches the JS port's `String(Number)` output
// for safe integer ranges (row indices stay well under 2^53).
func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}

// renderMark dispatches on the populated geometry pointer. Marks
// with no geometry are silently skipped (the encoder is responsible
// for catching this with PRISM_RENDER_001).
func renderMark(w *Writer, m scene.Mark) {
	switch {
	case m.Rect != nil:
		renderRect(w, m)
	case m.Arc != nil:
		renderArc(w, m)
	case m.Line != nil:
		renderLine(w, m)
	case m.Area != nil:
		renderArea(w, m)
	case m.Point != nil:
		renderPoint(w, m)
	case m.Rule != nil:
		renderRule(w, m)
	case m.Text != nil:
		renderTextMark(w, m)
	case m.Path != nil:
		renderPath(w, m)
	case m.Image != nil:
		renderImage(w, m)
	case m.Geoshape != nil:
		renderGeoshape(w, m)
	default:
		// Unknown geometry — defensive comment so the cause is visible.
		w.Raw("<!-- mark type ")
		w.Text(string(m.Type))
		w.Raw(" not rendered -->")
	}
}

// hasTooltip reports whether the mark carries at least one tooltip
// line worth rendering. Used by the per-mark renderers to decide
// between SelfClose (no tooltip) and open-close (with <title>) tag
// emission.
func hasTooltip(m scene.Mark) bool {
	return m.Tooltip != nil && len(m.Tooltip.Lines) > 0
}

// writeDatumAttr writes data-prism-datum-row="<row-id>" when the mark
// carries a Datum back-reference (D077). Marks without Datum (composite
// helpers, e.g. boxplot whisker pairs) get no attribute and the JS
// hit-test silently ignores them. Pure decimal integer formatting;
// no scientific notation.
func writeDatumAttr(w *Writer, m scene.Mark) {
	if m.Datum == nil {
		return
	}
	w.Attr("data-prism-datum-row", formatInt64(m.Datum.RowID))
}

// writeKeyAttr writes data-prism-mark-key="<key>" when the mark
// carries the animation join key (PR animation-1). Empty keys
// (the default) emit nothing, preserving existing SVG goldens
// byte-for-byte for non-animated specs.
// writeSeriesAttr stamps the colour category a path mark belongs to,
// giving a client a stable hook to highlight or hide one series
// without re-deriving the grouping the encoder already did.
func writeSeriesAttr(w *Writer, m scene.Mark) {
	if m.Series != "" {
		w.Attr("data-prism-series", m.Series)
	}
}

func writeKeyAttr(w *Writer, m scene.Mark) {
	if m.Key == "" {
		return
	}
	w.Attr("data-prism-mark-key", m.Key)
}

// writeTooltipChild emits a `<title>` child element carrying the
// joined tooltip lines (newline-separated). Called after the
// mark's opening tag has been closed with CloseTagOpen. Caller is
// responsible for the surrounding EndTag.
func writeTooltipChild(w *Writer, m scene.Mark) {
	if !hasTooltip(m) {
		return
	}
	parts := make([]string, 0, len(m.Tooltip.Lines))
	for _, ln := range m.Tooltip.Lines {
		parts = append(parts, ln.Label)
	}
	content := strings.Join(parts, "\n")
	w.OpenTag("title")
	w.CloseTagOpen()
	w.Text(content)
	w.EndTag("title")
}

func renderRect(w *Writer, m scene.Mark) {
	g := m.Rect
	// A rect rounded on ONE side cannot be an SVG <rect>: rx rounds
	// all four corners. Emit a path instead, which keeps the bar
	// planted on its baseline while softening the value end.
	if g.CornerR > 0 && g.CornerSide != "" {
		renderRoundedEndRect(w, m)
		return
	}
	w.OpenTag("rect")
	w.Attr("class", "prism-mark-bar")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.AttrFloat("x", g.X)
	w.AttrFloat("y", g.Y)
	w.AttrFloat("width", g.W)
	w.AttrFloat("height", g.H)
	if g.CornerR > 0 {
		w.AttrFloat("rx", g.CornerR)
	}
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("rect")
		return
	}
	w.SelfClose()
}

// renderRoundedEndRect emits a bar as a path with two rounded corners
// on the side the value reaches and two square corners on the
// baseline side.
func renderRoundedEndRect(w *Writer, m scene.Mark) {
	g := m.Rect
	w.OpenTag("path")
	w.Attr("class", "prism-mark-bar")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.Attr("d", roundedEndPath(g.X, g.Y, g.W, g.H, g.CornerR, g.CornerSide))
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("path")
		return
	}
	w.SelfClose()
}

// roundedEndPath builds the SVG d-string for a rect with two rounded
// corners. Coordinates route through formatF so the path quantises to
// the same 3 decimals as every other primitive and the cross-impl
// goldens stay byte-stable.
func roundedEndPath(x, y, wd, h, r float64, side string) string {
	f := formatF
	arc := func(cx, cy float64) string {
		return "A" + f(r) + " " + f(r) + " 0 0 1 " + f(cx) + " " + f(cy)
	}
	switch side {
	case "bottom":
		// Rounded along y+h.
		return "M" + f(x) + " " + f(y) +
			"H" + f(x+wd) +
			"V" + f(y+h-r) +
			arc(x+wd-r, y+h) +
			"H" + f(x+r) +
			arc(x, y+h-r) +
			"Z"
	case "left":
		return "M" + f(x+wd) + " " + f(y) +
			"H" + f(x+r) +
			arc(x, y+r) +
			"V" + f(y+h-r) +
			arc(x+r, y+h) +
			"H" + f(x+wd) +
			"Z"
	case "right":
		return "M" + f(x) + " " + f(y) +
			"H" + f(x+wd-r) +
			arc(x+wd, y+r) +
			"V" + f(y+h-r) +
			arc(x+wd-r, y+h) +
			"H" + f(x) +
			"Z"
	default: // "top"
		return "M" + f(x) + " " + f(y+h) +
			"V" + f(y+r) +
			arc(x+r, y) +
			"H" + f(x+wd-r) +
			arc(x+wd, y+r) +
			"V" + f(y+h) +
			"Z"
	}
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
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
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
	// Round joins and caps: a mitred join on a sharp reversal throws a
	// spike several times the stroke width past the vertex, which
	// reads as a data point that is not there.
	w.Attr("stroke-linejoin", "round")
	w.Attr("stroke-linecap", "round")
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("polyline")
		return
	}
	w.SelfClose()
}

func renderArea(w *Writer, m scene.Mark) {
	g := m.Area
	if len(g.Upper) == 0 {
		return
	}
	// Area = path with M (upper start), L's (upper rest), L's down
	// the reversed lower edge, Z to close. The encoder supplies Lower
	// as the y=0 baseline edge (one point per Upper x).
	w.OpenTag("path")
	w.Attr("class", "prism-mark-area")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
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
	// Reverse the lower (baseline) edge to close the shape. The encoder
	// always supplies Lower for area marks; a degenerate empty Lower
	// closes straight back to the upper start via Z.
	for i := len(g.Lower) - 1; i >= 0; i-- {
		w.Raw(" L")
		w.Raw(render.FormatFloat(g.Lower[i][0]))
		w.Raw(",")
		w.Raw(render.FormatFloat(g.Lower[i][1]))
	}
	w.Raw(" Z")
	w.CloseAttr()
	// The filled shape never takes the stroke.
	//
	// Stroking a closed area outlines the baseline and both vertical
	// sides as well as the top, boxing the fill in three lines that
	// encode nothing. The top edge — the one that IS the data — is
	// emitted separately below.
	fill := m.Style
	fill.Stroke = nil
	fill.StrokeWidth = 0
	writeStyleAttrs(w, fill)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("path")
	} else {
		w.SelfClose()
	}
	renderAreaEdge(w, m)
}

// renderAreaEdge draws the area's upper boundary as an open path, so a
// pale fill still has a definite line along the values it represents.
// Emitted only when the style carries a stroke.
func renderAreaEdge(w *Writer, m scene.Mark) {
	if m.Style.Stroke == nil || m.Style.StrokeWidth <= 0 || len(m.Area.Upper) < 2 {
		return
	}
	w.OpenTag("path")
	w.Attr("class", "prism-mark-area-edge")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID+"-edge")
	}
	writeSeriesAttr(w, m)
	w.OpenAttr("d")
	w.Raw("M")
	w.Raw(render.FormatFloat(m.Area.Upper[0][0]))
	w.Raw(",")
	w.Raw(render.FormatFloat(m.Area.Upper[0][1]))
	for _, p := range m.Area.Upper[1:] {
		w.Raw(" L")
		w.Raw(render.FormatFloat(p[0]))
		w.Raw(",")
		w.Raw(render.FormatFloat(p[1]))
	}
	w.CloseAttr()
	w.Attr("fill", "none")
	w.Attr("stroke", m.Style.Stroke.CSS())
	w.AttrFloat("stroke-width", m.Style.StrokeWidth)
	w.Attr("stroke-linejoin", "round")
	w.Attr("stroke-linecap", "round")
	w.SelfClose()
}

func renderPoint(w *Writer, m scene.Mark) {
	g := m.Point
	w.OpenTag("circle")
	w.Attr("class", "prism-mark-point")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.AttrFloat("cx", g.Cx)
	w.AttrFloat("cy", g.Cy)
	w.AttrFloat("r", g.R)
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("circle")
		return
	}
	w.SelfClose()
}

func renderTextMark(w *Writer, m scene.Mark) {
	g := m.Text
	w.OpenTag("text")
	w.Attr("class", "prism-mark-text")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.AttrFloat("x", g.X)
	w.AttrFloat("y", g.Y)
	switch g.Anchor {
	case scene.AnchorStart:
		w.Attr("text-anchor", "start")
	case scene.AnchorEnd:
		w.Attr("text-anchor", "end")
	default:
		w.Attr("text-anchor", "middle")
	}
	if g.FontSize > 0 {
		w.AttrFloat("font-size", g.FontSize)
	}
	if g.Angle != 0 {
		w.Attr("transform", rotateAround(g.Angle, g.X, g.Y))
	}
	writeStyleAttrs(w, m.Style)
	w.CloseTagOpen()
	w.Text(g.Content)
	writeTooltipChild(w, m)
	w.EndTag("text")
}

// renderArc emits a `<path class="prism-mark-arc" d="...">` sector.
// For donut sectors (InnerR > 0) the path includes both an outer and
// inner arc; for pie sectors (InnerR == 0) only the outer arc plus a
// line back to the center.
func renderArc(w *Writer, m scene.Mark) {
	g := m.Arc
	w.OpenTag("path")
	w.Attr("class", "prism-mark-arc")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.Attr("d", arcPath(g))
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("path")
		return
	}
	w.SelfClose()
}

// arcPath builds the SVG path data string for an ArcGeom.
// Path layout (donut sector, InnerR > 0):
//
//	M Ax,Ay A R,R 0 LF 1 Bx,By L Cx,Cy A r,r 0 LF 0 Dx,Dy Z
//
// where A = outer start, B = outer end, C = inner end, D = inner
// start. For a pie sector (InnerR == 0):
//
//	M Cx,Cy L Ax,Ay A R,R 0 LF 1 Bx,By Z
func arcPath(g *scene.ArcGeom) string {
	const sweepCW = 1 // SVG sweep flag: 1 = clockwise in pixel space.
	const sweepCCW = 0
	largeArc := "0"
	if (g.EndAngle - g.StartAngle) > 3.141592653589793 {
		largeArc = "1"
	}
	cosS := cos(g.StartAngle)
	sinS := sin(g.StartAngle)
	cosE := cos(g.EndAngle)
	sinE := sin(g.EndAngle)
	ax := g.Cx + g.OuterR*cosS
	ay := g.Cy + g.OuterR*sinS
	bx := g.Cx + g.OuterR*cosE
	by := g.Cy + g.OuterR*sinE
	if g.InnerR <= 0 {
		// Pie sector.
		return "M" + ff(g.Cx) + "," + ff(g.Cy) +
			" L" + ff(ax) + "," + ff(ay) +
			" A" + ff(g.OuterR) + "," + ff(g.OuterR) + " 0 " + largeArc + " " + itoa(sweepCW) + " " + ff(bx) + "," + ff(by) +
			" Z"
	}
	cx := g.Cx + g.InnerR*cosE
	cy := g.Cy + g.InnerR*sinE
	dx := g.Cx + g.InnerR*cosS
	dy := g.Cy + g.InnerR*sinS
	return "M" + ff(ax) + "," + ff(ay) +
		" A" + ff(g.OuterR) + "," + ff(g.OuterR) + " 0 " + largeArc + " " + itoa(sweepCW) + " " + ff(bx) + "," + ff(by) +
		" L" + ff(cx) + "," + ff(cy) +
		" A" + ff(g.InnerR) + "," + ff(g.InnerR) + " 0 " + largeArc + " " + itoa(sweepCCW) + " " + ff(dx) + "," + ff(dy) +
		" Z"
}

func ff(v float64) string { return render.FormatFloat(v) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	return "1"
}

func cos(a float64) float64 { return math.Cos(a) }
func sin(a float64) float64 { return math.Sin(a) }

// renderPath emits a `<path class="prism-mark-path" d="..."/>`
// passing the user-supplied d string straight through. The writer's
// attribute escaping (escapeAttr) handles the five XML special chars
// so pathological d values round-trip safely. See D068.
func renderPath(w *Writer, m scene.Mark) {
	g := m.Path
	w.OpenTag("path")
	w.Attr("class", "prism-mark-path")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.Attr("d", g.D)
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("path")
		return
	}
	w.SelfClose()
}

// renderImage emits a `<image class="prism-mark-image" x="" y=""
// width="" height="" href="..."/>`. The href is attribute-escaped.
// See D068 for the data: + relative-path allowlist enforced by
// validator PRISM_SPEC_016.
func renderImage(w *Writer, m scene.Mark) {
	g := m.Image
	w.OpenTag("image")
	w.Attr("class", "prism-mark-image")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.AttrFloat("x", g.X)
	w.AttrFloat("y", g.Y)
	w.AttrFloat("width", g.W)
	w.AttrFloat("height", g.H)
	w.Attr("href", g.Href)
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("image")
		return
	}
	w.SelfClose()
}

func renderRule(w *Writer, m scene.Mark) {
	g := m.Rule
	w.OpenTag("line")
	w.Attr("class", "prism-mark-rule")
	if m.ID != "" {
		w.Attr("data-prism-id", m.ID)
	}
	writeDatumAttr(w, m)
	writeKeyAttr(w, m)
	writeSeriesAttr(w, m)
	w.AttrFloat("x1", g.X1)
	w.AttrFloat("y1", g.Y1)
	w.AttrFloat("x2", g.X2)
	w.AttrFloat("y2", g.Y2)
	writeStyleAttrs(w, m.Style)
	if hasTooltip(m) {
		w.CloseTagOpen()
		writeTooltipChild(w, m)
		w.EndTag("line")
		return
	}
	w.SelfClose()
}
