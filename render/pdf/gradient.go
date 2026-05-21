package pdf

import (
	"github.com/signintech/gopdf"

	"github.com/frankbardon/prism/encode/scene"
)

// ApplyGradientFlatten paints a flattened-to-first-stop solid fill and
// returns the canonical warning code so the caller can attach it to
// SceneDoc.Warnings. PDF backend (signintech/gopdf v0.36) lacks a
// public axial / radial gradient API, so every gradient flattens.
//
// Warning code is chosen based on the gradient shape so consumers can
// distinguish the "expected" linear/radial flatten path from the
// "unsupported geometry" angular / text paths.
//
// Exported so per-mark renderers can opt in incrementally as their
// fill resolvers gain gradient awareness; the marks dispatcher does
// not call it directly yet.
func ApplyGradientFlatten(pdf *gopdf.GoPdf, g scene.Gradient, markType scene.MarkType) (code, gradientID string) {
	if len(g.Stops) > 0 {
		first := g.Stops[0].Color
		pdf.SetFillColor(first.R, first.G, first.B)
	}
	switch {
	case markType == scene.MarkText:
		return "PRISM_WARN_PDF_GRADIENT_TEXT_FLATTENED", g.Type
	case g.Type != "linear" && g.Type != "radial":
		return "PRISM_WARN_PDF_GRADIENT_ANGULAR_FLATTENED", g.Type
	default:
		return "PRISM_WARN_PDF_GRADIENT_FLATTENED", g.Type
	}
}
