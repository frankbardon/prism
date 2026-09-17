package svg

import "github.com/frankbardon/prism/encode/scene"

// renderAxis emits the axis domain line, tick lines + labels, grid
// lines, and (optional) title for one resolved scene.Axis. Geometry
// is dispatched per AxisPosition: bottom/top render horizontally,
// left/right render vertically. Minor ticks render shorter (3px) and
// without labels; LabelHidden tick labels are skipped (the tick mark
// itself stays).
//
// theme carries the resolved scene.Theme (may be nil); its
// AxisLabelLineHeight/AxisLabelLetterSpacing and
// AxisTitleLineHeight/AxisTitleLetterSpacing tokens (E2-S2) apply to
// tick labels and the axis title respectively.
func renderAxis(w *Writer, a scene.Axis, plot scene.Rect, theme *scene.Theme) {
	// The theme's per-axis override block for this channel, if any
	// (E8-S1). Its colour/stroke half never reaches here — theme/css.go
	// scopes those onto this very group's class and CSS inheritance
	// applies them. What is left is geometry, SVG-attribute typography,
	// and the group filter.
	perAxis := theme.AxisTokensFor(a.Channel)

	w.OpenTag("g")
	w.Attr("class", "prism-axis prism-axis-"+string(a.Channel))
	w.Attr("data-prism-axis-id", a.ID)
	if perAxis != nil {
		// Composes with the shared block's filter on the enclosing
		// prism-axes group rather than replacing it.
		writeFilterAttr(w, perAxis.Filter)
	}
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

	// Domain line — suppressed by `axis.domain: false` (E3-S2). The
	// grid lines above and the tick marks below are independent of it.
	if !a.HideDomain {
		w.OpenTag("line")
		w.Attr("class", "prism-axis-domain")
		w.AttrFloat("x1", a.Domain.X1)
		w.AttrFloat("y1", a.Domain.Y1)
		w.AttrFloat("x2", a.Domain.X2)
		w.AttrFloat("y2", a.Domain.Y2)
		w.SelfClose()
	}

	// Resolved typography tokens (E2-S2) — nil-safe extraction once,
	// reused across every tick label / the axis title below.
	// Per-axis overrides fall through to the shared block property by
	// property (E8-S1).
	var labelLH, labelLS, titleLH, titleLS *float64
	if theme != nil {
		labelLH, labelLS = theme.AxisLabelLineHeight, theme.AxisLabelLetterSpacing
		titleLH, titleLS = theme.AxisTitleLineHeight, theme.AxisTitleLetterSpacing
	}
	if perAxis != nil {
		labelLH = firstFloat(perAxis.LabelLineHeight, labelLH)
		labelLS = firstFloat(perAxis.LabelLetterSpacing, labelLS)
		titleLH = firstFloat(perAxis.TitleLineHeight, titleLH)
		titleLS = firstFloat(perAxis.TitleLetterSpacing, titleLS)
	}

	// Tick length and label gap (E3-S2). Precedence, highest first:
	// the axis's own spec value, then the theme's per-axis `axis_x` /
	// `axis_y` token, then the shared `axis` token
	// (--prism-axis-tick-size / --prism-axis-label-padding), then the
	// built-in metric. With all of them unset these reproduce the
	// historical 5 px tick and 18/8/8/8 px label offsets exactly.
	size := resolveAxisMetric(a.TickSize, themeTickSize(theme, a.Channel), defaultTickSize)
	pad := resolveAxisMetric(a.LabelPadding, themeLabelPadding(theme, a.Channel), defaultLabelPadding)

	// Ticks + labels. `axis.ticks: false` drops the marks, `axis.labels:
	// false` drops the text; the two are independent of each other and
	// of the domain line above.
	switch a.Position {
	case scene.AxisPositionBottom:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, t.Pixel, plot.Bottom(), 0, tickLen(t, size), true)
			}
			if t.Label != "" && !t.LabelHidden && !a.HideLabels {
				emitTickLabel(w, t.Label, t.Pixel, plot.Bottom()+baselineDropBottom+pad, "middle", a.LabelAngle, labelLH, labelLS)
			}
		}
	case scene.AxisPositionTop:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, t.Pixel, plot.Y, 0, -tickLen(t, size), true)
			}
			if t.Label != "" && !t.LabelHidden && !a.HideLabels {
				emitTickLabel(w, t.Label, t.Pixel, plot.Y-baselineDropOther-pad, "middle", a.LabelAngle, labelLH, labelLS)
			}
		}
	case scene.AxisPositionLeft:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, plot.X, t.Pixel, -tickLen(t, size), 0, false)
			}
			if t.Label != "" && !t.LabelHidden && !a.HideLabels {
				emitTickLabel(w, t.Label, plot.X-baselineDropOther-pad, t.Pixel+4, "end", a.LabelAngle, labelLH, labelLS)
			}
		}
	case scene.AxisPositionRight:
		for _, t := range a.Ticks {
			if !a.HideTicks {
				emitTickMark(w, plot.Right(), t.Pixel, tickLen(t, size), 0, false)
			}
			if t.Label != "" && !t.LabelHidden && !a.HideLabels {
				emitTickLabel(w, t.Label, plot.Right()+baselineDropOther+pad, t.Pixel+4, "start", a.LabelAngle, labelLH, labelLS)
			}
		}
	}

	// Title (one per axis). Its distance from the plot edge follows
	// the same precedence chain as the tick metrics above (E8-S2):
	// `axis.title_padding` > theme `axis_x`/`axis_y` > theme `axis`
	// (--prism-axis-title-padding) > the built-in 8 px. The per-side
	// titleDrop* constants are the fixed text allowance the padding is
	// added to, chosen so the built-in padding reproduces the
	// historical 34 / 28 / 30 px offsets exactly.
	if a.Title != "" {
		titlePad := resolveAxisMetric(a.TitlePadding, themeTitlePadding(theme, a.Channel), defaultTitlePadding)
		w.OpenTag("text")
		w.Attr("class", "prism-axis-title")
		switch a.Position {
		case scene.AxisPositionBottom:
			w.AttrFloat("x", plot.CenterX())
			w.AttrFloat("y", plot.Bottom()+titleDropBottom+titlePad)
			w.Attr("text-anchor", "middle")
		case scene.AxisPositionTop:
			w.AttrFloat("x", plot.CenterX())
			w.AttrFloat("y", plot.Y-titleDropTop-titlePad)
			w.Attr("text-anchor", "middle")
		case scene.AxisPositionLeft:
			x := plot.X - titleDropSide - titlePad
			w.AttrFloat("x", x)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr(x, plot.CenterY()))
		case scene.AxisPositionRight:
			x := plot.Right() + titleDropSide + titlePad
			w.AttrFloat("x", x)
			w.AttrFloat("y", plot.CenterY())
			w.Attr("text-anchor", "middle")
			w.Attr("transform", rotateAttr90(x, plot.CenterY()))
		}
		writeTypographyAttrs(w, titleLH, titleLS)
		w.CloseTagOpen()
		w.Text(a.Title)
		w.EndTag("text")
	}

	w.EndTag("g")
}

