package encode

import (
	"fmt"
	"math"

	"github.com/frankbardon/prism/encode/scene"
)

// LegendInputs carries the inputs the encoder collects to build one
// legend per non-trivial mark channel.
type LegendInputs struct {
	Channel    scene.Channel
	Title      string
	Categories []string       // for symbol legends
	Palette    []*scene.Color // for symbol legends
	Position   scene.LegendPosition
	// Style supplies the resolved token measurements. Zero value
	// falls back to DefaultLayoutStyle.
	Style LayoutStyle
	// Continuous gradient legend (optional, overrides Categories):
	Gradient *GradientLegend
}

// GradientLegend describes a continuous-color legend.
type GradientLegend struct {
	ID          string
	DomainMin   float64
	DomainMax   float64
	Stops       []scene.GradientStop
	LabelFormat string
}

// legendLabelMaxPx caps one legend label's width. Past this the label
// truncates and the full text travels as a <title>: a legend column
// wider than the chart it explains is not a legend.
const legendLabelMaxPx = 132

// gradientBarLength is the long dimension of a continuous legend's
// colour bar.
const gradientBarLength = 120

// ReserveSymbolLegend measures the space a categorical legend needs,
// before the plot rect exists. Returns nil when the channel is
// trivial (<= 1 category) — a legend that names one thing tells the
// reader nothing the title did not.
//
// Placement rule: a legend goes to the RIGHT while it is taller than
// it is disruptive, and moves UNDER the plot once the column would
// claim more than a third of the frame's width. A six-entry legend of
// long brand names beside a 700px chat-column chart would take half
// the drawing; the same six entries wrap into two rows underneath and
// cost ~40px of height.
func ReserveSymbolLegend(in LegendInputs, frameW, frameH float64) *LegendReserve {
	if len(in.Categories) < 2 {
		return nil
	}
	st := in.Style
	if st.LegendRowH == 0 {
		st = DefaultLayoutStyle()
	}
	labelM := TextMetrics{FontSize: st.LegendLabelSz}
	titleM := TextMetrics{FontSize: st.LegendTitleSz, Bold: true}

	maxLabel := 0.0
	for _, c := range in.Categories {
		w := math.Min(labelM.Width(c), legendLabelMaxPx)
		if w > maxLabel {
			maxLabel = w
		}
	}
	swatchCol := st.LegendSymbol + st.LegendSymbol*0.6 // swatch + gap
	colW := swatchCol + maxLabel
	if in.Title != "" {
		colW = math.Max(colW, math.Min(titleM.Width(in.Title), legendLabelMaxPx+swatchCol))
	}

	titleH := 0.0
	if in.Title != "" {
		titleH = titleM.Height() + st.LegendRowH*0.25
	}

	vertH := titleH + float64(len(in.Categories))*st.LegendRowH
	pos := in.Position
	if pos == "" {
		pos = scene.LegendRight
	}

	// Would the right-hand column eat too much of the frame?
	if pos == scene.LegendRight || pos == scene.LegendTopRight || pos == scene.LegendBottomRight {
		if colW+st.LegendGap > frameW/3 || vertH > frameH*0.85 {
			pos = scene.LegendBottom
		}
	}

	if pos == scene.LegendBottom {
		rows, w, h := horizontalLegendExtent(in.Categories, st, labelM, frameW, titleH)
		_ = rows
		return &LegendReserve{
			Position:   scene.LegendBottom,
			Direction:  scene.LegendHorizontal,
			Width:      w,
			Height:     h,
			Entries:    len(in.Categories),
			HasTitle:   in.Title != "",
			MaxLabelPx: maxLabel,
		}
	}
	return &LegendReserve{
		Position:   scene.LegendRight,
		Direction:  scene.LegendVertical,
		Width:      colW,
		Height:     vertH,
		Entries:    len(in.Categories),
		HasTitle:   in.Title != "",
		MaxLabelPx: maxLabel,
	}
}

// horizontalLegendExtent wraps entries across the available width and
// returns the row count plus the block's extent.
func horizontalLegendExtent(cats []string, st LayoutStyle, m TextMetrics, availW, titleH float64) (int, float64, float64) {
	gap := st.LegendSymbol * 1.8
	x := 0.0
	rows := 1
	widest := 0.0
	for _, c := range cats {
		w := st.LegendSymbol + st.LegendSymbol*0.6 + math.Min(m.Width(c), legendLabelMaxPx)
		if x > 0 && x+w > availW {
			rows++
			if x-gap > widest {
				widest = x - gap
			}
			x = 0
		}
		x += w + gap
	}
	if x-gap > widest {
		widest = x - gap
	}
	return rows, math.Min(widest, availW), titleH + float64(rows)*st.LegendRowH
}

