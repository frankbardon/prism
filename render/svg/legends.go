package svg

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
)

// renderLegends emits one <g class="prism-legend"> per Scene.Legend.
// Symbol entries get a 12×12 swatch + label; gradient entries get a
// 12×120 rect filled via url(#gradient-id) with axis labels along
// the side. filterName, when non-empty, is the resolved
// theme.Legend.Filter reference (E1-S2) — applied to the outer
// prism-legends wrapper via filter="url(#prism-filter-<name>)".
// theme (may be nil) additionally carries the resolved
// LegendLabelLineHeight/LegendLabelLetterSpacing and
// LegendTitleLineHeight/LegendTitleLetterSpacing tokens (E2-S2).
func renderLegends(w *Writer, legends []scene.Legend, filterName string, theme *scene.Theme) {
	if len(legends) == 0 {
		return
	}
	w.OpenTag("g")
	w.Attr("class", "prism-legends")
	writeFilterAttr(w, filterName)
	w.CloseTagOpen()
	for _, lg := range legends {
		renderLegend(w, lg, theme)
	}
	w.EndTag("g")
}

// legendTickBaseline nudges a gradient tick label down from the stop
// it marks so the text's baseline, not its top, lines up with the
// gradient bar at that offset. It doubles as the half-leading that
// centres a solid swatch's label against the swatch.
const legendTickBaseline = 4.0

// Legend entry geometry. legendSwatchSize is the default solid swatch
// (and the width of the swatch column every label is offset from);
// legendSymbolSize the default bounding box of a shaped symbol;
// legendLabelGap the space between the swatch column and the label;
// legendRowPitch the vertical pitch a vertical legend stacks rows at.
// They mirror the encoder-side constants in encode/legend_build.go —
// the frame is measured there, the content drawn here.
const (
	legendSwatchSize = 12.0
	legendSymbolSize = 10.0
	legendLabelGap   = 6.0
	legendRowPitch   = 18.0
)

func renderLegend(w *Writer, lg scene.Legend, theme *scene.Theme) {
	w.OpenTag("g")
	w.Attr("class", "prism-legend prism-legend-"+string(lg.Channel))
	w.Attr("data-prism-legend-id", lg.ID)
	w.CloseTagOpen()

	// Resolved typography tokens (E2-S2) — nil-safe extraction once,
	// reused across the title + every entry label below.
	var labelLH, labelLS, titleLH, titleLS *float64
	if theme != nil {
		labelLH, labelLS = theme.LegendLabelLineHeight, theme.LegendLabelLetterSpacing
		titleLH, titleLS = theme.LegendTitleLineHeight, theme.LegendTitleLetterSpacing
	}

	// Interior padding (E1-S3): legend.padding insets the content on
	// every side, on top of the fixed 4-px inset below. Zero — the
	// default — leaves the geometry exactly where it was.
	pad := lg.Padding

	// Title (if any) above entries.
	const titleH = 14.0
	if lg.Title != "" {
		w.OpenTag("text")
		w.Attr("class", "prism-legend-title")
		w.AttrFloat("x", lg.Frame.X+4+pad)
		w.AttrFloat("y", lg.Frame.Y+titleH+pad)
		writeTypographyAttrs(w, titleLH, titleLS)
		w.CloseTagOpen()
		w.Text(lg.Title)
		w.EndTag("text")
	}

	rowOffset := 0.0
	if lg.Title != "" {
		rowOffset = titleH + 4
	}

	// Entry flow (E3-S4). A vertical legend — the default, and what
	// every legend drew before — stacks entries down a column at the
	// 18-px row pitch. A horizontal one lays them out across a single
	// row, splitting the frame's interior width evenly: that is the
	// same per-entry width encode.LegendBox.Size sized the frame with,
	// so the two cannot drift.
	horizontal := lg.Direction == scene.LegendHorizontal
	entryPitch := 0.0
	if horizontal && len(lg.Entries) > 0 {
		entryPitch = (lg.Frame.W - 2*pad) / float64(len(lg.Entries))
	}

	for i, entry := range lg.Entries {
		sx := lg.Frame.X + 4 + pad
		y := lg.Frame.Y + pad + rowOffset + 8
		if horizontal {
			sx += float64(i) * entryPitch
		} else {
			y += float64(i) * legendRowPitch
		}
		// The swatch column is as wide as the swatch itself, never
		// narrower than the 12-px default, so a larger symbol_size
		// pushes the label out instead of drawing under it.
		size := swatchSize(entry.Swatch)
		colW := size
		if colW < legendSwatchSize {
			colW = legendSwatchSize
		}
		labelX := sx + colW + legendLabelGap
		switch entry.Swatch.Type {
		case scene.SwatchSolid:
			// Square swatch + label.
			w.OpenTag("rect")
			w.Attr("class", "prism-legend-swatch")
			w.AttrFloat("x", sx)
			w.AttrFloat("y", y)
			w.AttrFloat("width", size)
			w.AttrFloat("height", size)
			if entry.Swatch.Color != nil {
				w.Attr("fill", entry.Swatch.Color.CSS())
			}
			w.SelfClose()
			w.OpenTag("text")
			w.Attr("class", "prism-legend-label")
			w.AttrFloat("x", labelX)
			w.AttrFloat("y", y+size/2+legendTickBaseline)
			writeTypographyAttrs(w, labelLH, labelLS)
			w.CloseTagOpen()
			w.Text(entry.Label)
			w.EndTag("text")
		case scene.SwatchGradient:
			// 12-wide × Frame.H-tall rect filled with the gradient.
			barH := lg.Frame.H - rowOffset - 16 - 2*pad
			w.OpenTag("rect")
			w.Attr("class", "prism-legend-swatch")
			w.AttrFloat("x", sx)
			w.AttrFloat("y", y)
			w.AttrFloat("width", legendSwatchSize)
			w.AttrFloat("height", barH)
			w.Attr("fill", fmt.Sprintf("url(#%s)", entry.Swatch.GradientID))
			w.SelfClose()
			// legend.tick_count labelled stops run down the bar, the
			// first level with its top and the last with its bottom.
			// An entry with no ticks falls back to its own summary
			// label, which is what a gradient legend drew before E3-S3.
			if len(entry.Ticks) > 0 {
				for _, tk := range entry.Ticks {
					w.OpenTag("text")
					w.Attr("class", "prism-legend-label")
					w.AttrFloat("x", sx+legendSwatchSize+legendLabelGap)
					w.AttrFloat("y", y+tk.Offset*barH+legendTickBaseline)
					writeTypographyAttrs(w, labelLH, labelLS)
					w.CloseTagOpen()
					w.Text(tk.Label)
					w.EndTag("text")
				}
				break
			}
			w.OpenTag("text")
			w.Attr("class", "prism-legend-label")
			w.AttrFloat("x", sx+legendSwatchSize+legendLabelGap)
			w.AttrFloat("y", y+10)
			writeTypographyAttrs(w, labelLH, labelLS)
			w.CloseTagOpen()
			w.Text(entry.Label)
			w.EndTag("text")
		case scene.SwatchSymbol:
			// legend.symbol_type: the shape is drawn by the SAME
			// emitter renderPoint uses (render/svg/symbols.go), so a
			// point mark's diamond and this swatch's diamond are one
			// geometry. The symbol is centred in the swatch column.
			tag := symbolTag(entry.Swatch.Shape)
			w.OpenTag(tag)
			w.Attr("class", "prism-legend-symbol")
			writeSymbolGeom(w, entry.Swatch.Shape,
				sx+legendSwatchSize/2, y+legendSwatchSize/2, size/2)
			if entry.Swatch.Color != nil {
				w.Attr("fill", entry.Swatch.Color.CSS())
			}
			w.SelfClose()
			w.OpenTag("text")
			w.Attr("class", "prism-legend-label")
			w.AttrFloat("x", labelX)
			w.AttrFloat("y", y+10)
			writeTypographyAttrs(w, labelLH, labelLS)
			w.CloseTagOpen()
			w.Text(entry.Label)
			w.EndTag("text")
		}
	}

	w.EndTag("g")
}

