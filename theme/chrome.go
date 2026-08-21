package theme

// chrome.go holds the geometry and typography every built-in base
// shares, and the one place a "how a Prism chart is proportioned"
// decision is written down.
//
// Before this file each base restated all ~30 chrome tokens, so the
// five bases had quietly drifted apart: high_contrast reserved a
// different tick size than light, and a fix to one never reached the
// others. Chrome is not what distinguishes a base — colour and font
// size are — so the bases now supply those and inherit the rest.
//
// Every value here is a --prism-* token with a default, which is the
// contract downstream theming depends on: an organisation that wants
// squarer bars sets --prism-mark-bar-corner-radius and gets them, and
// nothing in this file is reachable only by editing Go.

// ChromeScale scales the whole chrome block for a base whose type is
// larger or smaller than the reference. high_contrast runs its labels
// at 13px rather than 11px, and its paddings and tick lengths have to
// grow with them or the chart reads as 11px chrome around 13px text.
type ChromeScale struct {
	// LabelSize is the base's axis-label size; 11 is the reference.
	LabelSize float64
	// TitleSize is the base's chart-title size; 15 is the reference.
	TitleSize float64
}

// chromeAxis returns the shared axis geometry, scaled and coloured.
//
// The restraint decisions live here:
//
//   - TickSize 4, down from 5, and the ticks themselves are suppressed
//     on any axis a grid line already registers. A tick's job is to
//     say where a label points; when a full-width grid line already
//     says it, the tick is a second answer to a question nobody asked.
//   - GridWidth stays 1px — a hairline grid at 0.5px disappears on a
//     non-retina screen — and the restraint comes from GridColor
//     instead, which is now barely-there rather than a visible grey.
//   - BandPadding 0.28, up from 0.10. At 0.10 four bars occupy 90% of
//     the plot and read as a filled block; the gap is what makes them
//     countable.
//   - BandMaxWidth 96 stops a one-category chart from drawing a single
//     bar the width of the plot.
func chromeAxis(sc ChromeScale, c chromeColors) *AxisStyle {
	f := func(v float64) *float64 { return &v }
	k := sc.labelRatio()
	return &AxisStyle{
		DomainColor:     c.Domain,
		DomainWidth:     f(1),
		TickColor:       c.Axis,
		TickWidth:       f(1),
		TickSize:        f(round1(4 * k)),
		GridColor:       c.Grid,
		GridWidth:       f(1),
		ZeroColor:       c.Zero,
		ZeroWidth:       f(1.5),
		BandPadding:     f(0.28),
		BandMaxWidth:    f(96),
		LabelColor:      c.TextMuted,
		LabelFontSize:   f(sc.LabelSize),
		LabelFontWeight: "400",
		LabelPadding:    f(round1(8 * k)),
		TitleColor:      c.TextMuted,
		TitleFontSize:   f(sc.LabelSize),
		TitleFontWeight: "500",
		TitlePadding:    f(round1(10 * k)),
	}
}

// chromeLegend returns the shared legend geometry.
//
// SymbolExtent 10 (a 10x10 swatch) with a 2px corner radius: the
// swatch is a sample of the mark, so it carries the mark's corner
// radius rather than being a bare square beside a rounded bar.
// RowHeight 18 at an 11px label is a 1.64 ratio — tight enough that
// six entries stay one block, loose enough that they do not touch.
func chromeLegend(sc ChromeScale, c chromeColors) *LegendStyle {
	f := func(v float64) *float64 { return &v }
	k := sc.labelRatio()
	return &LegendStyle{
		LabelColor:         c.Text,
		LabelFontSize:      f(sc.LabelSize),
		TitleColor:         c.TextMuted,
		TitleFontSize:      f(sc.LabelSize),
		TitleFontWeight:    "500",
		SymbolSize:         f(64),
		SymbolExtent:       f(round1(10 * k)),
		SymbolCornerRadius: f(2),
		Gap:                f(round1(16 * k)),
		RowHeight:          f(round1(18 * k)),
		Padding:            f(round1(8 * k)),
		RowPadding:         f(round1(4 * k)),
	}
}

