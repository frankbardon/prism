package scene

import "math"

// AxisLabelFontSize is the default tick label size (theme/css.go's
// --prism-font-size-label fallback, 11 px in every built-in theme but
// high_contrast / print). The side-axis band below is measured at it:
// like LabelCharWidth it is a layout estimate, not a rendered value.
const AxisLabelFontSize = 11.0

// Side-axis title geometry, shared by the encoder (which reserves the
// room) and the SVG renderer (which places the title), so the two
// cannot disagree. A left or right axis title is rotated a quarter
// turn and anchored on its baseline: AxisTitleDescent is how far its
// glyphs reach from that baseline toward the plot, AxisTitleAscent
// how far they reach away from it, both for the default 12 px title.
const (
	AxisTitleDescent = 3.0
	AxisTitleAscent  = 10.0
)

// EstimateTextWidth returns a conservative estimate, in pixels, of how
// wide s renders at fontSize in the default sans stack (system-ui /
// Inter / Segoe UI / Arial). Prism runs no text-measurement pass, so
// anything that has to reserve room for a string measures it here.
//
// The estimate sums per-rune advance widths from a small glyph-class
// table (in em) calibrated against Chrome's system-ui at 11 and 12 px,
// rounded up so a real label is never wider than its estimate: a flat
// per-character width under-reads digits, capitals and m / w by enough
// to clip a long category name at the frame edge. It is deterministic
// and font-independent by design.
func EstimateTextWidth(s string, fontSize float64) float64 {
	em := 0.0
	for _, r := range s {
		em += runeAdvanceEm(r)
	}
	return float64(em * fontSize)
}

// runeAdvanceEm is one rune's estimated advance width in em.
func runeAdvanceEm(r rune) float64 {
	switch {
	case r == 'i' || r == 'j' || r == 'l' || r == 'I' || r == '\'' || r == '|' ||
		r == '!' || r == '.' || r == ',' || r == ':' || r == ';':
		return 0.3
	case r == ' ' || r == 'f' || r == 't' || r == 'r' || r == '(' || r == ')' ||
		r == '[' || r == ']' || r == '{' || r == '}' || r == '/' || r == '\\' ||
		r == '-' || r == '"' || r == '`':
		return 0.42
	case r == 'm' || r == 'w' || r == 'M' || r == 'W' || r == '@' || r == '%' || r == '&':
		return 0.98
	case r >= 'a' && r <= 'z':
		return 0.6
	case r >= 'A' && r <= 'Z':
		return 0.74
	case r >= '0' && r <= '9':
		return 0.65
	case r < 0x80:
		// Remaining ASCII punctuation and symbols (+ = < > _ # $ * ~ …).
		return 0.65
	case r >= 0x2E80:
		// CJK, Hangul, full-width forms and emoji are a full em.
		return 1.0
	}
	// Other scripts (accented Latin, Greek, Cyrillic, …) and the
	// typographic punctuation Prism itself emits (−, …, ×).
	return 0.7
}

// AxisLabelBand is the estimated width, in pixels, of the widest tick
// label a left or right axis draws, measured perpendicular to the axis
// (a rotated label is projected onto it). Hidden, minor and empty
// labels draw nothing and are not measured; an axis drawing no labels
// at all has a zero band.
func AxisLabelBand(a Axis) float64 {
	if a.HideLabels {
		return 0
	}
	sin, cos := 0.0, 1.0
	if a.LabelAngle != 0 {
		rad := float64(a.LabelAngle * math.Pi / 180)
		sin, cos = math.Abs(math.Sin(rad)), math.Abs(math.Cos(rad))
	}
	band := 0.0
	for _, t := range a.Ticks {
		if t.Label == "" || t.LabelHidden || t.Minor {
			continue
		}
		w := EstimateTextWidth(t.Label, AxisLabelFontSize)
		if a.LabelAngle != 0 {
			w = float64(w*cos) + float64(LabelLineHeight*sin)
		}
		if w > band {
			band = w
		}
	}
	return band
}

// SideAxisTitleOffset is the distance from the plot edge to a left or
// right axis title's baseline that clears a label band of band pixels:
// the labels' own offset from the plot edge (labelOffset), the band,
// the title padding, then the title's descent toward the plot.
func SideAxisTitleOffset(labelOffset, band, titlePad float64) float64 {
	return labelOffset + band + titlePad + AxisTitleDescent
}
