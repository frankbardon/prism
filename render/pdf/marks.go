//go:build !js

package pdf

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/signintech/gopdf"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// renderMark dispatches one Scene IR Mark to its per-geom handler.
// Mirrors render/svg/marks.go's switch order so the two renderers
// stay structurally aligned. Returns the first per-mark error a
// handler raises (path subset rejection, image URL rejection, etc.);
// the caller decides whether to abort or continue.
func renderMark(pdf *gopdf.GoPdf, m scene.Mark, theme *scene.Theme) error {
	switch {
	case m.Rect != nil:
		return renderRect(pdf, m)
	case m.Arc != nil:
		return renderArc(pdf, m)
	case m.Line != nil:
		return renderLine(pdf, m)
	case m.Area != nil:
		return renderArea(pdf, m)
	case m.Point != nil:
		return renderPoint(pdf, m)
	case m.Rule != nil:
		return renderRule(pdf, m)
	case m.Text != nil:
		return renderText(pdf, m, theme)
	case m.Path != nil:
		return renderPath(pdf, m)
	case m.Image != nil:
		return renderImage(pdf, m)
	default:
		return nil // silent skip — encoder catches via PRISM_RENDER_001
	}
}

func renderRect(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Rect
	end := applyMarkStyle(pdf, m.Style)
	defer end()

	style := styleString(m.Style.Fill, m.Style.Stroke, m.Style.StrokeWidth)
	if style == "" {
		style = "F" // default — invisible-but-still-present mark
	}
	if g.CornerR > 0 {
		// gopdf.Rectangle handles rounded corners internally via a
		// per-corner Bezier approximation. radiusPointNum = 8
		// matches typical d3-shape rounding fidelity.
		_ = pdf.Rectangle(g.X, g.Y, g.X+g.W, g.Y+g.H, style, g.CornerR, 8)
	} else {
		pdf.RectFromUpperLeftWithStyle(g.X, g.Y, g.W, g.H, style)
	}
	return nil
}

func renderLine(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Line
	if len(g.Points) < 2 {
		return nil
	}
	end := applyMarkStyle(pdf, m.Style)
	defer end()

	// Curve dispatch — for non-linear curves we approximate via the
	// SVG renderer's logic: step = stair-step inserts; monotone +
	// cardinal = polyline approximation since gopdf's Curve takes
	// one cubic at a time and chains of independent cubics drift
	// without slope continuity tracking. We accept the linear
	// fallback for monotone/cardinal in v1 and document the
	// deviation in the SUMMARY.
	pts := g.Points
	if g.Curve == scene.CurveStep {
		pts = stepifyPoints(g.Points)
	}
	for i := 1; i < len(pts); i++ {
		pdf.Line(pts[i-1][0], pts[i-1][1], pts[i][0], pts[i][1])
	}
	return nil
}

// stepifyPoints inserts intermediate vertices at the X of each
// successor so adjacent points form a stair-step. Matches the SVG
// renderer's step curve behaviour for monotonic-X data.
func stepifyPoints(in [][2]float64) [][2]float64 {
	if len(in) < 2 {
		return in
	}
	out := make([][2]float64, 0, len(in)*2-1)
	out = append(out, in[0])
	for i := 1; i < len(in); i++ {
		out = append(out, [2]float64{in[i][0], in[i-1][1]})
		out = append(out, in[i])
	}
	return out
}

func renderArea(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Area
	if len(g.Upper) < 2 {
		return nil
	}
	end := applyMarkStyle(pdf, m.Style)
	defer end()

	// Build closed polygon: upper forward + reversed lower (or
	// baseline 0 when Lower is nil). The Y baseline is the maximum
	// Y of the upper polyline + a safe pad — but Scene IR's
	// convention is that the encoder has already pre-resolved
	// baseline coordinates, so nil Lower means "snap to plot
	// bottom" which we approximate via the upper polyline's max Y.
	points := make([]gopdf.Point, 0, len(g.Upper)*2)
	for _, p := range g.Upper {
		points = append(points, gopdf.Point{X: p[0], Y: p[1]})
	}
	if len(g.Lower) > 0 {
		for i := len(g.Lower) - 1; i >= 0; i-- {
			points = append(points, gopdf.Point{X: g.Lower[i][0], Y: g.Lower[i][1]})
		}
	} else {
		// Baseline 0 = rightmost X back to leftmost X at y=0.
		// Caller-supplied y=0 means the top of the page in PDF coords,
		// so this branch is rarely correct on its own — the encoder
		// usually pre-populates Lower. Defensive only.
		baseline := 0.0
		points = append(points,
			gopdf.Point{X: g.Upper[len(g.Upper)-1][0], Y: baseline},
			gopdf.Point{X: g.Upper[0][0], Y: baseline},
		)
	}

	style := styleString(m.Style.Fill, m.Style.Stroke, m.Style.StrokeWidth)
	if style == "" {
		style = "F"
	}
	pdf.Polygon(points, style)
	return nil
}

