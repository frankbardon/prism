package svg

import "github.com/frankbardon/prism/encode/scene"

// renderAxis emits the axis domain line, tick lines + labels, grid
// lines, and (optional) title for one resolved scene.Axis. Geometry
// is dispatched per AxisPosition: bottom/top render horizontally,
// left/right render vertically.
func renderAxis(w *Writer, a scene.Axis, plot scene.Rect) {
	w.OpenTag("g")
	w.Attr("class", "prism-axis prism-axis-"+string(a.Channel))
	w.Attr("data-prism-axis-id", a.ID)
	w.CloseTagOpen()

	// Grid lines first (so axis lines + ticks render on top).
	if len(a.Grid) > 0 {
		for _, line := range a.Grid {
			w.OpenTag("line")
			w.Attr("class", "prism-grid-line")
			w.AttrFloat("x1", line.X1)
			w.AttrFloat("y1", line.Y1)
			w.AttrFloat("x2", line.X2)
			w.AttrFloat("y2", line.Y2)
			w.SelfClose()
		}
	}

	// Domain line.
	w.OpenTag("line")
	w.Attr("class", "prism-axis-domain")
	w.AttrFloat("x1", a.Domain.X1)
	w.AttrFloat("y1", a.Domain.Y1)
	w.AttrFloat("x2", a.Domain.X2)
	w.AttrFloat("y2", a.Domain.Y2)
	w.SelfClose()

	// Ticks + labels.
	switch a.Position {
	case scene.AxisPositionBottom:
		for _, t := range a.Ticks {
			// Vertical tick mark from the domain line downward.
			w.OpenTag("line")
			w.Attr("class", "prism-axis-tick")
			w.AttrFloat("x1", t.Pixel)
			w.AttrFloat("y1", plot.Bottom())
			w.AttrFloat("x2", t.Pixel)
			w.AttrFloat("y2", plot.Bottom()+5)
			w.SelfClose()
			// Label below the tick.
			w.OpenTag("text")
			w.Attr("class", "prism-axis-label")
			w.AttrFloat("x", t.Pixel)
			w.AttrFloat("y", plot.Bottom()+18)
			w.Attr("text-anchor", "middle")
			w.CloseTagOpen()
			w.Text(t.Label)
			w.EndTag("text")
		}
	case scene.AxisPositionLeft:
		for _, t := range a.Ticks {
			w.OpenTag("line")
			w.Attr("class", "prism-axis-tick")
			w.AttrFloat("x1", plot.X-5)
			w.AttrFloat("y1", t.Pixel)
			w.AttrFloat("x2", plot.X)
			w.AttrFloat("y2", t.Pixel)
			w.SelfClose()
			w.OpenTag("text")
			w.Attr("class", "prism-axis-label")
			w.AttrFloat("x", plot.X-8)
			w.AttrFloat("y", t.Pixel+4)
			w.Attr("text-anchor", "end")
			w.CloseTagOpen()
			w.Text(t.Label)
			w.EndTag("text")
		}
	}

	// Title (one per axis).
	if a.Title != "" {
		w.OpenTag("text")
		w.Attr("class", "prism-axis-title")
		switch a.Position {
		case scene.AxisPositionBottom:
			w.AttrFloat("x", plot.CenterX())
			w.AttrFloat("y", plot.Bottom()+34)
			w.Attr("text-anchor", "middle")
		case scene.AxisPositionLeft:
			w.AttrFloat("x", plot.X-30)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr(plot.X-30, plot.CenterY()))
		}
		w.CloseTagOpen()
		w.Text(a.Title)
		w.EndTag("text")
	}

	w.EndTag("g")
}

// rotateAttr returns a CSS transform string that rotates -90 degrees
// around (x, y) — used for vertical y-axis titles.
func rotateAttr(x, y float64) string {
	return "rotate(-90 " + floatStr(x) + " " + floatStr(y) + ")"
}

// floatStr is render.FormatFloat aliased locally so axes.go does not
// have to import the render package directly. This avoids a circular
// import for marks.go's render.FormatFloat usage.
func floatStr(v float64) string {
	return formatF(v)
}
