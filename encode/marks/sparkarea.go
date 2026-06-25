package marks

import (
	"github.com/frankbardon/prism/encode/scene"
)

// encodeSparkarea emits a filled-area geometry identical to encodeArea.
// The "strip axes/legends/title" behaviour lives at the encode.go
// level (E2-S1) — it suppresses axis builders + legend builders +
// title-block construction when isSparkMark(markType) is true.
//
// Plot region uses 4-px padding (ComputeSparkline in encode/layout.go).
// The fill reaches the y=0 baseline because encodeArea populates
// AreaGeom.Lower from Y.Scale.Apply(0) (E1-S1). Zero new geometry code;
// sparkarea is a thin wrapper over area — the area-family sibling of
// sparkline.
func encodeSparkarea(in Inputs) ([]scene.Mark, error) {
	marks, err := encodeArea(in)
	if err != nil {
		return nil, err
	}
	// Tag the mark IDs so the renderer can still pick them out.
	for i := range marks {
		if marks[i].ID == "" {
			marks[i].ID = "sparkarea-0"
		} else {
			marks[i].ID = "sparkarea-" + marks[i].ID
		}
	}
	return marks, nil
}