// ReserveGradientLegend measures a continuous legend.
func ReserveGradientLegend(in LegendInputs, frameW, frameH float64) *LegendReserve {
	if in.Gradient == nil {
		return nil
	}
	st := in.Style
	if st.LegendRowH == 0 {
		st = DefaultLayoutStyle()
	}
	labelM := TextMetrics{FontSize: st.LegendLabelSz, Tabular: true}
	titleM := TextMetrics{FontSize: st.LegendTitleSz, Bold: true}

	f := AutoTickFormat([]float64{in.Gradient.DomainMin, in.Gradient.DomainMax})
	lw := math.Max(labelM.Width(f.Format(in.Gradient.DomainMin)), labelM.Width(f.Format(in.Gradient.DomainMax)))
	barW := st.LegendSymbol * 1.1
	w := barW + st.LegendSymbol*0.6 + lw
	if in.Title != "" {
		w = math.Max(w, titleM.Width(in.Title))
	}
	titleH := 0.0
	if in.Title != "" {
		titleH = titleM.Height() + st.LegendRowH*0.25
	}
	return &LegendReserve{
		Position:  scene.LegendRight,
		Direction: scene.LegendVertical,
		Width:     w,
		Height:    titleH + gradientBarLength,
		Entries:   1,
		HasTitle:  in.Title != "",
	}
}

// BuildSymbolLegend materialises the legend into the frame the layout
// reserved for it. Entry positions are resolved here rather than in
// the renderer so a horizontal legend's ragged wrapping is computed
// once, against the same metrics that sized the reservation.
func BuildSymbolLegend(in LegendInputs, res *LegendReserve, frame scene.Rect) *scene.Legend {
	if res == nil || len(in.Categories) < 2 {
		return nil
	}
	st := in.Style
	if st.LegendRowH == 0 {
		st = DefaultLayoutStyle()
	}
	labelM := TextMetrics{FontSize: st.LegendLabelSz}

	entries := make([]scene.LegendEntry, len(in.Categories))
	titleH := 0.0
	if in.Title != "" {
		titleH = TextMetrics{FontSize: st.LegendTitleSz, Bold: true}.Height() + st.LegendRowH*0.25
	}

	swatchCol := st.LegendSymbol + st.LegendSymbol*0.6
	gap := st.LegendSymbol * 1.8
	x, y := 0.0, titleH
	for i, c := range in.Categories {
		var color *scene.Color
		if len(in.Palette) > 0 {
			color = in.Palette[i%len(in.Palette)]
		}
		label, cut := labelM.Truncate(c, legendLabelMaxPx)
		full := ""
		if cut {
			full = c
		}
		e := scene.LegendEntry{
			Label:  label,
			Full:   full,
			Swatch: scene.SwatchSpec{Type: scene.SwatchSolid, Color: color},
		}
		if res.Direction == scene.LegendHorizontal {
			w := swatchCol + labelM.Width(label)
			if x > 0 && x+w > frame.W {
				x = 0
				y += st.LegendRowH
			}
			e.X, e.Y = x, y
			x += w + gap
		} else {
			e.X, e.Y = 0, y
			y += st.LegendRowH
		}
		entries[i] = e
	}

	pos := res.Position
	if pos == "" {
		pos = scene.LegendRight
	}
	return &scene.Legend{
		ID:         fmt.Sprintf("legend-%s", in.Channel),
		Channel:    in.Channel,
		Position:   pos,
		Direction:  res.Direction,
		Title:      in.Title,
		Entries:    entries,
		Frame:      frame,
		RowHeight:  st.LegendRowH,
		SymbolSize: st.LegendSymbol,
	}
}

// BuildGradientLegend returns one Legend with a single gradient
// swatch referencing the supplied Gradient via scene.Defs, labelled
// at both ends through the same auto formatter the axes use.
func BuildGradientLegend(in LegendInputs, res *LegendReserve, frame scene.Rect) *scene.Legend {
	if in.Gradient == nil || res == nil {
		return nil
	}
	st := in.Style
	if st.LegendRowH == 0 {
		st = DefaultLayoutStyle()
	}
	f := AutoTickFormat([]float64{in.Gradient.DomainMin, in.Gradient.DomainMax})
	entries := []scene.LegendEntry{
		{
			Label: f.Format(in.Gradient.DomainMax) + " " + f.Format(in.Gradient.DomainMin),
			Swatch: scene.SwatchSpec{
				Type:       scene.SwatchGradient,
				GradientID: in.Gradient.ID,
			},
		},
	}
	return &scene.Legend{
		ID:         fmt.Sprintf("legend-%s", in.Channel),
		Channel:    in.Channel,
		Position:   res.Position,
		Direction:  scene.LegendVertical,
		Title:      in.Title,
		Entries:    entries,
		Frame:      frame,
		RowHeight:  st.LegendRowH,
		SymbolSize: st.LegendSymbol,
	}
}
