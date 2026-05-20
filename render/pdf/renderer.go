// Package pdf is the Prism PDF renderer. It walks the Scene IR
// directly and emits PDF drawing primitives via
// github.com/signintech/gopdf (D088). Vector preserved throughout:
// no SVG → PNG bridge, no rasterisation; every mark dispatches to a
// per-geom handler that emits PDF path / curve / image operators.
//
// Default fonts (Inter regular + bold, JetBrains Mono regular —
// D089) are bundled under render/pdf/fonts/ as TTF subsets embedded
// at compile time via go:embed. The --font-dir CLI flag lets users
// override with their own .ttf set; missing canonical names fall
// back to the bundle.
//
// Multi-page output via RenderOpts.Paginate: a SceneGrid with N
// cells produces an N-page PDF when Paginate is true; otherwise the
// whole grid renders to a single page sized to the outer frame.
//
// Theme handoff per D090: PDF reads SceneDoc.Theme RGB triplets
// directly (NOT the Theme.CSS string the JS port consumes). Alpha
// per D091: per-mark alpha applied via ExtGState; gradient fills
// flatten to the first stop with PRISM_WARN_PDF_GRADIENT_FLATTENED
// appended to SceneDoc.Warnings.
package pdf

import (
	"fmt"
	"math"

	"github.com/signintech/gopdf"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// Renderer is the PDF implementation of render.Renderer. Stateless
// between calls; safe to share across goroutines (gopdf state is
// created fresh inside Render).
type Renderer struct {
	fontDir string
}

// Option mutates a Renderer at construction time.
type Option func(*Renderer)

// WithFontDir overrides the bundled fonts. Files named
// prism-sans-regular.ttf / prism-sans-bold.ttf /
// prism-mono-regular.ttf inside the dir take precedence; a
// case-insensitive scan handles common upstream names (Inter-*.ttf,
// JetBrainsMono-*.ttf).
func WithFontDir(dir string) Option {
	return func(r *Renderer) { r.fontDir = dir }
}

// New returns a PDF renderer configured by the supplied options.
// Zero options = bundled fonts, default page size from the
// SceneDoc's outer frame.
func New(opts ...Option) *Renderer {
	r := &Renderer{}
	for _, o := range opts {
		o(r)
	}
	return r
}

// MimeType implements render.Renderer.
func (r *Renderer) MimeType() string { return "application/pdf" }

// Render implements render.Renderer. Walks the SceneDoc top-down:
//
//   - SceneGrid → AddPage per cell (Paginate=true) or one outer page
//     with all cells (Paginate=false)
//   - Scene → applyTheme background fill (when non-transparent)
//   - SceneLayer → ordered by slice index (D052)
//   - Mark → renderMark dispatches to one of nine per-geom handlers
//
// Warnings raised at render time (gradient flatten, unsupported
// path commands recovered as partial output) append to doc.Warnings
// — the caller surfaces them.
func (r *Renderer) Render(doc *scene.SceneDoc, opts render.RenderOpts) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("pdf.Render: nil SceneDoc")
	}
	theme := opts.Theme
	if theme == nil {
		theme = doc.Theme
	}
	if theme == nil {
		theme = scene.Default()
	}

	pdf := &gopdf.GoPdf{}

	// Default page size from outerFrame or supplied Width/Height.
	frame := outerFrame(doc.Grid)
	if frame.W == 0 || frame.H == 0 {
		frame = scene.Rect{W: 595, H: 842} // A4 portrait pt
	}
	pageW := opts.Width
	if pageW == 0 {
		pageW = frame.W
	}
	pageH := opts.Height
	if pageH == 0 {
		pageH = frame.H
	}

	pdf.Start(gopdf.Config{
		PageSize: gopdf.Rect{W: pageW, H: pageH},
	})

	if err := loadFonts(pdf, r.fontDir); err != nil {
		return nil, fmt.Errorf("pdf.Render: %w", err)
	}

	// Default font for any subsequent Cell calls that fire before a
	// text mark sets its own. SetFont errors only when the family
	// isn't registered; we just registered it, so this is paranoid.
	_ = pdf.SetFont(FontSansRegular, "", 10)

	if opts.Paginate && len(doc.Grid.Cells) > 1 {
		for i := range doc.Grid.Cells {
			cell := &doc.Grid.Cells[i]
			cellW, cellH := cell.Scene.Frame.W, cell.Scene.Frame.H
			if cellW == 0 {
				cellW = pageW
			}
			if cellH == 0 {
				cellH = pageH
			}
			pdf.AddPageWithOption(gopdf.PageOption{
				PageSize: &gopdf.Rect{W: cellW, H: cellH},
			})
			// Translate the cell scene to (0, 0) — the encoder
			// pre-offsets cells into the outer-grid coordinate
			// system, so we subtract the cell's frame origin to
			// re-root it on its own page.
			offsetScene := cell.Scene
			if cell.Scene.Frame.X != 0 || cell.Scene.Frame.Y != 0 {
				offsetScene = translateScene(cell.Scene, -cell.Scene.Frame.X, -cell.Scene.Frame.Y)
			}
			if err := renderScene(pdf, &offsetScene, theme, doc); err != nil {
				return nil, err
			}
		}
	} else {
		pdf.AddPage()
		applyBackground(pdf, theme, pageW, pageH)
		for i := range doc.Grid.Cells {
			cell := &doc.Grid.Cells[i]
			if err := renderScene(pdf, &cell.Scene, theme, doc); err != nil {
				return nil, err
			}
		}
	}

	return pdf.GetBytesPdfReturnErr()
}