// swatchSize resolves a swatch's drawn pixel extent: legend.symbol_size
// when the encoder set one, else the per-form default — a 12-px solid
// square, a 10-px symbol bounding box. Those defaults are what keep
// every committed legend golden byte-identical.
func swatchSize(sw scene.SwatchSpec) float64 {
	if sw.Size > 0 {
		return sw.Size
	}
	if sw.Type == scene.SwatchSymbol {
		return legendSymbolSize
	}
	return legendSwatchSize
}

// renderDefs emits a single <defs> block for scene-level resources
// (gradients, patterns, clips). Called once per Scene.
func renderDefs(w *Writer, defs *scene.Defs) {
	if defs == nil {
		return
	}
	if len(defs.Gradients) == 0 && len(defs.Patterns) == 0 && len(defs.Clips) == 0 {
		return
	}
	w.OpenTag("defs")
	w.CloseTagOpen()
	// Sorted, because a Go map iterates in random order and the SVG
	// bytes are a golden / cross-impl contract. Harmless while a
	// scene carries a single gradient; load-bearing the moment one
	// carries two.
	gradientIDs := make([]string, 0, len(defs.Gradients))
	for id := range defs.Gradients {
		gradientIDs = append(gradientIDs, id)
	}
	sort.Strings(gradientIDs)
	for _, id := range gradientIDs {
		g := defs.Gradients[id]
		switch g.Type {
		case "linear":
			w.OpenTag("linearGradient")
			w.Attr("id", id)
			w.AttrFloat("x1", g.X1)
			w.AttrFloat("y1", g.Y1)
			w.AttrFloat("x2", g.X2)
			w.AttrFloat("y2", g.Y2)
			w.CloseTagOpen()
			for _, s := range g.Stops {
				w.OpenTag("stop")
				w.AttrFloat("offset", s.Offset)
				w.Attr("stop-color", (&s.Color).CSS())
				w.SelfClose()
			}
			w.EndTag("linearGradient")
		case "radial":
			// Radial: cx/cy = X1/Y1 (center), r = X2 (radius).
			w.OpenTag("radialGradient")
			w.Attr("id", id)
			w.AttrFloat("cx", g.X1)
			w.AttrFloat("cy", g.Y1)
			w.AttrFloat("r", g.X2)
			w.CloseTagOpen()
			for _, s := range g.Stops {
				w.OpenTag("stop")
				w.AttrFloat("offset", s.Offset)
				w.Attr("stop-color", (&s.Color).CSS())
				w.SelfClose()
			}
			w.EndTag("radialGradient")
		}
	}
	for id, c := range defs.Clips {
		w.OpenTag("clipPath")
		w.Attr("id", id)
		w.CloseTagOpen()
		w.OpenTag("rect")
		w.AttrFloat("x", c.X)
		w.AttrFloat("y", c.Y)
		w.AttrFloat("width", c.W)
		w.AttrFloat("height", c.H)
		w.SelfClose()
		w.EndTag("clipPath")
	}
	w.EndTag("defs")
}
