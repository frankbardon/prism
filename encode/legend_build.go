package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// LegendInputs carries the inputs the encoder collects to build one
// legend per non-trivial mark channel.
type LegendInputs struct {
	Channel    scene.Channel
	Title      string
	Categories []string        // for symbol legends
	Palette    []*scene.Color  // for symbol legends
	Position   scene.LegendPosition
	// Continuous gradient legend (optional, overrides Categories):
	Gradient *GradientLegend
}

// GradientLegend describes a continuous-color legend.
type GradientLegend struct {
	ID         string
	DomainMin  float64
	DomainMax  float64
	Stops      []scene.GradientStop
	LabelFormat string
}

// BuildSymbolLegend returns one Legend with N solid swatches, one
// per category. Returns nil when the channel is trivial (<=1
// category).
func BuildSymbolLegend(in LegendInputs, plot scene.Rect) *scene.Legend {
	if len(in.Categories) < 2 {
		return nil
	}
	pos := in.Position
	if pos == "" {
		pos = scene.LegendTopRight
	}
	entries := make([]scene.LegendEntry, len(in.Categories))
	for i, c := range in.Categories {
		var color *scene.Color
		if len(in.Palette) > 0 {
			color = in.Palette[i%len(in.Palette)]
		}
		entries[i] = scene.LegendEntry{
			Label: c,
			Swatch: scene.SwatchSpec{
				Type:  scene.SwatchSolid,
				Color: color,
			},
		}
	}
	return &scene.Legend{
		ID:       fmt.Sprintf("legend-%s", in.Channel),
		Channel:  in.Channel,
		Position: pos,
		Title:    in.Title,
		Entries:  entries,
		Frame:    placeLegendFrame(pos, len(entries), 16, plot),
	}
}

// BuildGradientLegend returns one Legend with a single gradient
// swatch referencing the supplied Gradient via scene.Defs.
func BuildGradientLegend(in LegendInputs, plot scene.Rect) *scene.Legend {
	if in.Gradient == nil {
		return nil
	}
	pos := in.Position
	if pos == "" {
		pos = scene.LegendRight
	}
	mnLabel := fmt.Sprintf("%g", in.Gradient.DomainMin)
	mxLabel := fmt.Sprintf("%g", in.Gradient.DomainMax)
	entries := []scene.LegendEntry{
		{
			Label: mnLabel + "–" + mxLabel,
			Swatch: scene.SwatchSpec{
				Type:       scene.SwatchGradient,
				GradientID: in.Gradient.ID,
			},
		},
	}
	return &scene.Legend{
		ID:       fmt.Sprintf("legend-%s", in.Channel),
		Channel:  in.Channel,
		Position: pos,
		Title:    in.Title,
		Entries:  entries,
		Frame:    placeLegendFrame(pos, 1, 130, plot),
	}
}

// placeLegendFrame returns a rough pixel rect for the legend. P06
// layout is intentionally simple — top-right anchors near the plot
// region's top-right corner; right anchors along the right side; etc.
// Vertical lists size by entry count × 18px row height.
func placeLegendFrame(pos scene.LegendPosition, entries int, rowH float64, plot scene.Rect) scene.Rect {
	const swatch = 12.0
	const pad = 4.0
	const labelMaxChars = 14
	width := swatch + pad + float64(labelMaxChars)*6 + pad
	height := float64(entries)*rowH + pad*2
	switch pos {
	case scene.LegendTopRight:
		return scene.Rect{X: plot.Right() - width, Y: plot.Y, W: width, H: height}
	case scene.LegendTopLeft:
		return scene.Rect{X: plot.X, Y: plot.Y, W: width, H: height}
	case scene.LegendBottomRight:
		return scene.Rect{X: plot.Right() - width, Y: plot.Bottom() - height, W: width, H: height}
	case scene.LegendBottomLeft:
		return scene.Rect{X: plot.X, Y: plot.Bottom() - height, W: width, H: height}
	case scene.LegendRight:
		return scene.Rect{X: plot.Right() + 10, Y: plot.Y, W: width, H: height}
	case scene.LegendLeft:
		return scene.Rect{X: plot.X - width - 10, Y: plot.Y, W: width, H: height}
	case scene.LegendTop:
		return scene.Rect{X: plot.X, Y: plot.Y - height - 4, W: width, H: height}
	case scene.LegendBottom:
		return scene.Rect{X: plot.X, Y: plot.Bottom() + 4, W: width, H: height}
	}
	return scene.Rect{X: plot.Right() - width, Y: plot.Y, W: width, H: height}
}