// applyBackground paints a page-fill rect when theme.Background is
// a concrete color. "transparent" / empty = no fill emitted.
func applyBackground(pdf *gopdf.GoPdf, theme *scene.Theme, w, h float64) {
	if theme == nil || theme.Background == "" || theme.Background == "transparent" {
		return
	}
	c, err := scene.ColorFromHex(theme.Background)
	if err != nil || c == nil {
		return
	}
	pdf.SetFillColor(c.R, c.G, c.B)
	pdf.RectFromUpperLeftWithStyle(0, 0, w, h, "F")
}

// renderScene walks one Scene IR's layers + marks in z-order.
// Annotations / axes / legends are NOT yet emitted by the PDF
// renderer (deferred to a follow-up); the v1 surface focuses on
// mark dispatch + page composition. The encoder still produces
// these fields for SVG / canvas consumers, so this is a renderer-
// level omission, not an IR-level loss.
func renderScene(pdf *gopdf.GoPdf, s *scene.Scene, theme *scene.Theme, doc *scene.SceneDoc) error {
	// Frame background — same offset semantics as the SVG renderer.
	if s.Frame.W > 0 && s.Frame.H > 0 && theme != nil &&
		theme.Background != "" && theme.Background != "transparent" {
		// Already painted on the outer page; per-scene background
		// would double-paint when multiple cells share one page.
		// Skip.
	}

	// Title / subtitle — emit as text marks at the encoded
	// positions.
	if s.Title != nil && s.Title.Content != "" {
		_ = renderTextElement(pdf, s.Title, theme, FontSansBold, 14)
	}
	if s.Subtitle != nil && s.Subtitle.Content != "" {
		_ = renderTextElement(pdf, s.Subtitle, theme, FontSansRegular, 10)
	}

	// Axes — light-touch implementation: tick marks + labels via
	// the same text-render path. Full axis polish (grid lines,
	// titles) stays SVG-only in v1; document in SUMMARY.
	for _, ax := range s.Axes {
		renderAxis(pdf, ax, theme)
	}

	for _, layer := range s.Layers {
		for _, m := range layer.Marks {
			if err := renderMark(pdf, m, theme); err != nil {
				// Surface as warning, continue rendering — a single
				// unsupported mark shouldn't blank the whole page.
				appendWarning(doc, "PRISM_RENDER_001", fmt.Sprintf("mark in layer %s skipped: %v", layer.ID, err))
			}
		}
	}
	return nil
}

func renderTextElement(pdf *gopdf.GoPdf, el *scene.TextElement, theme *scene.Theme, defaultFont string, defaultSize float64) error {
	font := defaultFont
	size := defaultSize
	if el.Style.FontFamily != "" {
		// Match SVG's convention: any "mono" hint → mono font.
		font = pickFont(el.Style, defaultFont)
	}
	if err := pdf.SetFont(font, "", size); err != nil {
		return err
	}
	if el.Style.Fill != nil {
		pdf.SetTextColor(el.Style.Fill.R, el.Style.Fill.G, el.Style.Fill.B)
	} else if theme != nil && theme.ColorText != nil {
		pdf.SetTextColor(theme.ColorText.R, theme.ColorText.G, theme.ColorText.B)
	}
	pdf.SetX(el.X)
	pdf.SetY(el.Y)
	return pdf.Cell(nil, el.Content)
}

