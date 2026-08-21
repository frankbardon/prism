package svg

import "github.com/frankbardon/prism/encode/scene"

// renderAxis emits the axis grid, domain line, tick marks + labels,
// and title for one resolved scene.Axis.
//
// Geometry it does NOT decide any more: the perpendicular distance to
// the label and to the title now arrive on the Axis as LabelOffset and
// TitleOffset, resolved at encode time from theme tokens against the
// measured label extents. They used to be the literals 18 and 34 here,
// which is why a wide numeric column pushed its labels under the axis
// title and why raising the label font size in a theme moved nothing.
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

	// Zero baseline, above the grid and below the marks.
	if a.ZeroLine != nil {
		w.OpenTag("line")
		w.Attr("class", "prism-zero-line")
		w.AttrFloat("x1", a.ZeroLine.X1)
		w.AttrFloat("y1", a.ZeroLine.Y1)
		w.AttrFloat("x2", a.ZeroLine.X2)
		w.AttrFloat("y2", a.ZeroLine.Y2)
		w.SelfClose()
	}

	// Domain line, unless a grid line already covers those pixels.
	if !a.HideDomain {
		w.OpenTag("line")
		w.Attr("class", "prism-axis-domain")
		w.AttrFloat("x1", a.Domain.X1)
		w.AttrFloat("y1", a.Domain.Y1)
		w.AttrFloat("x2", a.Domain.X2)
		w.AttrFloat("y2", a.Domain.Y2)
		w.SelfClose()
	}

	labelOff := a.LabelOffset
	if labelOff == 0 {
		labelOff = 12
	}

	// Ticks + labels. A hidden tick keeps its label and loses only the
	// stroke.
	switch a.Position {
	case scene.AxisPositionBottom:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, t.Pixel, plot.Bottom(), 0, tickLen(t), true)
			}
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t, t.Pixel, plot.Bottom()+labelOff, anchorForAngle(a.LabelAngle, "middle"), a.LabelAngle)
			}
		}
	case scene.AxisPositionTop:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, t.Pixel, plot.Y, 0, -tickLen(t), true)
			}
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t, t.Pixel, plot.Y-labelOff, anchorForAngle(a.LabelAngle, "middle"), a.LabelAngle)
			}
		}
	case scene.AxisPositionLeft:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, plot.X, t.Pixel, -tickLen(t), 0, false)
			}
			if t.Label != "" && !t.LabelHidden {
				// +0.32em lifts the baseline to the label's optical centre
				// against the grid line it annotates.
				emitTickLabel(w, t, plot.X-labelOff, t.Pixel+labelBaselineShift, "end", a.LabelAngle)
			}
		}
	case scene.AxisPositionRight:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, plot.Right(), t.Pixel, tickLen(t), 0, false)
			}
			if t.Label != "" && !t.LabelHidden {
				emitTickLabel(w, t, plot.Right()+labelOff, t.Pixel+labelBaselineShift, "start", a.LabelAngle)
			}
		}
	}

	// Title (one per axis).
	if a.Title != "" {
		titleOff := a.TitleOffset
		if titleOff == 0 {
			titleOff = 30
		}
		w.OpenTag("text")
		w.Attr("class", "prism-axis-title")
		switch a.Position {
		case scene.AxisPositionBottom:
			w.AttrFloat("x", plot.CenterX())
			w.AttrFloat("y", plot.Bottom()+titleOff)
			w.Attr("text-anchor", "middle")
		case scene.AxisPositionTop:
			w.AttrFloat("x", plot.CenterX())
			w.AttrFloat("y", plot.Y-titleOff)
			w.Attr("text-anchor", "middle")
		case scene.AxisPositionLeft:
			w.AttrFloat("x", plot.X-titleOff)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr(plot.X-titleOff, plot.CenterY()))
		case scene.AxisPositionRight:
			w.AttrFloat("x", plot.Right()+titleOff)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr90(plot.Right()+titleOff, plot.CenterY()))
		}
		w.CloseTagOpen()
		w.Text(a.Title)
		w.EndTag("text")
	}

	w.EndTag("g")
}

// labelBaselineShift centres a tick label vertically on the pixel it
// annotates. An SVG text baseline sits at the bottom of the glyphs, so
// a label placed at the tick's exact y reads as sitting above the grid
// line rather than on it.
const labelBaselineShift = 3.6

// anchorForAngle returns the text-anchor for a rotated label. A label
// rotated -45° must anchor at its END, so it hangs back-and-left from
// the tick; anchoring it in the middle swings half the string under
// the neighbouring category.
func anchorForAngle(angle float64, dflt string) string {
	switch {
	case angle < 0:
		return "end"
	case angle > 0:
		return "start"
	}
	return dflt
}

// tickLen returns the pixel length of the tick mark. Ticks whose axis
// was flagged HideTicks never reach here — the encoder dropped them.
func tickLen(t scene.Tick) float64 {
	if t.Minor {
		return 2.5
	}
	return 4
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
// anchor point. A truncated label carries its full text as a <title>
// child, so shortening a category to fit never destroys it — the
// reader can still recover the value on hover, and a screen reader
// announces the whole thing.
func emitTickLabel(w *Writer, t scene.Tick, x, y float64, anchor string, angle float64) {
	w.OpenTag("text")
	w.Attr("class", "prism-axis-label")
	w.AttrFloat("x", x)
	w.AttrFloat("y", y)
	w.Attr("text-anchor", anchor)
	if angle != 0 {
		w.Attr("transform", rotateAround(angle, x, y))
	}
	w.CloseTagOpen()
	w.Text(t.Label)
	if t.Full != "" {
		w.OpenTag("title")
		w.CloseTagOpen()
		w.Text(t.Full)
		w.EndTag("title")
	}
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