func renderPoint(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Point
	end := applyMarkStyle(pdf, m.Style)
	defer end()

	style := styleString(m.Style.Fill, m.Style.Stroke, m.Style.StrokeWidth)
	if style == "" {
		style = "F"
	}

	switch g.Shape {
	case "", scene.ShapeCircle:
		// gopdf.Oval signature is (x1, y1, x2, y2) = bounding box.
		// Oval strokes without fill control; for filled circles we
		// emulate with the bezier circle approximation.
		if style == "D" {
			pdf.Oval(g.Cx-g.R, g.Cy-g.R, g.Cx+g.R, g.Cy+g.R)
		} else {
			drawCircleBezier(pdf, g.Cx, g.Cy, g.R, style)
		}
	case scene.ShapeSquare:
		pdf.RectFromUpperLeftWithStyle(g.Cx-g.R, g.Cy-g.R, g.R*2, g.R*2, style)
	case scene.ShapeTriangle:
		pts := []gopdf.Point{
			{X: g.Cx, Y: g.Cy - g.R},
			{X: g.Cx + g.R, Y: g.Cy + g.R},
			{X: g.Cx - g.R, Y: g.Cy + g.R},
		}
		pdf.Polygon(pts, style)
	case scene.ShapeDiamond:
		pts := []gopdf.Point{
			{X: g.Cx, Y: g.Cy - g.R},
			{X: g.Cx + g.R, Y: g.Cy},
			{X: g.Cx, Y: g.Cy + g.R},
			{X: g.Cx - g.R, Y: g.Cy},
		}
		pdf.Polygon(pts, style)
	case scene.ShapeCross:
		// Two perpendicular line segments. Cross marks ignore fill.
		pdf.Line(g.Cx-g.R, g.Cy, g.Cx+g.R, g.Cy)
		pdf.Line(g.Cx, g.Cy-g.R, g.Cx, g.Cy+g.R)
	default:
		drawCircleBezier(pdf, g.Cx, g.Cy, g.R, style)
	}
	return nil
}

// drawCircleBezier emits a circle as four cubic Beziers (kappa
// approximation). Matches the standard SVG-to-PDF circle decomposition
// used by every PDF library that doesn't expose a native circle
// operator beyond Oval (which gopdf only strokes).
func drawCircleBezier(pdf *gopdf.GoPdf, cx, cy, r float64, style string) {
	const kappa = 0.5522847498
	cr := r * kappa
	// Four quadrant beziers, each from (cx+r, cy) around CCW.
	pdf.Curve(cx+r, cy, cx+r, cy-cr, cx+cr, cy-r, cx, cy-r, style)
	pdf.Curve(cx, cy-r, cx-cr, cy-r, cx-r, cy-cr, cx-r, cy, style)
	pdf.Curve(cx-r, cy, cx-r, cy+cr, cx-cr, cy+r, cx, cy+r, style)
	pdf.Curve(cx, cy+r, cx+cr, cy+r, cx+r, cy+cr, cx+r, cy, style)
}

func renderRule(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Rule
	end := applyMarkStyle(pdf, m.Style)
	defer end()
	pdf.Line(g.X1, g.Y1, g.X2, g.Y2)
	return nil
}

func renderArc(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Arc
	if g.OuterR <= 0 {
		return nil
	}
	end := applyMarkStyle(pdf, m.Style)
	defer end()

	style := styleString(m.Style.Fill, m.Style.Stroke, m.Style.StrokeWidth)
	if style == "" {
		style = "F"
	}

	// Sector / donut: approximate via cubic-Bezier arc decomposition
	// plus straight radial segments. For a full ring (start ≈ end + 2π)
	// we draw outer + inner ovals; otherwise we sample the boundary
	// as a polygon.
	full := math.Abs(math.Abs(g.EndAngle-g.StartAngle)-2*math.Pi) < 1e-6
	if full && g.InnerR == 0 {
		// Full disk.
		if style == "D" {
			pdf.Oval(g.Cx-g.OuterR, g.Cy-g.OuterR, g.Cx+g.OuterR, g.Cy+g.OuterR)
		} else {
			drawCircleBezier(pdf, g.Cx, g.Cy, g.OuterR, style)
		}
		return nil
	}

	// Sample the boundary as a polygon. We use ≥ 32 segments per
	// full circle to keep the visual smooth; sectors get
	// proportionally fewer.
	segs := sampleArcPolygon(g)
	pdf.Polygon(segs, style)
	return nil
}

