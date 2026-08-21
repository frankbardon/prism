package encode

import (
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// Padding carries the per-side pixel padding around the plot region.
type Padding struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// Layout is the resolved frame + plot region for a single Scene.
// Frame = the outer SVG bounds; Plot = the inner rect marks render
// into. LegendFrame, when non-zero, is the rect reserved OUTSIDE
// Plot for the legend — reserved rather than overlaid, so a legend
// can never be drawn on top of the data it describes.
type Layout struct {
	Frame       scene.Rect
	Plot        scene.Rect
	Padding     Padding
	LegendFrame scene.Rect
	// XLabelAngle is the rotation the x tick labels need to fit
	// without colliding; 0 when they fit horizontally.
	XLabelAngle float64
	// XLabelMaxWidth is the width budget one x tick label may occupy
	// before it must be truncated.
	XLabelMaxWidth float64
	// YLabelMaxWidth is the same budget for the y tick column.
	YLabelMaxWidth float64
}

// LayoutStyle carries the theme-derived measurements the layout needs.
// Every field is a resolved --prism-* token value; nothing here is a
// literal chosen at the call site, so an organisation that raises its
// label font size gets a wider left margin without any other change.
type LayoutStyle struct {
	EdgePadding    float64 // --prism-padding-edge
	LabelFontSize  float64 // --prism-axis-label-font-size
	TitleFontSize  float64 // --prism-axis-title-font-size
	LabelPadding   float64 // --prism-axis-label-padding
	TitlePadding   float64 // --prism-axis-title-padding
	TickSize       float64 // --prism-axis-tick-size
	ChartTitleSize float64 // --prism-title-font-size
	TitleBlockPad  float64 // --prism-title-padding
	LegendGap      float64 // --prism-legend-gap
	LegendRowH     float64 // --prism-legend-row-height
	LegendSymbol   float64 // --prism-legend-symbol-extent
	LegendLabelSz  float64 // --prism-legend-label-font-size
	LegendTitleSz  float64 // --prism-legend-title-font-size
}

// DefaultLayoutStyle is the fallback when no theme resolved — the
// same numbers the light base declares, so a themeless render is not
// a different chart.
func DefaultLayoutStyle() LayoutStyle {
	return LayoutStyle{
		EdgePadding:    12,
		LabelFontSize:  11,
		TitleFontSize:  11,
		LabelPadding:   8,
		TitlePadding:   10,
		TickSize:       4,
		ChartTitleSize: 15,
		TitleBlockPad:  14,
		LegendGap:      16,
		LegendRowH:     18,
		LegendSymbol:   10,
		LegendLabelSz:  11,
		LegendTitleSz:  11,
	}
}

// LayoutInputs is everything Compute needs to place the plot rect.
type LayoutInputs struct {
	Width, Height float64
	Style         LayoutStyle

	Title    string
	Subtitle string

	// YLabels / XLabels are the tick labels a provisional pass
	// resolved. They exist only to be measured.
	YLabels []string
	XLabels []string
	YTitle  string
	XTitle  string
	// XCategorical marks a band / point / ordinal x axis, where the
	// label slot is the band step rather than the gap between two
	// numeric ticks.
	XCategorical bool
	// HasXAxis / HasYAxis suppress the whole reservation for a mark
	// that draws no axis (arc, sankey, geo).
	HasXAxis bool
	HasYAxis bool

	Legend *LegendReserve
}

// LegendReserve describes the space a legend needs, measured from
// its entry labels before the plot rect is fixed.
type LegendReserve struct {
	Position   scene.LegendPosition
	Direction  scene.LegendDirection
	Width      float64
	Height     float64
	Entries    int
	HasTitle   bool
	MaxLabelPx float64
}

// maxLabelFraction caps how much of the frame a single tick label
// column may claim.
//
// A 40-character category on the y axis would otherwise take the
// whole chart and leave a sliver for the bars, so past this fraction
// the label truncates instead. A third for the vertical column
// (labels sit beside the data and the reader scans them); a quarter
// of the HEIGHT for rotated x labels, which eat into the data area
// far more aggressively for the same character count.
const (
	maxYLabelFraction = 0.30
	maxXLabelFraction = 0.28
)

// titleGap is the space between the title block and the plot's top
// edge, expressed as a multiple of the title's own size so the
// relationship holds when an organisation resizes the title.
//
// A full line-height rather than a half: the topmost grid line is the
// plot's own top edge, so anything less puts the title's descenders
// on a rule.
const titleGap = 1.15

// Compute resolves the frame and plot rect from measured label
// extents.
//
// This is the pass that replaced a fixed 40/20/40/20 padding. Fixed
// padding is wrong in both directions at once: a y axis labelled
// "1,284,000" was clipped by it, and a y axis labelled "0/5/10" wasted
// three quarters of the reserved column. Everything below is measured
// from the labels that will actually be drawn, via TextMetrics.
func Compute(in LayoutInputs) Layout {
	st := in.Style
	if st.LabelFontSize == 0 {
		st = DefaultLayoutStyle()
	}
	edge := st.EdgePadding

	labelM := TextMetrics{FontSize: st.LabelFontSize, Tabular: true}
	titleM := TextMetrics{FontSize: st.TitleFontSize, Bold: true}

	top := edge
	if in.Title != "" {
		top += TextMetrics{FontSize: st.ChartTitleSize, Bold: true}.Height()
		if in.Subtitle != "" {
			top += TextMetrics{FontSize: st.ChartTitleSize * 0.82}.Height() * 1.1
		}
		top += st.ChartTitleSize * titleGap
	}

	left := edge
	yLabelBudget := in.Width * maxYLabelFraction
	yLabelW := 0.0
	if in.HasYAxis {
		yLabelW = math.Min(labelM.MaxWidth(in.YLabels), yLabelBudget)
		if len(in.YLabels) == 0 {
			// A caller that has no labels to hand over yet (a composite
			// cell, a mark that builds its own scales) still needs a
			// column reserved. Four digits is the median numeric label.
			yLabelW = labelM.Width("0000")
		}
		left += yLabelW + st.LabelPadding + st.TickSize
		if in.YTitle != "" {
			left += titleM.Height() + st.TitlePadding
		}
	}

	bottom := edge
	if in.HasXAxis {
		bottom += st.TickSize + st.LabelPadding
		if in.XTitle != "" {
			bottom += titleM.Height() + st.TitlePadding
		}
	}

	right := edge

	// Legend reservation. Right-side legends take width; bottom
	// legends take height. Either way the space leaves the plot rect
	// rather than being drawn over it.
	var legendFrame scene.Rect
	if in.Legend != nil {
		switch in.Legend.Position {
		case scene.LegendBottom:
			bottom += in.Legend.Height + st.LegendGap
		default:
			right += in.Legend.Width + st.LegendGap
		}
	}

	// X labels: decide rotation from the slot each label gets, then
	// fold the resulting band height into the bottom padding. Done
	// after the legend reservation because the plot width — and so
	// the slot width — depends on it.
	provisionalPlotW := in.Width - left - right
	xAngle, xLabelMaxW, xBandH := resolveXLabels(in, st, labelM, provisionalPlotW)
	if in.HasXAxis {
		bottom += xBandH
	}

	// Guard the degenerate frame: a chart smaller than its own chrome
	// still has to produce a valid rect rather than a negative one.
	plotW := in.Width - left - right
	plotH := in.Height - top - bottom
	if plotW < 1 {
		plotW = math.Max(1, in.Width*0.25)
		left = math.Max(0, (in.Width-plotW)/2)
		right = in.Width - left - plotW
	}
	if plotH < 1 {
		plotH = math.Max(1, in.Height*0.25)
		top = math.Max(0, (in.Height-plotH)/2)
		bottom = in.Height - top - plotH
	}

	plot := scene.Rect{X: left, Y: top, W: plotW, H: plotH}

	if in.Legend != nil {
		legendFrame = placeLegendFrame(*in.Legend, st, plot, in.Width, in.Height)
	}

	return Layout{
		Frame:          scene.Rect{X: 0, Y: 0, W: in.Width, H: in.Height},
		Plot:           plot,
		Padding:        Padding{Top: top, Right: right, Bottom: bottom, Left: left},
		LegendFrame:    legendFrame,
		XLabelAngle:    xAngle,
		XLabelMaxWidth: xLabelMaxW,
		YLabelMaxWidth: yLabelBudget,
	}
}

// resolveXLabels decides whether the x tick labels fit horizontally,
// and returns the angle, the per-label width budget, and the height
// the label band occupies.
//
// The escalation is intentional and in this order: keep them
// horizontal, then rotate, and only then truncate. Rotation before
// truncation because a rotated label still says the whole word, and
// the reader loses only the convenience of reading it level; a
// truncated one has lost information. Dropping every other label —
// what the previous code did — is last-resort and reserved for
// numeric axes, where the omitted values are interpolable and the
// category ones are not.
func resolveXLabels(in LayoutInputs, st LayoutStyle, m TextMetrics, plotW float64) (angle, maxW, bandH float64) {
	lineH := m.Height()
	if !in.HasXAxis || len(in.XLabels) == 0 {
		return 0, 0, lineH
	}
	n := len(in.XLabels)
	slot := plotW / math.Max(1, float64(n))
	// Labels need a gutter between them or they read as one string.
	const gutter = 8.0
	widest := m.MaxWidth(in.XLabels)

	if widest <= slot-gutter {
		return 0, slot - gutter, lineH
	}

	// Rotated at -45°, a label's horizontal footprint is its height
	// over sin(45°) — the diagonal only needs to clear its neighbour
	// by one line box — so rotation buys room whenever the slot can
	// hold ~1.4 line heights.
	const rot = -45.0
	rad := math.Pi / 4
	if slot >= lineH/math.Sin(rad) {
		// Height consumed is the label's own width projected onto the
		// vertical, capped so a long category cannot push the plot to
		// nothing.
		budget := in.Height * maxXLabelFraction
		proj := widest * math.Sin(rad)
		if proj > budget {
			proj = budget
			// Re-derive the width that fits the capped band.
			widest = budget / math.Sin(rad)
		}
		return rot, widest, proj + lineH*0.5
	}

	// Too tight even rotated: keep them level and truncate to the slot.
	return 0, math.Max(slot-gutter, m.Width("W")), lineH
}

// placeLegendFrame positions the reserved legend rect. Right-side
// legends centre vertically against the plot — a legend pinned to the
// top of a tall chart reads as a caption that fell off something.
func placeLegendFrame(lg LegendReserve, st LayoutStyle, plot scene.Rect, frameW, frameH float64) scene.Rect {
	switch lg.Position {
	case scene.LegendBottom:
		return scene.Rect{
			X: plot.X,
			Y: plot.Bottom() + (frameH - plot.Bottom()) - lg.Height - st.EdgePadding,
			W: math.Max(lg.Width, plot.W),
			H: lg.Height,
		}
	default:
		y := plot.Y
		if lg.Height < plot.H {
			y = plot.Y + (plot.H-lg.Height)/2
		}
		return scene.Rect{
			X: plot.Right() + st.LegendGap,
			Y: y,
			W: lg.Width,
			H: lg.Height,
		}
	}
}

// ComputeSparkline returns a Layout for a sparkline plot: 4-px
// padding all sides, no axis/legend/title reservation. See D067.
func ComputeSparkline(width, height float64) Layout {
	pad := Padding{Top: 4, Right: 4, Bottom: 4, Left: 4}
	frame := scene.Rect{X: 0, Y: 0, W: width, H: height}
	plot := scene.Rect{
		X: pad.Left,
		Y: pad.Top,
		W: width - pad.Left - pad.Right,
		H: height - pad.Top - pad.Bottom,
	}
	return Layout{Frame: frame, Plot: plot, Padding: pad}
}

// ComputeProvisional returns the first-pass layout used to resolve
// scales before any label exists to measure. It reserves a generous
// fixed margin; the real Compute pass then shrinks or grows it to the
// labels that came out. Two passes rather than a fixed point: the
// only feedback from pass two into pass one is the tick COUNT, which
// changes labels only when the plot height crosses a ~60px band, and
// a third pass has never moved a fixture.
func ComputeProvisional(width, height float64, hasTitle bool) Layout {
	top := 16.0
	if hasTitle {
		top = 46.0
	}
	pad := Padding{Top: top, Right: 20, Bottom: 48, Left: 52}
	return Layout{
		Frame:   scene.Rect{X: 0, Y: 0, W: width, H: height},
		Plot:    scene.Rect{X: pad.Left, Y: pad.Top, W: width - pad.Left - pad.Right, H: height - pad.Top - pad.Bottom},
		Padding: pad,
	}
}