// Built-in axis metrics — the values that apply when neither the
// axis's spec block nor the theme states one.
const (
	// defaultTickSize is a major tick mark's length in pixels.
	defaultTickSize = 5.0
	// minorTickRatio shortens a minor tick relative to a major one.
	// 0.6 of the 5 px default is the historical 3 px.
	minorTickRatio = 0.6
	// defaultLabelPadding is the gap between the axis line and its
	// tick labels, matching theme/css.go's
	// --prism-axis-label-padding:4px.
	defaultLabelPadding = 4.0
	// baselineDropBottom / baselineDropOther are the fixed text
	// allowances a tick label needs on top of the padding: a label
	// under a bottom axis is anchored at its baseline and so sits a
	// full line lower, while the other three sides only clear the
	// tick. defaultLabelPadding added to each reproduces the
	// historical 18 / 8 px offsets.
	baselineDropBottom = 14.0
	baselineDropOther  = 4.0
	// defaultTitlePadding is the gap between an axis's tick labels and
	// its title, matching theme/css.go's --prism-axis-title-padding
	// and the 8 px every built-in theme states.
	defaultTitlePadding = 8.0
	// titleDropBottom / titleDropTop / titleDropSide are the fixed
	// text allowances the axis title needs on top of the padding, one
	// per side. defaultTitlePadding added to each reproduces the
	// historical hard-coded 34 / 28 / 30 px offsets exactly, which is
	// what keeps every committed golden byte-identical for a theme
	// stating the default 8.
	titleDropBottom = 26.0
	titleDropTop    = 20.0
	titleDropSide   = 22.0
)

