package svg

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// renderLegends emits one <g class="prism-legend"> per Scene.Legend.
// Symbol entries get a 12×12 swatch + label; gradient entries get a
// 12×120 rect filled via url(#gradient-id) with axis labels along
// the side.
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

func renderLegend(w *Writer, lg scene.Legend) {
	w.OpenTag("g")
	w.Attr("class", "prism-legend prism-legend-"+string(lg.Channel))
	w.Attr("data-prism-legend-id", lg.ID)
	w.CloseTagOpen()

	// Title (if any) above entries.
	const titleH = 14.0
	if lg.Title != "" {
		w.OpenTag("text")
		w.Attr("class", "prism-legend-title")
		w.AttrFloat("x", lg.Frame.X+4)
		w.AttrFloat("y", lg.Frame.Y+titleH)
		w.CloseTagOpen()
		w.Text(lg.Title)
		w.EndTag("text")
	}

	rowOffset := 0.0
	if lg.Title != "" {
		rowOffset = titleH + 4
	}

	for i, entry := range lg.Entries {
		y := lg.Frame.Y + rowOffset + float64(i)*18 + 8
		switch entry.Swatch.Type {
		case scene.SwatchSolid:
			// 12x12 swatch + label.
			w.OpenTag("rect")
			w.Attr("class", "prism-legend-swatch")
			w.AttrFloat("x", lg.Frame.X+4)
			w.AttrFloat("y", y)
			w.AttrFloat("width", 12)
			w.AttrFloat("height", 12)
			if entry.Swatch.Color != nil {
				w.Attr("fill", entry.Swatch.Color.CSS())
			}
			w.SelfClose()
			w.OpenTag("text")
			w.Attr("class", "prism-legend-label")
			w.AttrFloat("x", lg.Frame.X+22)
			w.AttrFloat("y", y+10)
			w.CloseTagOpen()
			w.Text(entry.Label)
			w.EndTag("text")
		case scene.SwatchGradient:
			// 12-wide × Frame.H-tall rect filled with the gradient.
			w.OpenTag("rect")
			w.Attr("class", "prism-legend-swatch")
			w.AttrFloat("x", lg.Frame.X+4)
			w.AttrFloat("y", y)
			w.AttrFloat("width", 12)
			w.AttrFloat("height", lg.Frame.H-rowOffset-16)
			w.Attr("fill", fmt.Sprintf("url(#%s)", entry.Swatch.GradientID))
			w.SelfClose()
			w.OpenTag("text")
			w.Attr("class", "prism-legend-label")
			w.AttrFloat("x", lg.Frame.X+22)
			w.AttrFloat("y", y+10)
			w.CloseTagOpen()
			w.Text(entry.Label)
			w.EndTag("text")
		case scene.SwatchSymbol:
			// Reuse the solid swatch shape with a class hint.
			w.OpenTag("circle")
			w.Attr("class", "prism-legend-symbol")
			w.AttrFloat("cx", lg.Frame.X+10)
			w.AttrFloat("cy", y+6)
			w.AttrFloat("r", 5)
			if entry.Swatch.Color != nil {
				w.Attr("fill", entry.Swatch.Color.CSS())
			}
			w.SelfClose()
			w.OpenTag("text")
			w.Attr("class", "prism-legend-label")
			w.AttrFloat("x", lg.Frame.X+22)
			w.AttrFloat("y", y+10)
			w.CloseTagOpen()
			w.Text(entry.Label)
			w.EndTag("text")
		}
	}

	w.EndTag("g")
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
