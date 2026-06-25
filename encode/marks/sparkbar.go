package marks

import (
	"github.com/frankbardon/prism/encode/scene"
)

// encodeSparkbar emits a rect (column) geometry identical to encodeBar.
// The "strip axes/legends/title" behaviour lives at the encode.go
// level (E2-S1) — it suppresses axis builders + legend builders +
// title-block construction when isSparkMark(markType) is true.
//
// Plot region uses 4-px padding (ComputeSparkline in encode/layout.go).
// Zero new geometry code; sparkbar is a thin wrapper over bar — the
// bar-family sibling of sparkline.
func encodeSparkbar(in Inputs) ([]scene.Mark, error) {
	marks, err := encodeBar(in)
	if err != nil {
		return nil, err
	}
	// Tag the mark IDs so the renderer can still pick them out.
	for i := range marks {
		if marks[i].ID == "" {
			marks[i].ID = "sparkbar-0"
		} else {
			marks[i].ID = "sparkbar-" + marks[i].ID
		}
	}
	// Opt-in adornments (E4); no-op when no adornment field is set.
	return appendSparkAdornments(in, marks)
}
