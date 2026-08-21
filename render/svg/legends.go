package svg

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// renderLegends emits one <g class="prism-legend"> per Scene.Legend.
// Swatch extent and row pitch come from the legend's own resolved
// tokens; entry positions come from the encoder.
func renderLegends(w *Writer, legends []scene.Legend) {
	if len(legends) == 0 {
		return
	}
	w.OpenTag("g")
	w.Attr("class", "prism-legends")
	w.CloseTagOpen()
	for _, lg := range legends {
		renderLegend(w, lg)
	}
	w.EndTag("g")
}

// renderLegend emits one legend block.
//
// Entry POSITIONS arrive resolved on each LegendEntry rather than
// being re-derived here from a hardcoded 18px row pitch. The encoder
// computed them against the same metrics it reserved the frame with,
// which is what keeps a horizontal legend's ragged wrapping inside
// the space set aside for it.
func renderLegend(w *Writer, lg scene.Legend) {
	w.OpenTag("g")
	w.Attr("class", "prism-legend prism-legend-"+string(lg.Channel))
	w.Attr("data-prism-legend-id", lg.ID)
	w.CloseTagOpen()

	sym := lg.SymbolSize
	if sym <= 0 {
		sym = 10
	}
	if lg.Title != "" {
		w.OpenTag("text")
		w.Attr("class", "prism-legend-title")
		w.AttrFloat("x", lg.Frame.X)
		w.AttrFloat("y", lg.Frame.Y+sym)
		w.CloseTagOpen()
		w.Text(lg.Title)
		w.EndTag("text")
	}

	for _, entry := range lg.Entries {
		x := lg.Frame.X + entry.X
		y := lg.Frame.Y + entry.Y
		labelX := x + sym*1.6
		// Centre the label's baseline on the swatch rather than on the
		// row, so a swatch and its name read as one object.
		labelY := y + sym*0.85
		switch entry.Swatch.Type {
		case scene.SwatchGradient:
			w.OpenTag("rect")
			w.Attr("class", "prism-legend-swatch")
			w.AttrFloat("x", x)
			w.AttrFloat("y", y)
			w.AttrFloat("width", sym*1.1)
			w.AttrFloat("height", gradientBarLength)
			w.Attr("fill", fmt.Sprintf("url(#%s)", entry.Swatch.GradientID))
			w.SelfClose()
			// A continuous ramp is labelled at its ends, not in the
			// middle: the label carries "max min" and each half anchors
			// to the end it describes.
			hi, lo := splitRangeLabel(entry.Label)
			emitLegendLabel(w, hi, "", x+sym*1.6, y+sym*0.8)
			emitLegendLabel(w, lo, "", x+sym*1.6, y+gradientBarLength)
		case scene.SwatchSymbol:
			w.OpenTag("circle")
			w.Attr("class", "prism-legend-symbol")
			w.AttrFloat("cx", x+sym/2)
			w.AttrFloat("cy", y+sym/2)
			w.AttrFloat("r", sym/2)
			if entry.Swatch.Color != nil {
				w.Attr("fill", entry.Swatch.Color.CSS())
			}
			w.SelfClose()
			emitLegendLabel(w, entry.Label, entry.Full, labelX, labelY)
		default:
			w.OpenTag("rect")
			w.Attr("class", "prism-legend-swatch")
			w.AttrFloat("x", x)
			w.AttrFloat("y", y)
			w.AttrFloat("width", sym)
			w.AttrFloat("height", sym)
			w.Attr("rx", "var(--prism-legend-symbol-corner-radius, 2)")
			if entry.Swatch.Color != nil {
				w.Attr("fill", entry.Swatch.Color.CSS())
			}
			w.SelfClose()
			emitLegendLabel(w, entry.Label, entry.Full, labelX, labelY)
		}
	}

	w.EndTag("g")
}

// gradientBarLength mirrors the encoder's reservation for a
// continuous legend's colour bar.
const gradientBarLength = 120

// emitLegendLabel writes one legend label, attaching the untruncated
// text as a <title> when the encoder shortened it.
func emitLegendLabel(w *Writer, label, full string, x, y float64) {
	if label == "" {
		return
	}
	w.OpenTag("text")
	w.Attr("class", "prism-legend-label")
	w.AttrFloat("x", x)
	w.AttrFloat("y", y)
	w.CloseTagOpen()
	w.Text(label)
	if full != "" {
		w.OpenTag("title")
		w.CloseTagOpen()
		w.Text(full)
		w.EndTag("title")
	}
	w.EndTag("text")
}

// splitRangeLabel splits the gradient legend's "max min" label into
// its two ends. A label without the separator is treated as the high
// end alone.
func splitRangeLabel(s string) (hi, lo string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
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
	for id, g := range defs.Gradients {
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
