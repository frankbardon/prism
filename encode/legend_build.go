package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Legend box metrics. These are fixed pixels, not measured text —
// Prism has no text-measurement pass (see the note on the layout
// constants). legendInset mirrors the 4-px content inset the SVG
// renderer draws swatches and labels at, so the frame the encoder
// emits and the geometry the renderer draws stay in step.
const (
	legendSwatchSize    = 12.0
	legendInset         = 4.0
	legendLabelMaxChars = 14.0
	legendLabelCharW    = 6.0
	// legendSymbolRowH is the per-entry height the symbol legend's
	// frame is sized with. It is intentionally smaller than the pitch
	// the renderer draws rows at — the frame is a historical
	// approximation the goldens pin, so only the side reservation
	// (which must hold what is actually drawn) uses legendRowPitch.
	legendSymbolRowH = 16.0
	legendGradientH  = 130.0
	// legendGradientTickCount is the number of labelled stops a
	// gradient legend draws when legend.tick_count is unset. It
	// matches the axis builder's default tick count so a gradient
	// legend reads at the same density as the axis beside it.
	legendGradientTickCount = 5
	// legendRowPitch is the vertical pitch render/svg/legends.go draws
	// legend rows at, and legendTitleBand the extra height a legend
	// title claims above the first row (14-px baseline + 4-px gap).
	legendRowPitch  = 18.0
	legendTitleBand = 18.0
	// legendSideGap / legendBandGap are the default offsets between
	// the plot's chrome and a side-placed legend. They reproduce the
	// gaps the pre-E1-S3 placement hard-coded.
	legendSideGap = 10.0
	legendBandGap = 4.0
)

// LegendOrientNone is the legend.orient value that suppresses the
// legend entirely.
const LegendOrientNone = "none"

// LegendInputs carries the inputs the encoder collects to build one
// legend per non-trivial mark channel.
type LegendInputs struct {
	Channel    scene.Channel
	Title      string
	Categories []string       // for symbol legends
	Palette    []*scene.Color // for symbol legends
	Placement  LegendPlacement
	// Content is the resolved content half of the channel's legend
	// block (E3-S3): title override, entry filter, label format and
	// label limit. The zero value reproduces the defaults.
	Content LegendContent
	// Continuous gradient legend (optional, overrides Categories):
	Gradient *GradientLegend
}

// LegendPlacement is the resolved placement of one legend: where it
// anchors, how much interior padding its box carries, the gap it
// keeps from the plot's chrome, and — for side placements only — the
// depth of the margin band the layout set aside for it.
//
// The distinction that drives everything here: **side placements
// (left / right / top / bottom) consume margin, corner placements
// (the four `*-left` / `*-right` anchors) overlay the plot.** Only a
// side placement carries a non-zero Reserve.
type LegendPlacement struct {
	Position scene.LegendPosition
	// Padding is legend.padding: interior breathing room added on
	// every side of the legend box, growing the frame by 2×Padding in
	// each dimension and pushing the drawn content in by Padding.
	Padding float64
	// Offset is legend.offset: the gap between the legend frame and
	// the plot's chrome for a side placement, or the inward inset
	// from the plot corner for a corner placement.
	Offset float64
	// Reserve is the depth of the reserved margin band on this
	// legend's side, measured from the plot edge outward (axis
	// reservation included). Left / top placements anchor their far
	// edge at plot edge − Reserve, which is what keeps them clear of
	// the axis. Zero falls back to anchoring off the plot edge — the
	// pre-E1-S3 behaviour, which overlays whatever sits in the margin.
	Reserve float64
}

// IsSideLegend reports whether pos is one of the four side
// placements, which reserve margin, rather than one of the four
// corner placements, which overlay the plot.
func IsSideLegend(pos scene.LegendPosition) bool {
	switch pos {
	case scene.LegendLeft, scene.LegendRight, scene.LegendTop, scene.LegendBottom:
		return true
	}
	return false
}

// LegendPositionFromOrient maps a spec legend.orient value onto the
// scene placement. All eight scene.LegendPosition values are
// reachable; ok is false for an unrecognised orient (including
// "none", which the caller handles as suppression).
func LegendPositionFromOrient(orient string) (scene.LegendPosition, bool) {
	switch scene.LegendPosition(orient) {
	case scene.LegendLeft, scene.LegendRight, scene.LegendTop, scene.LegendBottom,
		scene.LegendTopLeft, scene.LegendTopRight,
		scene.LegendBottomLeft, scene.LegendBottomRight:
		return scene.LegendPosition(orient), true
	}
	return "", false
}

