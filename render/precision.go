package render

import (
	"math"
	"strconv"
)

// RenderPrecision pins float-to-string precision across every
// renderer for cross-impl + cross-Go-version stability. SVG goldens
// and the JS port's Canvas output both anchor on this constant. 3
// decimals = sub-millimetre at typical chart sizes; no perceivable
// visual difference vs higher precision. JS port matches via the
// scene-JSON having pre-rounded floats (the encoder snaps before
// emitting).
const RenderPrecision = 3

// FormatFloat renders v with RenderPrecision decimal places, trimming
// trailing zeros after the decimal point and dropping the decimal
// point itself when no fractional part remains (so "1.000" → "1",
// "1.230" → "1.23"). Matches D3's default formatting and keeps
// goldens diff-friendly.
//
// NaN and +/-Inf coordinates are rendered as "0" defensively — the
// encoder is responsible for catching non-finite values upstream and
// raising PRISM_RENDER_001; the renderer's job is to never emit
// broken SVG even when handed garbage.
func FormatFloat(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	s := strconv.FormatFloat(v, 'f', RenderPrecision, 64)
	// Trim trailing zeros after the decimal, then the decimal itself.
	dot := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return s
	}
	end := len(s)
	for end > dot+1 && s[end-1] == '0' {
		end--
	}
	if end == dot+1 {
		end = dot
	}
	return s[:end]
}