// resolveAxisMetric applies the axis-metric precedence chain: the
// spec-level value wins, the theme token fills in where the spec is
// silent, and the built-in metric is the floor. This is the single
// place the rule is expressed, so `tick_size`, `label_padding` and
// `title_padding` cannot drift apart.
//
// themeVal has already had the per-axis `axis_x` / `axis_y` block
// folded over the shared `axis` one by themeTickSize /
// themeLabelPadding / themeTitlePadding (E8-S1, E8-S2), so the full
// chain this participates in is
// spec > theme.axis_x|axis_y > theme.axis > built-in.
func resolveAxisMetric(specVal, themeVal *float64, builtin float64) float64 {
	if specVal != nil {
		return *specVal
	}
	if themeVal != nil {
		return *themeVal
	}
	return builtin
}

// themeTickSize / themeLabelPadding / themeTitlePadding read the
// resolved axis geometry tokens off a possibly-nil scene.Theme, for
// one channel. The
// channel's own `axis_x` / `axis_y` block wins over the shared `axis`
// one (E8-S1); a nil there falls through to the shared value, which is
// what makes the theme layer merge per property rather than
// wholesale. The result is still only the *theme* arm — the spec's own
// value outranks it in resolveAxisMetric.
func themeTickSize(t *scene.Theme, ch scene.Channel) *float64 {
	if t == nil {
		return nil
	}
	if per := t.AxisTokensFor(ch); per != nil && per.TickSize != nil {
		return per.TickSize
	}
	return t.AxisTickSize
}

func themeLabelPadding(t *scene.Theme, ch scene.Channel) *float64 {
	if t == nil {
		return nil
	}
	if per := t.AxisTokensFor(ch); per != nil && per.LabelPadding != nil {
		return per.LabelPadding
	}
	return t.AxisLabelPadding
}

// themeTitlePadding is the same per-axis-then-shared fall-through for
// the axis title's gap (E8-S2).
func themeTitlePadding(t *scene.Theme, ch scene.Channel) *float64 {
	if t == nil {
		return nil
	}
	if per := t.AxisTokensFor(ch); per != nil && per.TitlePadding != nil {
		return per.TitlePadding
	}
	return t.AxisTitlePadding
}

// firstFloat is the same per-property fall-through applied to the
// typography tokens: the per-axis value when stated, the shared one
// otherwise.
func firstFloat(per, shared *float64) *float64 {
	if per != nil {
		return per
	}
	return shared
}

// tickLen returns the pixel length of the tick mark, given the
// resolved major tick size. Minor ticks are shorter than majors.
func tickLen(t scene.Tick, size float64) float64 {
	if t.Minor {
		return size * minorTickRatio
	}
	return size
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
// anchor point. lineHeight/letterSpacing carry the resolved
// AxisLabelLineHeight/AxisLabelLetterSpacing tokens (E2-S2, may be
// nil).
func emitTickLabel(w *Writer, label string, x, y float64, anchor string, angle float64, lineHeight, letterSpacing *float64) {
	w.OpenTag("text")
	w.Attr("class", "prism-axis-label")
	w.AttrFloat("x", x)
	w.AttrFloat("y", y)
	w.Attr("text-anchor", anchor)
	if angle != 0 {
		w.Attr("transform", rotateAround(angle, x, y))
	}
	writeTypographyAttrs(w, lineHeight, letterSpacing)
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