// DefaultLegendOffset is the gap a placement keeps from the plot when
// legend.offset is unset. The corner placements sit flush against the
// plot corner, as they always have.
func DefaultLegendOffset(pos scene.LegendPosition) float64 {
	switch pos {
	case scene.LegendLeft, scene.LegendRight:
		return legendSideGap
	case scene.LegendTop, scene.LegendBottom:
		return legendBandGap
	}
	return 0
}

// LegendBox is the measured geometry of one legend box: how many
// entries it draws, the per-entry row height, the interior padding,
// the label character budget it is sized against and whether a title
// sits above the first row.
//
// It is the single place a legend's pixel extent is computed — the
// frame the encoder emits (placeLegendFrame) and the margin band the
// layout reserves (SideExtent) both go through it, which is what
// keeps the two in step. E3-S4's `direction` is a field here: laying
// entries out in a row instead of a column changes Size, and the
// frame plus the reservation follow automatically.
type LegendBox struct {
	Entries int
	RowH    float64
	Padding float64
	// MaxChars is the label character budget. Zero falls back to the
	// default 14-character budget.
	MaxChars float64
	HasTitle bool
}

// maxChars resolves the character budget, defaulting when unset.
func (b LegendBox) maxChars() float64 {
	if b.MaxChars > 0 {
		return b.MaxChars
	}
	return legendLabelMaxChars
}

// Size returns the frame size of the box.
func (b LegendBox) Size() (w, h float64) {
	w = legendSwatchSize + legendInset + b.maxChars()*legendLabelCharW + legendInset + 2*b.Padding
	h = float64(b.Entries)*b.RowH + legendInset*2 + 2*b.Padding
	return w, h
}

// SideExtent returns the pixels a side-placed legend claims beyond
// the plot's chrome: its drawn size across the side plus the offset
// gap. Corner placements overlay the plot and claim nothing, so they
// return 0.
//
// Left / right measure the box width; top / bottom measure what the
// renderer actually draws (legendRowPitch per entry plus the title
// band), which is taller than the frame the goldens pin — the
// reservation has to hold the drawing, not the approximation.
func (b LegendBox) SideExtent(pos scene.LegendPosition, offset float64) float64 {
	switch pos {
	case scene.LegendLeft, scene.LegendRight:
		w, _ := b.Size()
		return w + offset
	case scene.LegendTop, scene.LegendBottom:
		h := float64(b.Entries)*legendRowPitch + legendInset*2 + 2*b.Padding
		if b.HasTitle {
			h += legendTitleBand
		}
		return h + offset
	}
	return 0
}

// ResolveLegendPlacement turns the spec legend block on a mark
// channel into a resolved placement. def is the placement used when
// the block is absent or carries no orient. The second return is
// false when the channel suppresses its legend (orient "none"), in
// which case no legend is built and no margin is reserved.
//
// An unrecognised orient falls back to def — the JSON Schema
// (schema/v1/legend.schema.json) already constrains the enum, so a
// bad value never reaches a validated spec.
func ResolveLegendPlacement(lg *spec.Legend, def scene.LegendPosition) (LegendPlacement, bool) {
	pl := LegendPlacement{Position: def}
	if lg != nil && lg.Orient != "" {
		if lg.Orient == LegendOrientNone {
			return pl, false
		}
		if pos, ok := LegendPositionFromOrient(lg.Orient); ok {
			pl.Position = pos
		}
	}
	pl.Offset = DefaultLegendOffset(pl.Position)
	if lg != nil {
		if lg.Offset != nil {
			pl.Offset = *lg.Offset
		}
		if lg.Padding != nil && *lg.Padding > 0 {
			pl.Padding = *lg.Padding
		}
	}
	return pl, true
}

// legendSpecOf returns the legend block the encoder builds a legend
// from. Only the color channel builds one today.
func legendSpecOf(enc *spec.Encoding) *spec.Legend {
	if enc == nil || enc.Color == nil {
		return nil
	}
	return enc.Color.Legend
}