func pickFont(s scene.Style, fallback string) string {
	if s.FontFamily == "" {
		return fallback
	}
	// crude mono detection — matches SVG's css-class heuristic.
	for _, h := range []string{"mono", "Mono", "MONO"} {
		if contains(s.FontFamily, h) {
			return FontMonoRegular
		}
	}
	if s.FontWeight >= 600 {
		return FontSansBold
	}
	return FontSansRegular
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// renderAxis emits axis tick labels + the axis domain line as PDF
// primitives. Tick-mark glyphs, grid lines, and axis titles are
// deferred to a follow-up; v1 is "labels + baseline" which is
// enough to read the chart while keeping the PDF output minimal.
//
// Tick label placement: derived from the axis Position + Tick.Pixel.
// For bottom / top axes the label sits at (Pixel, Domain.Y1 ± pad);
// for left / right axes at (Domain.X1 ± pad, Pixel). Pad is a small
// fixed offset (3pt) — close enough to the SVG renderer's defaults
// for legibility.
func renderAxis(pdf *gopdf.GoPdf, ax scene.Axis, theme *scene.Theme) {
	if theme != nil && theme.ColorAxis != nil {
		pdf.SetStrokeColor(theme.ColorAxis.R, theme.ColorAxis.G, theme.ColorAxis.B)
		pdf.SetTextColor(theme.ColorAxis.R, theme.ColorAxis.G, theme.ColorAxis.B)
	}
	// Domain line.
	if ax.Domain.X1 != 0 || ax.Domain.Y1 != 0 || ax.Domain.X2 != 0 || ax.Domain.Y2 != 0 {
		pdf.SetLineWidth(0.5)
		pdf.Line(ax.Domain.X1, ax.Domain.Y1, ax.Domain.X2, ax.Domain.Y2)
	}
	if len(ax.Ticks) == 0 {
		return
	}
	_ = pdf.SetFont(FontSansRegular, "", 9)
	const pad = 12.0
	for _, t := range ax.Ticks {
		if t.Label == "" || t.LabelHidden {
			continue
		}
		var lx, ly float64
		switch ax.Position {
		case scene.AxisPositionBottom:
			lx = t.Pixel - 6
			ly = ax.Domain.Y1 + pad
		case scene.AxisPositionTop:
			lx = t.Pixel - 6
			ly = ax.Domain.Y1 - pad
		case scene.AxisPositionLeft:
			lx = ax.Domain.X1 - pad - 20
			ly = t.Pixel - 4
		case scene.AxisPositionRight:
			lx = ax.Domain.X1 + pad
			ly = t.Pixel - 4
		default:
			continue
		}
		pdf.SetX(lx)
		pdf.SetY(ly)
		_ = pdf.Cell(nil, t.Label)
	}
}

// outerFrame computes the bounding box of every cell in the grid.
// Used to derive the default page size when the caller doesn't
// supply Width / Height. Matches the SVG renderer's outerFrame
// helper.
func outerFrame(g scene.SceneGrid) scene.Rect {
	if len(g.Cells) == 0 {
		return scene.Rect{}
	}
	var maxR, maxB float64
	for _, c := range g.Cells {
		if r := c.Scene.Frame.X + c.Scene.Frame.W; r > maxR {
			maxR = r
		}
		if b := c.Scene.Frame.Y + c.Scene.Frame.H; b > maxB {
			maxB = b
		}
	}
	return scene.Rect{X: 0, Y: 0, W: maxR, H: maxB}
}

// translateScene returns a copy of s with every coordinate offset
// by (dx, dy). Used by the paginate branch to re-root each cell at
// (0, 0) on its own page.
func translateScene(s scene.Scene, dx, dy float64) scene.Scene {
	out := s
	out.Frame.X += dx
	out.Frame.Y += dy
	out.Plot.X += dx
	out.Plot.Y += dy
	if out.Title != nil {
		t := *out.Title
		t.X += dx
		t.Y += dy
		out.Title = &t
	}
	if out.Subtitle != nil {
		t := *out.Subtitle
		t.X += dx
		t.Y += dy
		out.Subtitle = &t
	}
	axes := make([]scene.Axis, len(s.Axes))
	for i, a := range s.Axes {
		axes[i] = translateAxis(a, dx, dy)
	}
	out.Axes = axes
	layers := make([]scene.SceneLayer, len(s.Layers))
	for i, l := range s.Layers {
		layers[i] = translateLayer(l, dx, dy)
	}
	out.Layers = layers
	return out
}

func translateAxis(a scene.Axis, dx, dy float64) scene.Axis {
	out := a
	ticks := make([]scene.Tick, len(a.Ticks))
	for i, t := range a.Ticks {
		// Ticks carry a single Pixel coord along the axis; the
		// perpendicular axis placement is implicit in the axis
		// Position. Translation only affects whichever axis
		// orientation matches the channel.
		switch a.Channel {
		case scene.ChannelX:
			t.Pixel += dx
		case scene.ChannelY:
			t.Pixel += dy
		}
		ticks[i] = t
	}
	out.Ticks = ticks
	// Domain line + grid translate along both axes.
	out.Domain.X1 += dx
	out.Domain.X2 += dx
	out.Domain.Y1 += dy
	out.Domain.Y2 += dy
	grid := make([]scene.Line, len(a.Grid))
	for i, g := range a.Grid {
		g.X1 += dx
		g.X2 += dx
		g.Y1 += dy
		g.Y2 += dy
		grid[i] = g
	}
	out.Grid = grid
	return out
}

func translateLayer(l scene.SceneLayer, dx, dy float64) scene.SceneLayer {
	out := l
	marks := make([]scene.Mark, len(l.Marks))
	for i, m := range l.Marks {
		marks[i] = translateMark(m, dx, dy)
	}
	out.Marks = marks
	return out
}

func translateMark(m scene.Mark, dx, dy float64) scene.Mark {
	out := m
	if m.Rect != nil {
		r := *m.Rect
		r.X += dx
		r.Y += dy
		out.Rect = &r
	}
	if m.Line != nil {
		l := *m.Line
		l.Points = translatePoints(m.Line.Points, dx, dy)
		out.Line = &l
	}
	if m.Area != nil {
		a := *m.Area
		a.Upper = translatePoints(m.Area.Upper, dx, dy)
		a.Lower = translatePoints(m.Area.Lower, dx, dy)
		out.Area = &a
	}
	if m.Point != nil {
		p := *m.Point
		p.Cx += dx
		p.Cy += dy
		out.Point = &p
	}
	if m.Rule != nil {
		r := *m.Rule
		r.X1 += dx
		r.X2 += dx
		r.Y1 += dy
		r.Y2 += dy
		out.Rule = &r
	}
	if m.Arc != nil {
		a := *m.Arc
		a.Cx += dx
		a.Cy += dy
		out.Arc = &a
	}
	if m.Text != nil {
		t := *m.Text
		t.X += dx
		t.Y += dy
		out.Text = &t
	}
	if m.Image != nil {
		im := *m.Image
		im.X += dx
		im.Y += dy
		out.Image = &im
	}
	// Path is a passthrough d-string — translation would require
	// path-data rewriting, which v1 doesn't support. Paths in
	// paginate mode render at their original encoded coordinates;
	// most paginate use cases use composite mark fixtures that
	// don't emit raw PathGeom marks at top level.
	return out
}

func translatePoints(in [][2]float64, dx, dy float64) [][2]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make([][2]float64, len(in))
	for i, p := range in {
		out[i] = [2]float64{p[0] + dx, p[1] + dy}
	}
	return out
}

// appendWarning attaches a SceneWarning to doc when one isn't
// already present with the same code+message pair. Idempotent so
// repeated identical issues don't flood the output.
func appendWarning(doc *scene.SceneDoc, code, msg string) {
	for _, w := range doc.Warnings {
		if w.Code == code && w.Message == msg {
			return
		}
	}
	doc.Warnings = append(doc.Warnings, scene.Warning{Code: code, Message: msg})
}

// degToRad returns d * pi / 180.
func degToRad(d float64) float64 { return d * math.Pi / 180 }