// chromeTitle returns the shared title block.
//
// Anchor "start" and it is now honoured: the previous renderer
// centred every title regardless of the token, so an organisation
// that set start-aligned titles got centred ones. A left-aligned
// title also puts the chart's subject directly above the y axis,
// which is where a reader's eye enters a chart in a chat column.
func chromeTitle(sc ChromeScale, c chromeColors) *TitleStyle {
	f := func(v float64) *float64 { return &v }
	return &TitleStyle{
		Color:      c.Text,
		FontSize:   f(sc.TitleSize),
		FontWeight: "600",
		Anchor:     "start",
		Padding:    f(round1(14 * sc.titleRatio())),
	}
}

// chromeView returns the shared view block. Padding is the outer
// inset every side of the frame keeps clear, so a chart never renders
// flush against the container that holds it.
func chromeView(c chromeColors) *ViewStyle {
	f := func(v float64) *float64 { return &v }
	return &ViewStyle{
		Background: c.Background,
		Padding:    f(12),
	}
}

// chromeColors is the per-base colour set the shared chrome reads.
type chromeColors struct {
	Text       string
	TextMuted  string
	Axis       string
	Grid       string
	Domain     string
	Zero       string
	Background string
	// Surface is the actual paper colour, which is NOT Background:
	// Background is "transparent" on the light base so a chart adopts
	// whatever container it lands in, and a hairline drawn in
	// "transparent" is not a hairline. Anything that separates one
	// mark from its neighbour — an arc's divider, a point's halo —
	// draws in Surface.
	Surface string
}

func (s ChromeScale) labelRatio() float64 {
	if s.LabelSize <= 0 {
		return 1
	}
	return s.LabelSize / 11.0
}

func (s ChromeScale) titleRatio() float64 {
	if s.TitleSize <= 0 {
		return 1
	}
	return s.TitleSize / 15.0
}

// round1 quantises a scaled token to one decimal so the emitted CSS
// carries "5.2px" rather than "5.199999999999999px".
func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// chromeMarks returns the shared per-mark defaults over a primary
// colour.
//
// The three that changed and why:
//
//   - bar corner radius 2. Enough to soften the bar's shoulder at the
//     ~700px a chat column renders at, small enough that it does not
//     read as a pill or shorten the bar's apparent value. It applies
//     to the value end only; a rounded baseline would lift the bar off
//     its own axis.
//   - line stroke 2, up from 1.5. A 1.5px stroke inside an 800-wide
//     viewBox scaled into a 700px column lands at 1.3 device pixels
//     and antialiases into a grey smear. 2px survives the scale.
//   - point size 100 (r≈5.6), up from 64 (r≈4.5), with a surface-
//     coloured 1px stroke so overlapping points stay countable rather
//     than merging into a blob. Size is an AREA, so the radius grows
//     as its square root and 64→100 is a 25% wider dot, not a 56% one.
func chromeMarks(primary string, c chromeColors) map[string]*MarkStyle {
	f := func(v float64) *float64 { return &v }
	return map[string]*MarkStyle{
		"line":     {Stroke: primary, StrokeWidth: f(2), Fill: "transparent"},
		"rule":     {Stroke: primary, StrokeWidth: f(1)},
		"area":     {Fill: primary, FillOpacity: f(0.18), Stroke: primary, StrokeWidth: f(2)},
		"point":    {Fill: primary, Stroke: c.Surface, StrokeWidth: f(1), Size: f(100)},
		"bar":      {Fill: primary, CornerRadius: f(2)},
		"text":     {Fill: c.Text, FontSize: f(11), FontWeight: "500"},
		"tick":     {Stroke: primary, StrokeWidth: f(1)},
		"geoshape": {Fill: "#cbd5e1", Stroke: c.Surface, StrokeWidth: f(0.5)},
		"geopoint": {Fill: primary, StrokeWidth: f(0), Size: f(40)},
		"arc":      {Stroke: c.Surface, StrokeWidth: f(1.5)},
	}
}