// sampleArcPolygon samples a sector / donut boundary as a polygon
// suitable for gopdf.Polygon emission. Returns the points in
// drawing order (outer arc forward + inner arc reverse + close).
func sampleArcPolygon(g *scene.ArcGeom) []gopdf.Point {
	sweep := g.EndAngle - g.StartAngle
	n := int(math.Ceil(math.Abs(sweep) / (math.Pi / 16)))
	if n < 8 {
		n = 8
	}
	pts := make([]gopdf.Point, 0, n*2+2)

	// Outer arc forward.
	for i := 0; i <= n; i++ {
		t := g.StartAngle + sweep*float64(i)/float64(n)
		pts = append(pts, gopdf.Point{
			X: g.Cx + g.OuterR*math.Cos(t),
			Y: g.Cy + g.OuterR*math.Sin(t),
		})
	}
	// Inner arc reverse (when InnerR > 0).
	if g.InnerR > 0 {
		for i := n; i >= 0; i-- {
			t := g.StartAngle + sweep*float64(i)/float64(n)
			pts = append(pts, gopdf.Point{
				X: g.Cx + g.InnerR*math.Cos(t),
				Y: g.Cy + g.InnerR*math.Sin(t),
			})
		}
	} else {
		// Sector — close back through the center.
		pts = append(pts, gopdf.Point{X: g.Cx, Y: g.Cy})
	}
	return pts
}

func renderText(pdf *gopdf.GoPdf, m scene.Mark, theme *scene.Theme) error {
	g := m.Text
	if g.Content == "" {
		return nil
	}
	fontName := FontSansRegular
	if isMono := m.Style.FontFamily != "" && strings.Contains(strings.ToLower(m.Style.FontFamily), "mono"); isMono {
		fontName = FontMonoRegular
	} else if m.Style.FontWeight >= 600 {
		fontName = FontSansBold
	}
	size := g.FontSize
	if size == 0 {
		size = 10
	}
	if err := pdf.SetFont(fontName, "", size); err != nil {
		return fmt.Errorf("pdf.renderText: SetFont: %w", err)
	}

	// Color from Style.Fill (defaults to theme.ColorText).
	if m.Style.Fill != nil {
		pdf.SetTextColor(m.Style.Fill.R, m.Style.Fill.G, m.Style.Fill.B)
	} else if theme != nil && theme.ColorText != nil {
		pdf.SetTextColor(theme.ColorText.R, theme.ColorText.G, theme.ColorText.B)
	}

	// Anchor handling — measure width and offset X.
	x := g.X
	if g.Anchor == scene.AnchorMiddle || g.Anchor == scene.AnchorEnd {
		w, err := pdf.MeasureTextWidth(g.Content)
		if err == nil {
			switch g.Anchor {
			case scene.AnchorMiddle:
				x -= w / 2
			case scene.AnchorEnd:
				x -= w
			}
		}
	}

	if g.Angle != 0 {
		pdf.Rotate(g.Angle, g.X, g.Y)
		defer pdf.RotateReset()
	}

	pdf.SetX(x)
	pdf.SetY(g.Y)
	return pdf.Cell(nil, g.Content)
}

func renderPath(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Path
	if g.D == "" {
		return nil
	}
	end := applyMarkStyle(pdf, m.Style)
	defer end()

	cmds, err := parsePath(g.D)
	if err != nil {
		return err
	}
	style := styleString(m.Style.Fill, m.Style.Stroke, m.Style.StrokeWidth)
	if style == "" {
		style = "F"
	}
	return emitPath(pdf, cmds, style)
}

func renderImage(pdf *gopdf.GoPdf, m scene.Mark) error {
	g := m.Image
	if !strings.HasPrefix(g.Href, "data:image/") {
		return prismerrors.New(
			"PRISM_SPEC_016",
			"PDF renderer only embeds data: URLs (D093); relative paths and remote URLs are rejected at render time.",
			map[string]any{"URL": g.Href},
		)
	}
	// Extract base64 body.
	idx := strings.Index(g.Href, ";base64,")
	if idx < 0 {
		return prismerrors.New(
			"PRISM_SPEC_016",
			"PDF image data URL must use ;base64, encoding.",
			map[string]any{"URL": g.Href},
		)
	}
	raw, err := base64.StdEncoding.DecodeString(g.Href[idx+len(";base64,"):])
	if err != nil {
		return prismerrors.New(
			"PRISM_SPEC_016",
			fmt.Sprintf("PDF image data URL base64 decode failed: %v", err),
			map[string]any{"URL": g.Href[:min(len(g.Href), 64)]},
		)
	}
	holder, err := gopdf.ImageHolderByBytes(raw)
	if err != nil {
		return fmt.Errorf("pdf.renderImage: ImageHolderByBytes: %w", err)
	}
	rect := &gopdf.Rect{W: g.W, H: g.H}
	return pdf.ImageByHolder(holder, g.X, g.Y, rect)
}