// distinctStringValues returns the first-appearance-ordered distinct
// string values of a column, skipping non-string cells. It is the
// shared basis for categorical palettes and symbol-legend entries.
func distinctStringValues(col table.Column) []string {
	out := []string{}
	seen := map[string]bool{}
	for i := 0; i < col.Len(); i++ {
		v, ok := col.ValueAt(i).(string)
		if !ok || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// legendEntryCount reports how many symbol-legend entries the color
// channel of enc would produce over tbl. The layout needs the count
// before the color channel is resolved, because a top/bottom legend's
// reservation grows with its entry count.
func legendEntryCount(enc *spec.Encoding, tbl *table.Table) int {
	if enc == nil || enc.Color == nil || enc.Color.Field == "" || tbl == nil {
		return 0
	}
	col, ok := tbl.Column(enc.Color.Field)
	if !ok {
		return 0
	}
	cats := distinctStringValues(col)
	// Fewer than two categories builds no legend at all, so nothing
	// is reserved. Above that the count is the *shown* entry count:
	// legend.values filters entries, and the reserved band has to
	// match what BuildSymbolLegend will actually draw.
	if len(cats) < 2 {
		return 0
	}
	return len(ResolveLegendContent(legendSpecOf(enc)).SelectCategories(cats))
}

// GradientLegend describes a continuous-color legend.
type GradientLegend struct {
	ID          string
	DomainMin   float64
	DomainMax   float64
	Stops       []scene.GradientStop
	LabelFormat string
}

// BuildSymbolLegend returns one Legend with a solid swatch per shown
// category. Returns nil when the channel is trivial (<=1 category),
// or when legend.values filters every entry away.
//
// legend.values selects and reorders the entries; each surviving
// entry keeps the palette slot of its *original* category index, so a
// filtered legend's swatches still match the marks. legend.format
// then renders each label and legend.label_limit truncates it.
func BuildSymbolLegend(in LegendInputs, plot scene.Rect) *scene.Legend {
	if len(in.Categories) < 2 {
		return nil
	}
	shown := in.Content.SelectCategories(in.Categories)
	if len(shown) == 0 {
		return nil
	}
	pl := in.Placement
	if pl.Position == "" {
		pl.Position = scene.LegendTopRight
	}
	entries := make([]scene.LegendEntry, len(shown))
	for i, idx := range shown {
		var color *scene.Color
		if len(in.Palette) > 0 {
			color = in.Palette[idx%len(in.Palette)]
		}
		entries[i] = scene.LegendEntry{
			Label: in.Content.Truncate(in.Content.Label(in.Categories[idx])),
			Swatch: scene.SwatchSpec{
				Type:  scene.SwatchSolid,
				Color: color,
			},
		}
	}
	title := legendTitle(in.Title, in.Content)
	box := LegendBox{
		Entries:  len(entries),
		RowH:     legendSymbolRowH,
		Padding:  pl.Padding,
		MaxChars: in.Content.MaxChars(),
		HasTitle: title != "",
	}
	return &scene.Legend{
		ID:       fmt.Sprintf("legend-%s", in.Channel),
		Channel:  in.Channel,
		Position: pl.Position,
		Title:    title,
		Entries:  entries,
		Padding:  pl.Padding,
		Frame:    placeLegendFrame(pl, box, plot),
	}
}

// BuildGradientLegend returns one Legend with a single gradient
// swatch referencing the supplied Gradient via scene.Defs, carrying
// legend.tick_count labelled stops along the bar.
//
// The entry's Label stays the "min–max" summary the legend has always
// carried, so an IR consumer that ignores Ticks still reads something
// sensible; a renderer that understands Ticks draws those instead.
func BuildGradientLegend(in LegendInputs, plot scene.Rect) *scene.Legend {
	if in.Gradient == nil {
		return nil
	}
	pl := in.Placement
	if pl.Position == "" {
		pl.Position = scene.LegendRight
	}
	content := in.Content
	if content.Format == nil && in.Gradient.LabelFormat != "" {
		if sp, err := format.Parse(in.Gradient.LabelFormat); err == nil {
			content.Format = sp
		}
	}
	mnLabel := content.Truncate(content.Label(in.Gradient.DomainMin))
	mxLabel := content.Truncate(content.Label(in.Gradient.DomainMax))
	entries := []scene.LegendEntry{
		{
			Label: mnLabel + "–" + mxLabel,
			Swatch: scene.SwatchSpec{
				Type:       scene.SwatchGradient,
				GradientID: in.Gradient.ID,
			},
			Ticks: gradientLegendTicks(in.Gradient, content),
		},
	}
	title := legendTitle(in.Title, in.Content)
	box := LegendBox{
		Entries:  1,
		RowH:     legendGradientH,
		Padding:  pl.Padding,
		MaxChars: in.Content.MaxChars(),
		HasTitle: title != "",
	}
	return &scene.Legend{
		ID:       fmt.Sprintf("legend-%s", in.Channel),
		Channel:  in.Channel,
		Position: pl.Position,
		Title:    title,
		Entries:  entries,
		Padding:  pl.Padding,
		Frame:    placeLegendFrame(pl, box, plot),
	}
}

// gradientLegendTicks returns the labelled stops drawn alongside a
// gradient bar, ordered from the domain minimum to the maximum.
//
// legend.values, when given, pins the stops outright (non-numeric and
// out-of-domain entries are dropped). Otherwise legend.tick_count
// evenly spaced stops are generated: the default is
// legendGradientTickCount, a count of 1 labels the minimum alone, and
// 0 or less labels nothing — which is Vega-Lite's reading of a zero
// tick count and leaves a bare bar.
func gradientLegendTicks(g *GradientLegend, c LegendContent) []scene.LegendTick {
	span := g.DomainMax - g.DomainMin
	offsetOf := func(v float64) float64 {
		if span == 0 {
			return 0
		}
		return (v - g.DomainMin) / span
	}
	if len(c.Values) > 0 {
		out := make([]scene.LegendTick, 0, len(c.Values))
		for _, v := range c.Values {
			f, ok := legendValueFloat(v)
			if !ok || f < g.DomainMin || f > g.DomainMax {
				continue
			}
			out = append(out, scene.LegendTick{
				Offset: offsetOf(f),
				Label:  c.Truncate(c.Label(f)),
			})
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	n := legendGradientTickCount
	if c.TickCount != nil {
		n = *c.TickCount
	}
	if n <= 0 {
		return nil
	}
	out := make([]scene.LegendTick, 0, n)
	for i := 0; i < n; i++ {
		frac := 0.0
		if n > 1 {
			frac = float64(i) / float64(n-1)
		}
		v := g.DomainMin + frac*span
		out = append(out, scene.LegendTick{
			Offset: frac,
			Label:  c.Truncate(c.Label(v)),
		})
	}
	return out
}

// placeLegendFrame returns the pixel rect for the legend. Corner
// placements overlay the plot region, anchored to its corners and
// inset by Offset. Side placements sit outside the plot: right and
// bottom anchor off the near plot edge by Offset, while left and top
// anchor their far edge at the outer end of the reserved band
// (Reserve), which is what keeps them clear of the axis chrome. With
// no Reserve supplied a side placement falls back to hugging the plot
// edge — the pre-E1-S3 geometry.
func placeLegendFrame(pl LegendPlacement, box LegendBox, plot scene.Rect) scene.Rect {
	w, h := box.Size()
	off := pl.Offset
	switch pl.Position {
	case scene.LegendTopLeft:
		return scene.Rect{X: plot.X + off, Y: plot.Y + off, W: w, H: h}
	case scene.LegendBottomRight:
		return scene.Rect{X: plot.Right() - w - off, Y: plot.Bottom() - h - off, W: w, H: h}
	case scene.LegendBottomLeft:
		return scene.Rect{X: plot.X + off, Y: plot.Bottom() - h - off, W: w, H: h}
	case scene.LegendRight:
		return scene.Rect{X: plot.Right() + off, Y: plot.Y, W: w, H: h}
	case scene.LegendLeft:
		x := plot.X - w - off
		if pl.Reserve > 0 {
			x = plot.X - pl.Reserve
		}
		return scene.Rect{X: x, Y: plot.Y, W: w, H: h}
	case scene.LegendTop:
		y := plot.Y - h - off
		if pl.Reserve > 0 {
			y = plot.Y - pl.Reserve
		}
		return scene.Rect{X: plot.X, Y: y, W: w, H: h}
	case scene.LegendBottom:
		return scene.Rect{X: plot.X, Y: plot.Bottom() + off, W: w, H: h}
	}
	// scene.LegendTopRight and any unrecognised placement.
	return scene.Rect{X: plot.Right() - w - off, Y: plot.Y + off, W: w, H: h}
}
