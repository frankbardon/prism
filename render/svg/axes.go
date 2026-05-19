package svg

import "github.com/frankbardon/prism/encode/scene"

// renderAxis emits the axis domain line, tick lines + labels, grid
// lines, and (optional) title for one resolved scene.Axis. Geometry
// is dispatched per AxisPosition: bottom/top render horizontally,
// left/right render vertically. Minor ticks render shorter (3px) and
// without labels; LabelHidden tick labels are skipped (the tick mark
// itself stays).
func renderAxis(w *Writer, a scene.Axis, plot scene.Rect) {
	w.OpenTag("g")
	w.Attr("class", "prism-axis prism-axis-"+string(a.Channel))
	w.Attr("data-prism-axis-id", a.ID)
	w.CloseTagOpen()

	// Grid lines first (so axis lines + ticks render on top).
	for _, line := range a.Grid {
		w.OpenTag("line")
		w.Attr("class", "prism-grid-line")
		w.AttrFloat("x1", line.X1)
		w.AttrFloat("y1", line.Y1)
		w.AttrFloat("x2", line.X2)
		w.AttrFloat("y2", line.Y2)
		w.SelfClose()
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
			emitTickMark(w, t.Pixel, plot.Bottom(), 0, tickLen(t), true)
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t.Label, t.Pixel, plot.Bottom()+18, "middle", a.LabelAngle)
			}
		}
	case scene.AxisPositionTop:
		for _, t := range a.Ticks {
			emitTickMark(w, t.Pixel, plot.Y, 0, -tickLen(t), true)
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t.Label, t.Pixel, plot.Y-8, "middle", a.LabelAngle)
			}
		}
	case scene.AxisPositionLeft:
		for _, t := range a.Ticks {
			emitTickMark(w, plot.X, t.Pixel, -tickLen(t), 0, false)
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t.Label, plot.X-8, t.Pixel+4, "end", a.LabelAngle)
			}
		}
	case scene.AxisPositionRight:
		for _, t := range a.Ticks {
			emitTickMark(w, plot.Right(), t.Pixel, tickLen(t), 0, false)
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t.Label, plot.Right()+8, t.Pixel+4, "start", a.LabelAngle)
			}
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
		case scene.AxisPositionTop:
			w.AttrFloat("x", plot.CenterX())
			w.AttrFloat("y", plot.Y-28)
			w.Attr("text-anchor", "middle")
		case scene.AxisPositionLeft:
			w.AttrFloat("x", plot.X-30)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr(plot.X-30, plot.CenterY()))
		case scene.AxisPositionRight:
			w.AttrFloat("x", plot.Right()+30)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr90(plot.Right()+30, plot.CenterY()))
		}
		w.CloseTagOpen()
		w.Text(a.Title)
		w.EndTag("text")
	}

	w.EndTag("g")
}

// tickLen returns the pixel length of the tick mark. Minor ticks are
// half the length of majors.
func tickLen(t scene.Tick) float64 {
	if t.Minor {
		return 3
	}
	return 5
}

// emitTickMark draws a tick mark line. dx, dy are the offsets from
// (x, y) — for a bottom axis, dy>0 makes the tick extend downward.
// vertical=true draws a vertical line (x stays, y varies); false draws
// horizontal.
func emitTickMark(w *Writer, x, y, dx, dy float64, vertical bool) {
	w.OpenTag("line")
	w.Attr("class", "prism-axis-tick")
	if vertical {
		w.AttrFloat("x1", x)
		w.AttrFloat("y1", y)
		w.AttrFloat("x2", x)
		w.AttrFloat("y2", y+dy)
	} else {
		w.AttrFloat("x1", x)
		w.AttrFloat("y1", y)
		w.AttrFloat("x2", x+dx)
		w.AttrFloat("y2", y)
	}
	w.SelfClose()
}

// emitTickLabel emits a tick label, optionally rotated around its
// anchor point.
func emitTickLabel(w *Writer, label string, x, y float64, anchor string, angle float64) {
	w.OpenTag("text")
	w.Attr("class", "prism-axis-label")
	w.AttrFloat("x", x)
	w.AttrFloat("y", y)
	w.Attr("text-anchor", anchor)
	if angle != 0 {
		w.Attr("transform", rotateAround(angle, x, y))
	}
	w.CloseTagOpen()
	w.Text(label)
	w.EndTag("text")
}

// rotateAttr returns a CSS transform rotating -90 around (x, y).
func rotateAttr(x, y float64) string {
	return "rotate(-90 " + floatStr(x) + " " + floatStr(y) + ")"
}

// rotateAttr90 rotates +90 (used for right-axis titles).
func rotateAttr90(x, y float64) string {
	return "rotate(90 " + floatStr(x) + " " + floatStr(y) + ")"
}

// rotateAround formats a rotate transform around an arbitrary angle.
func rotateAround(angle, x, y float64) string {
	return "rotate(" + floatStr(angle) + " " + floatStr(x) + " " + floatStr(y) + ")"
}

// floatStr is render.FormatFloat aliased locally so axes.go does not
// have to import the render package directly.
func floatStr(v float64) string {
	return formatF(v)
}
