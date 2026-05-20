package pdf

import (
	"github.com/signintech/gopdf"

	"github.com/frankbardon/prism/encode/scene"
)

// styleString returns the gopdf draw-style string per the populated
// fields. "F" for fill-only, "D" for stroke-only, "DF" for both,
// "" for neither (caller decides whether to elide the operator
// entirely).
func styleString(fill, stroke *scene.Color, strokeWidth float64) string {
	hasFill := fill != nil && fill.A > 0
	hasStroke := stroke != nil && stroke.A > 0 && strokeWidth > 0
	switch {
	case hasFill && hasStroke:
		return "DF"
	case hasFill:
		return "F"
	case hasStroke:
		return "D"
	default:
		return ""
	}
}

// setFill applies the fill color to pdf. nil color leaves the prior
// fill state untouched; alpha is handled at the mark level via
// applyMarkStyle's transparency wrap so this helper only writes the
// RGB triplet.
func setFill(pdf *gopdf.GoPdf, c *scene.Color) {
	if c == nil {
		return
	}
	pdf.SetFillColor(c.R, c.G, c.B)
}

// setStroke applies the stroke color + width. Empty width = leave as
// caller default; gopdf's default is 1pt. Pass nil color when only
// width should change.
func setStroke(pdf *gopdf.GoPdf, c *scene.Color, width float64) {
	if c != nil {
		pdf.SetStrokeColor(c.R, c.G, c.B)
	}
	if width > 0 {
		pdf.SetLineWidth(width)
	}
}

// setLineDash applies a dash pattern via gopdf.SetCustomLineType.
// Empty slice clears any prior dash (solid line). Phase is fixed at
// 0 — Prism's Scene IR carries the array form per D079-style usage,
// no phase offset.
func setLineDash(pdf *gopdf.GoPdf, dash []float64) {
	if len(dash) == 0 {
		pdf.SetCustomLineType(nil, 0)
		return
	}
	// Defensive copy — gopdf retains the slice header inside the
	// content stream until the page is closed; the caller's slice
	// can outlive that lifetime.
	cp := make([]float64, len(dash))
	copy(cp, dash)
	pdf.SetCustomLineType(cp, 0)
}

// applyMarkStyle composes fill / stroke / dash / alpha and returns
// an end closure the caller defers. The closure restores the prior
// transparency state and clears the dash pattern so subsequent
// marks start clean.
//
// Effective alpha = Style.Opacity (when > 0) * (fillAlpha / 255 or
// strokeAlpha / 255 — whichever is most restrictive). When alpha is
// effectively 1.0, no transparency state is pushed.
func applyMarkStyle(pdf *gopdf.GoPdf, s scene.Style) func() {
	setFill(pdf, s.Fill)
	setStroke(pdf, s.Stroke, s.StrokeWidth)
	setLineDash(pdf, s.StrokeDash)

	alpha := effectiveAlpha(s)
	if alpha < 1.0 {
		t, err := gopdf.NewTransparency(alpha, "Normal")
		if err == nil {
			_ = pdf.SetTransparency(t)
			return func() {
				pdf.ClearTransparency()
				if len(s.StrokeDash) > 0 {
					pdf.SetCustomLineType(nil, 0)
				}
			}
		}
	}
	return func() {
		if len(s.StrokeDash) > 0 {
			pdf.SetCustomLineType(nil, 0)
		}
	}
}

// effectiveAlpha collapses Style.Opacity + fill / stroke alpha into
// a single transparency factor. Zero Opacity is treated as "unset"
// and falls back to the color alpha channel. Values clamped to
// [0, 1].
func effectiveAlpha(s scene.Style) float64 {
	op := s.Opacity
	if op == 0 {
		op = 1.0
	}
	if op > 1 {
		op = 1
	}
	if op < 0 {
		op = 0
	}

	colorAlpha := 1.0
	if s.Fill != nil && s.Fill.A < 255 {
		colorAlpha = float64(s.Fill.A) / 255.0
	} else if s.Stroke != nil && s.Stroke.A < 255 {
		colorAlpha = float64(s.Stroke.A) / 255.0
	}
	return op * colorAlpha
}
