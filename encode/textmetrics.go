package encode

import "strings"

// TextMetrics estimates rendered text extents without a font engine.
//
// The renderer emits SVG that a browser lays out, so Prism never sees
// the real advance widths. Layout still has to reserve space for axis
// labels, legend entries, and titles BEFORE the bytes exist, and a
// wrong estimate is visible: too small clips the label, too large
// wastes plot area. The table below is per-character advance width in
// em units for a UI sans (Inter / system-ui / Helvetica are within a
// few percent of each other at these sizes), which is accurate to
// roughly ±4% on mixed-case strings — well inside the padding the
// layout adds anyway.
//
// Pure stdlib and allocation-light on purpose: this runs in the WASM
// build, where a font engine is not available at any price.
//
// Widths are intentionally biased ~2% high. Reserving a hair too much
// space costs a few pixels of plot; reserving too little clips a
// label, which is the failure the user actually sees.
type TextMetrics struct {
	// FontSize is the em size the estimate is scaled to.
	FontSize float64
	// Tabular reports whether digits render at a uniform advance
	// (font-variant-numeric: tabular-nums). Numeric axis labels do,
	// which makes their width exactly len(s) * digitEm.
	Tabular bool
	// Bold widens the estimate; semibold/bold faces run ~3% wider
	// than their regular counterparts at UI sizes.
	Bold bool
}

// digitEm is the advance width of a digit in a tabular-numeral UI
// sans. Kept separate because numeric labels are the common case and
// their width must be exact enough to align a right-aligned column.
const digitEm = 0.60

// narrowEm / wideEm / defaultEm bracket the per-character table.
const (
	narrowEm  = 0.29
	defaultEm = 0.545
	wideEm    = 0.86
)

// charEm returns the advance width of r in em units.
func charEm(r rune) float64 {
	switch {
	case r >= '0' && r <= '9':
		return digitEm
	case r == ' ':
		return 0.26
	}
	if w, ok := charWidths[r]; ok {
		return w
	}
	switch {
	case r >= 'a' && r <= 'z':
		return defaultEm
	case r >= 'A' && r <= 'Z':
		return 0.67
	case r > 0x7f:
		// Non-Latin: CJK and emoji are near-square, accented Latin is
		// not. Treat anything past Latin-1 Supplement as full width;
		// accented Latin keeps the lowercase default.
		if r > 0x024f {
			return 1.0
		}
		return defaultEm
	}
	return defaultEm
}

// charWidths holds the characters that deviate enough from the
// bucket default to move a label's box by a visible amount.
var charWidths = map[rune]float64{
	'i': narrowEm, 'j': narrowEm, 'l': narrowEm, 'I': narrowEm,
	'f': 0.32, 't': 0.36, 'r': 0.38,
	'.': 0.28, ',': 0.28, ':': 0.28, ';': 0.28, '\'': 0.22, '"': 0.38,
	'|': 0.26, '!': 0.28, '(': 0.34, ')': 0.34, '[': 0.32, ']': 0.32,
	'/': 0.40, '\\': 0.40, '-': 0.38, '–': 0.55, '—': 0.90,
	'm': wideEm, 'w': 0.78, 'M': 0.88, 'W': 0.92,
	'%': 0.86, '@': 0.98, '#': 0.68, '$': 0.60, '&': 0.72,
}

// Width returns the estimated rendered width of s in pixels.
func (m TextMetrics) Width(s string) float64 {
	if s == "" {
		return 0
	}
	size := m.FontSize
	if size <= 0 {
		size = 11
	}
	var em float64
	if m.Tabular && isNumericLabel(s) {
		// Tabular numerals give every digit — and the separators that
		// travel with them — the same advance, so the width is exact.
		em = float64(len([]rune(s))) * digitEm
	} else {
		for _, r := range s {
			em += charEm(r)
		}
	}
	if m.Bold {
		em *= 1.03
	}
	return em * size * 1.02
}

// Height returns the estimated line box height in pixels. UI sans
// faces run about 1.15em cap-to-descender at label sizes.
func (m TextMetrics) Height() float64 {
	size := m.FontSize
	if size <= 0 {
		size = 11
	}
	return size * 1.15
}

// isNumericLabel reports whether s is composed only of characters
// that a tabular-numeral face renders at the digit advance: digits,
// the separators that appear inside formatted numbers, and the SI /
// percent suffixes that follow them.
func isNumericLabel(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '.' || r == ',' || r == '-' || r == '−' || r == '+' ||
			r == '%' || r == 'k' || r == 'M' || r == 'G' || r == 'B' ||
			r == 'T' || r == 'm' || r == 'µ' || r == 'e' || r == ' ':
		default:
			return false
		}
	}
	return s != ""
}

// MaxWidth returns the widest estimated width across labels.
func (m TextMetrics) MaxWidth(labels []string) float64 {
	var max float64
	for _, l := range labels {
		if w := m.Width(l); w > max {
			max = w
		}
	}
	return max
}

// Ellipsis is the single character appended to a truncated label. One
// glyph rather than three dots so the truncation costs the label as
// little width as possible.
const Ellipsis = "…"

// Truncate shortens s until its estimated width fits maxWidth,
// appending Ellipsis. Returns s unchanged when it already fits, and
// reports whether it truncated so the caller can attach the full text
// as a <title> — a truncated label must never be the only place the
// value existed.
//
// Never returns a bare ellipsis: below the width where even one
// character plus the ellipsis fits, the first character is kept and
// the label overflows slightly rather than becoming meaningless.
func (m TextMetrics) Truncate(s string, maxWidth float64) (string, bool) {
	if s == "" || maxWidth <= 0 || m.Width(s) <= maxWidth {
		return s, false
	}
	runes := []rune(s)
	ellW := m.Width(Ellipsis)
	budget := maxWidth - ellW
	if budget <= 0 {
		return string(runes[:1]) + Ellipsis, true
	}
	var acc float64
	cut := 0
	size := m.FontSize
	if size <= 0 {
		size = 11
	}
	for i, r := range runes {
		w := charEm(r) * size * 1.02
		if m.Bold {
			w *= 1.03
		}
		if acc+w > budget {
			cut = i
			break
		}
		acc += w
		cut = i + 1
	}
	if cut < 1 {
		cut = 1
	}
	// Do not leave a dangling separator before the ellipsis.
	trimmed := strings.TrimRight(string(runes[:cut]), " ,.-/")
	if trimmed == "" {
		trimmed = string(runes[:1])
	}
	return trimmed + Ellipsis, true
}
